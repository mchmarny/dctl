package data

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	// Bus factor: count of developers whose cumulative event share reaches 50%.
	// Uses a CTE that ranks developers by event count descending, then
	// accumulates a running sum to find the cutoff.
	selectBusFactorSQL = `WITH dev_counts AS (
			SELECT username, COUNT(*) AS cnt
			FROM event
			WHERE org = COALESCE(?, org)
			  AND repo = COALESCE(?, repo)
			  AND date >= ?
			GROUP BY username
			ORDER BY cnt DESC
		),
		running AS (
			SELECT username, cnt,
				SUM(cnt) OVER (ORDER BY cnt DESC) AS cumsum,
				SUM(cnt) OVER () AS total
			FROM dev_counts
		)
		SELECT COUNT(*) FROM running WHERE cumsum - cnt < total * 0.5
	`

	// Pony factor: same pattern but grouped by entity.
	selectPonyFactorSQL = `WITH ent_counts AS (
			SELECT d.entity, COUNT(*) AS cnt
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE(?, e.org)
			  AND e.repo = COALESCE(?, e.repo)
			  AND e.date >= ?
			  AND d.entity IS NOT NULL AND d.entity != ''
			GROUP BY d.entity
			ORDER BY cnt DESC
		),
		running AS (
			SELECT entity, cnt,
				SUM(cnt) OVER (ORDER BY cnt DESC) AS cumsum,
				SUM(cnt) OVER () AS total
			FROM ent_counts
		)
		SELECT COUNT(*) FROM running WHERE cumsum - cnt < total * 0.5
	`

	// Contributor retention: for each month, count new (first seen) vs returning developers.
	selectRetentionSQL = `WITH first_seen AS (
			SELECT username, MIN(substr(date, 1, 7)) AS first_month
			FROM event
			WHERE org = COALESCE(?, org)
			  AND repo = COALESCE(?, repo)
			  AND date >= ?
			GROUP BY username
		),
		monthly AS (
			SELECT DISTINCT e.username, substr(e.date, 1, 7) AS month
			FROM event e
			WHERE e.org = COALESCE(?, e.org)
			  AND e.repo = COALESCE(?, e.repo)
			  AND e.date >= ?
		)
		SELECT m.month,
			SUM(CASE WHEN f.first_month = m.month THEN 1 ELSE 0 END) AS new_contributors,
			SUM(CASE WHEN f.first_month < m.month THEN 1 ELSE 0 END) AS returning_contributors
		FROM monthly m
		JOIN first_seen f ON m.username = f.username
		GROUP BY m.month
		ORDER BY m.month
	`

	// Time-to-merge: avg days from created_at to merged_at for merged PRs, per month.
	selectTimeToMergeSQL = `SELECT
			substr(created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(julianday(merged_at) - julianday(created_at)) AS avg_days
		FROM event
		WHERE type = 'pr'
		  AND merged_at IS NOT NULL
		  AND created_at IS NOT NULL
		  AND org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		  AND date >= ?
		GROUP BY month
		ORDER BY month
	`

	// Time-to-close: avg days from created_at to closed_at for closed issues, per month.
	selectTimeToCloseSQL = `SELECT
			substr(created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(julianday(closed_at) - julianday(created_at)) AS avg_days
		FROM event
		WHERE type = 'issue'
		  AND closed_at IS NOT NULL
		  AND created_at IS NOT NULL
		  AND state = 'closed'
		  AND org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		  AND date >= ?
		GROUP BY month
		ORDER BY month
	`

	// PR-to-review ratio: monthly PR and PR review counts with computed ratio.
	selectPRReviewRatioSQL = `SELECT
			substr(date, 1, 7) AS month,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS prs,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS reviews
		FROM event
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		  AND date >= ?
		  AND type IN (?, ?)
		GROUP BY month
		ORDER BY month
	`
)

type VelocitySeries struct {
	Months  []string  `json:"months"`
	Count   []int     `json:"count"`
	AvgDays []float64 `json:"avg_days"`
}

type InsightsSummary struct {
	BusFactor  int `json:"bus_factor"`
	PonyFactor int `json:"pony_factor"`
}

type RetentionSeries struct {
	Months    []string `json:"months"`
	New       []int    `json:"new"`
	Returning []int    `json:"returning"`
}

type PRReviewRatioSeries struct {
	Months  []string  `json:"months"`
	PRs     []int     `json:"prs"`
	Reviews []int     `json:"reviews"`
	Ratio   []float64 `json:"ratio"`
}

func GetInsightsSummary(db *sql.DB, org, repo, entity *string, months int) (*InsightsSummary, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")
	summary := &InsightsSummary{}

	if err := db.QueryRow(selectBusFactorSQL, org, repo, since).Scan(&summary.BusFactor); err != nil {
		return nil, fmt.Errorf("failed to query bus factor: %w", err)
	}

	if err := db.QueryRow(selectPonyFactorSQL, org, repo, since).Scan(&summary.PonyFactor); err != nil {
		return nil, fmt.Errorf("failed to query pony factor: %w", err)
	}

	return summary, nil
}

func GetContributorRetention(db *sql.DB, org, repo *string, months int) (*RetentionSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectRetentionSQL, org, repo, since, org, repo, since)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query contributor retention: %w", err)
	}
	defer rows.Close()

	s := &RetentionSeries{
		Months:    make([]string, 0),
		New:       make([]int, 0),
		Returning: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var newC, retC int
		if err := rows.Scan(&month, &newC, &retC); err != nil {
			return nil, fmt.Errorf("failed to scan retention row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.New = append(s.New, newC)
		s.Returning = append(s.Returning, retC)
	}

	return s, nil
}

func GetPRReviewRatio(db *sql.DB, org, repo *string, months int) (*PRReviewRatioSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectPRReviewRatioSQL,
		EventTypePR, EventTypePRReview,
		org, repo, since,
		EventTypePR, EventTypePRReview)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query PR review ratio: %w", err)
	}
	defer rows.Close()

	s := &PRReviewRatioSeries{
		Months:  make([]string, 0),
		PRs:     make([]int, 0),
		Reviews: make([]int, 0),
		Ratio:   make([]float64, 0),
	}

	for rows.Next() {
		var month string
		var prs, reviews int
		if err := rows.Scan(&month, &prs, &reviews); err != nil {
			return nil, fmt.Errorf("failed to scan PR review ratio row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.PRs = append(s.PRs, prs)
		s.Reviews = append(s.Reviews, reviews)

		var ratio float64
		if prs > 0 {
			ratio = float64(reviews) / float64(prs)
		}
		s.Ratio = append(s.Ratio, ratio)
	}

	return s, nil
}

func getVelocitySeries(db *sql.DB, query string, org, repo *string, months int) (*VelocitySeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(query, org, repo, since)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query velocity series: %w", err)
	}
	defer rows.Close()

	s := &VelocitySeries{
		Months:  make([]string, 0),
		Count:   make([]int, 0),
		AvgDays: make([]float64, 0),
	}

	for rows.Next() {
		var month string
		var cnt int
		var avgDays float64
		if err := rows.Scan(&month, &cnt, &avgDays); err != nil {
			return nil, fmt.Errorf("failed to scan velocity row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Count = append(s.Count, cnt)
		s.AvgDays = append(s.AvgDays, avgDays)
	}

	return s, nil
}

func GetTimeToMerge(db *sql.DB, org, repo *string, months int) (*VelocitySeries, error) {
	return getVelocitySeries(db, selectTimeToMergeSQL, org, repo, months)
}

func GetTimeToClose(db *sql.DB, org, repo *string, months int) (*VelocitySeries, error) {
	return getVelocitySeries(db, selectTimeToCloseSQL, org, repo, months)
}
