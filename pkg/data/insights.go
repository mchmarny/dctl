package data

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	// Bus factor: count of developers whose cumulative event share reaches 50%.
	selectBusFactorSQL = `WITH dev_counts AS (
			SELECT e.username, COUNT(*) AS cnt
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE(?, e.org)
			  AND e.repo = COALESCE(?, e.repo)
			  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
			  AND e.date >= ?
			  AND e.username NOT LIKE '%[bot]'
			  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
			GROUP BY e.username
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
			  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
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
			SELECT e.username, MIN(substr(e.date, 1, 7)) AS first_month
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE(?, e.org)
			  AND e.repo = COALESCE(?, e.repo)
			  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
			  AND e.date >= ?
			  AND e.username NOT LIKE '%[bot]'
			  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
			GROUP BY e.username
		),
		monthly AS (
			SELECT DISTINCT e.username, substr(e.date, 1, 7) AS month
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE(?, e.org)
			  AND e.repo = COALESCE(?, e.repo)
			  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
			  AND e.date >= ?
			  AND e.username NOT LIKE '%[bot]'
			  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
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
			substr(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(julianday(e.merged_at) - julianday(e.created_at)) AS avg_days
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.merged_at IS NOT NULL
		  AND e.created_at IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY month
		ORDER BY month
	`

	// Time-to-restore (bugs): avg days to close bug issues filed within 7 days of a release.
	selectTimeToRestoreBugsSQL = `SELECT
			substr(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(julianday(e.closed_at) - julianday(e.created_at)) AS avg_days
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.closed_at IS NOT NULL
		  AND e.created_at IS NOT NULL
		  AND e.state = 'closed'
		  AND LOWER(e.labels) LIKE '%bug%'
		  AND EXISTS (
		      SELECT 1 FROM release r
		      WHERE r.org = e.org AND r.repo = e.repo
		        AND julianday(e.created_at) - julianday(r.published_at) BETWEEN 0 AND 7
		  )
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY month
		ORDER BY month
	`

	// Time-to-close: avg days from created_at to closed_at for closed issues, per month.
	selectTimeToCloseSQL = `SELECT
			substr(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(julianday(e.closed_at) - julianday(e.created_at)) AS avg_days
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.closed_at IS NOT NULL
		  AND e.created_at IS NOT NULL
		  AND e.state = 'closed'
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY month
		ORDER BY month
	`

	// Forks and activity: monthly fork count and total event count.
	selectForksAndActivitySQL = `SELECT
			substr(e.date, 1, 7) AS month,
			SUM(CASE WHEN e.type = 'fork' THEN 1 ELSE 0 END) AS forks,
			COUNT(*) AS events
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.date >= ?
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY month
		ORDER BY month
	`

	// PR-to-review ratio: monthly PR and PR review counts with computed ratio.
	selectPRReviewRatioSQL = `SELECT
			substr(e.date, 1, 7) AS month,
			SUM(CASE WHEN e.type = ? THEN 1 ELSE 0 END) AS prs,
			SUM(CASE WHEN e.type = ? THEN 1 ELSE 0 END) AS reviews
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.date >= ?
		  AND e.type IN (?, ?)
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY month
		ORDER BY month
	`

	selectChangeFailuresSQL = `SELECT
		substr(e.created_at, 1, 7) AS month,
		COUNT(*) AS failures
	FROM event e
	JOIN developer d ON e.username = d.username
	WHERE (
	    (e.type = 'issue' AND LOWER(e.labels) LIKE '%bug%'
	     AND EXISTS (
	        SELECT 1 FROM release r
	        WHERE r.org = e.org AND r.repo = e.repo
	          AND julianday(e.created_at) - julianday(r.published_at) BETWEEN 0 AND 7
	     ))
	    OR
	    (e.type = 'pr' AND LOWER(e.title) LIKE '%revert%')
	)
	  AND e.org = COALESCE(?, e.org)
	  AND e.repo = COALESCE(?, e.repo)
	  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
	  AND e.created_at >= ?
	  AND e.username NOT LIKE '%[bot]'
	  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
	GROUP BY month
	ORDER BY month
	`

	selectDeploymentCountSQL = `SELECT
		substr(published_at, 1, 7) AS month,
		COUNT(*) AS cnt
	FROM release
	WHERE org = COALESCE(?, org)
	  AND repo = COALESCE(?, repo)
	  AND published_at >= ?
	GROUP BY month
	ORDER BY month
	`

	// Review latency: avg hours from PR creation to first review, per month.
	selectReviewLatencySQL = `SELECT
		month,
		COUNT(*) AS cnt,
		AVG(hours) AS avg_hours
	FROM (
	    SELECT
	        substr(pr.created_at, 1, 7) AS month,
	        (julianday(MIN(rev.created_at)) - julianday(pr.created_at)) * 24 AS hours
	    FROM event pr
	    JOIN event rev ON pr.org = rev.org AND pr.repo = rev.repo AND pr.number = rev.number
	        AND rev.type = 'pr_review'
	    JOIN developer d ON pr.username = d.username
	    WHERE pr.type = 'pr'
	      AND pr.number IS NOT NULL
	      AND pr.created_at IS NOT NULL
	      AND rev.created_at IS NOT NULL
	      AND pr.org = COALESCE(?, pr.org)
	      AND pr.repo = COALESCE(?, pr.repo)
	      AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
	      AND pr.created_at >= ?
	      AND pr.username NOT LIKE '%[bot]'
	      AND pr.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
	    GROUP BY pr.org, pr.repo, pr.number, month
	)
	GROUP BY month
	ORDER BY month
	`

	selectPRSizeDistributionSQL = `SELECT
		substr(e.created_at, 1, 7) AS month,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) < 50 THEN 1 ELSE 0 END) AS small,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 50 AND 249 THEN 1 ELSE 0 END) AS medium,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 250 AND 999 THEN 1 ELSE 0 END) AS large,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) >= 1000 THEN 1 ELSE 0 END) AS xlarge
	FROM event e
	JOIN developer d ON e.username = d.username
	WHERE e.type = 'pr'
	  AND e.created_at IS NOT NULL
	  AND e.org = COALESCE(?, e.org)
	  AND e.repo = COALESCE(?, e.repo)
	  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
	  AND e.created_at >= ?
	  AND e.username NOT LIKE '%[bot]'
	  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
	GROUP BY month
	ORDER BY month
	`

	// Contributor momentum: rolling 3-month active contributor count.
	selectContributorMomentumSQL = `WITH months AS (
		SELECT DISTINCT substr(date, 1, 7) AS month
		FROM event
		WHERE date >= ?
	)
	SELECT
		m.month,
		COUNT(DISTINCT e.username) AS active
	FROM months m
	JOIN event e ON substr(e.date, 1, 7) >= substr(date(m.month || '-01', '-2 months'), 1, 7)
		AND substr(e.date, 1, 7) <= m.month
	JOIN developer d ON e.username = d.username
	WHERE e.org = COALESCE(?, e.org)
	  AND e.repo = COALESCE(?, e.repo)
	  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
	  AND e.username NOT LIKE '%[bot]'
	  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
	GROUP BY m.month
	ORDER BY m.month
	`

	// Contributor funnel: first comment, first PR, first merge per user, counted by month.
	selectContributorFunnelSQL = `WITH firsts AS (
		SELECT
			e.username,
			MIN(CASE WHEN e.type = 'issue_comment' THEN e.date END) AS first_comment,
			MIN(CASE WHEN e.type = 'pr' THEN e.date END) AS first_pr,
			MIN(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN e.date END) AS first_merge
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY e.username
	),
	months AS (
		SELECT DISTINCT substr(date, 1, 7) AS month FROM event WHERE date >= ?
	)
	SELECT
		m.month,
		SUM(CASE WHEN f.first_comment IS NOT NULL AND substr(f.first_comment, 1, 7) = m.month THEN 1 ELSE 0 END) AS fc,
		SUM(CASE WHEN f.first_pr IS NOT NULL AND substr(f.first_pr, 1, 7) = m.month THEN 1 ELSE 0 END) AS fp,
		SUM(CASE WHEN f.first_merge IS NOT NULL AND substr(f.first_merge, 1, 7) = m.month THEN 1 ELSE 0 END) AS fm
	FROM months m
	CROSS JOIN firsts f
	WHERE substr(COALESCE(f.first_comment, f.first_pr, f.first_merge), 1, 7) >= (SELECT MIN(month) FROM months)
	GROUP BY m.month
	HAVING fc > 0 OR fp > 0 OR fm > 0
	ORDER BY m.month
	`

	selectDailyActivitySQL = `SELECT e.date, COUNT(*) AS cnt
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.date >= ?
		  AND e.username NOT LIKE '%[bot]'
		  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
		GROUP BY e.date
		ORDER BY e.date
	`
)

type DailyActivitySeries struct {
	Dates  []string `json:"dates"`
	Counts []int    `json:"counts"`
}

type VelocitySeries struct {
	Months  []string  `json:"months" yaml:"months"`
	Count   []int     `json:"count" yaml:"count"`
	AvgDays []float64 `json:"avg_days" yaml:"avgDays"`
}

type InsightsSummary struct {
	BusFactor  int `json:"bus_factor" yaml:"busFactor"`
	PonyFactor int `json:"pony_factor" yaml:"ponyFactor"`
}

type RetentionSeries struct {
	Months    []string `json:"months" yaml:"months"`
	New       []int    `json:"new" yaml:"new"`
	Returning []int    `json:"returning" yaml:"returning"`
}

type PRReviewRatioSeries struct {
	Months  []string  `json:"months" yaml:"months"`
	PRs     []int     `json:"prs" yaml:"prs"`
	Reviews []int     `json:"reviews" yaml:"reviews"`
	Ratio   []float64 `json:"ratio" yaml:"ratio"`
}

type ChangeFailureRateSeries struct {
	Months      []string  `json:"months" yaml:"months"`
	Failures    []int     `json:"failures" yaml:"failures"`
	Deployments []int     `json:"deployments" yaml:"deployments"`
	Rate        []float64 `json:"rate" yaml:"rate"`
}

type ReviewLatencySeries struct {
	Months   []string  `json:"months" yaml:"months"`
	Count    []int     `json:"count" yaml:"count"`
	AvgHours []float64 `json:"avg_hours" yaml:"avgHours"`
}

type PRSizeSeries struct {
	Months []string `json:"months" yaml:"months"`
	Small  []int    `json:"small" yaml:"small"`
	Medium []int    `json:"medium" yaml:"medium"`
	Large  []int    `json:"large" yaml:"large"`
	XLarge []int    `json:"xlarge" yaml:"xlarge"`
}

func GetInsightsSummary(db *sql.DB, org, repo, entity *string, months int) (*InsightsSummary, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")
	summary := &InsightsSummary{}

	if err := db.QueryRow(selectBusFactorSQL, org, repo, entity, since).Scan(&summary.BusFactor); err != nil {
		return nil, fmt.Errorf("failed to query bus factor: %w", err)
	}

	if err := db.QueryRow(selectPonyFactorSQL, org, repo, entity, since).Scan(&summary.PonyFactor); err != nil {
		return nil, fmt.Errorf("failed to query pony factor: %w", err)
	}

	return summary, nil
}

func GetDailyActivity(db *sql.DB, org, repo, entity *string, months int) (*DailyActivitySeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectDailyActivitySQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query daily activity: %w", err)
	}
	defer rows.Close()

	series := &DailyActivitySeries{}
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan daily activity row: %w", err)
		}
		series.Dates = append(series.Dates, date)
		series.Counts = append(series.Counts, count)
	}

	return series, nil
}

func GetContributorRetention(db *sql.DB, org, repo, entity *string, months int) (*RetentionSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectRetentionSQL, org, repo, entity, since, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func GetPRReviewRatio(db *sql.DB, org, repo, entity *string, months int) (*PRReviewRatioSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectPRReviewRatioSQL,
		EventTypePR, EventTypePRReview,
		org, repo, entity, since,
		EventTypePR, EventTypePRReview)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func GetChangeFailureRate(db *sql.DB, org, repo, entity *string, months int) (*ChangeFailureRateSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	// Get failures by month
	failureMap := make(map[string]int)

	rows, err := db.Query(selectChangeFailuresSQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query change failures: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var month string
		var failures int
		if scanErr := rows.Scan(&month, &failures); scanErr != nil {
			return nil, fmt.Errorf("failed to scan change failure row: %w", scanErr)
		}
		failureMap[month] = failures
	}

	// Get deployments by month
	deployMap := make(map[string]int)

	dRows, err := db.Query(selectDeploymentCountSQL, org, repo, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query deployment count: %w", err)
	}
	defer dRows.Close()

	for dRows.Next() {
		var month string
		var cnt int
		if scanErr := dRows.Scan(&month, &cnt); scanErr != nil {
			return nil, fmt.Errorf("failed to scan deployment count row: %w", scanErr)
		}
		deployMap[month] = cnt
	}

	// Merge all months from both maps
	monthSet := make(map[string]bool)
	for m := range failureMap {
		monthSet[m] = true
	}
	for m := range deployMap {
		monthSet[m] = true
	}

	// Sort months
	sortedMonths := make([]string, 0, len(monthSet))
	for m := range monthSet {
		sortedMonths = append(sortedMonths, m)
	}
	sort.Strings(sortedMonths)

	s := &ChangeFailureRateSeries{
		Months:      make([]string, 0, len(sortedMonths)),
		Failures:    make([]int, 0, len(sortedMonths)),
		Deployments: make([]int, 0, len(sortedMonths)),
		Rate:        make([]float64, 0, len(sortedMonths)),
	}

	for _, m := range sortedMonths {
		f := failureMap[m]
		d := deployMap[m]
		var rate float64
		if d > 0 {
			rate = float64(f) / float64(d) * 100
		}
		s.Months = append(s.Months, m)
		s.Failures = append(s.Failures, f)
		s.Deployments = append(s.Deployments, d)
		s.Rate = append(s.Rate, rate)
	}

	return s, nil
}

//nolint:dupl // similar structure to getVelocitySeries but different return type (ReviewLatencySeries vs VelocitySeries)
func GetReviewLatency(db *sql.DB, org, repo, entity *string, months int) (*ReviewLatencySeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReviewLatencySQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query review latency: %w", err)
	}
	defer rows.Close()

	s := &ReviewLatencySeries{
		Months:   make([]string, 0),
		Count:    make([]int, 0),
		AvgHours: make([]float64, 0),
	}

	for rows.Next() {
		var month string
		var cnt int
		var avgHours float64
		if err := rows.Scan(&month, &cnt, &avgHours); err != nil {
			return nil, fmt.Errorf("failed to scan review latency row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Count = append(s.Count, cnt)
		s.AvgHours = append(s.AvgHours, avgHours)
	}

	return s, nil
}

//nolint:dupl // similar structure to GetReviewLatency but different return type (VelocitySeries vs ReviewLatencySeries)
func getVelocitySeries(db *sql.DB, query string, org, repo, entity *string, months int) (*VelocitySeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(query, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func GetTimeToMerge(db *sql.DB, org, repo, entity *string, months int) (*VelocitySeries, error) {
	return getVelocitySeries(db, selectTimeToMergeSQL, org, repo, entity, months)
}

func GetTimeToClose(db *sql.DB, org, repo, entity *string, months int) (*VelocitySeries, error) {
	return getVelocitySeries(db, selectTimeToCloseSQL, org, repo, entity, months)
}

func GetTimeToRestoreBugs(db *sql.DB, org, repo, entity *string, months int) (*VelocitySeries, error) {
	return getVelocitySeries(db, selectTimeToRestoreBugsSQL, org, repo, entity, months)
}

func GetPRSizeDistribution(db *sql.DB, org, repo, entity *string, months int) (*PRSizeSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectPRSizeDistributionSQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query PR size distribution: %w", err)
	}
	defer rows.Close()

	s := &PRSizeSeries{
		Months: make([]string, 0),
		Small:  make([]int, 0),
		Medium: make([]int, 0),
		Large:  make([]int, 0),
		XLarge: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var small, medium, large, xlarge int
		if err := rows.Scan(&month, &small, &medium, &large, &xlarge); err != nil {
			return nil, fmt.Errorf("failed to scan PR size row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Small = append(s.Small, small)
		s.Medium = append(s.Medium, medium)
		s.Large = append(s.Large, large)
		s.XLarge = append(s.XLarge, xlarge)
	}

	return s, nil
}

type MomentumSeries struct {
	Months []string `json:"months" yaml:"months"`
	Active []int    `json:"active" yaml:"active"`
	Delta  []int    `json:"delta" yaml:"delta"`
}

type ForksAndActivitySeries struct {
	Months []string `json:"months" yaml:"months"`
	Forks  []int    `json:"forks" yaml:"forks"`
	Events []int    `json:"events" yaml:"events"`
}

type ContributorFunnelSeries struct {
	Months       []string `json:"months" yaml:"months"`
	FirstComment []int    `json:"first_comment" yaml:"firstComment"`
	FirstPR      []int    `json:"first_pr" yaml:"firstPR"`
	FirstMerge   []int    `json:"first_merge" yaml:"firstMerge"`
}

func GetForksAndActivity(db *sql.DB, org, repo, entity *string, months int) (*ForksAndActivitySeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectForksAndActivitySQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query forks and activity: %w", err)
	}
	defer rows.Close()

	s := &ForksAndActivitySeries{
		Months: make([]string, 0),
		Forks:  make([]int, 0),
		Events: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var forks, events int
		if err := rows.Scan(&month, &forks, &events); err != nil {
			return nil, fmt.Errorf("failed to scan forks and activity row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Forks = append(s.Forks, forks)
		s.Events = append(s.Events, events)
	}

	return s, nil
}

func GetContributorFunnel(db *sql.DB, org, repo, entity *string, months int) (*ContributorFunnelSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectContributorFunnelSQL, org, repo, entity, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query contributor funnel: %w", err)
	}
	defer rows.Close()

	s := &ContributorFunnelSeries{
		Months:       make([]string, 0),
		FirstComment: make([]int, 0),
		FirstPR:      make([]int, 0),
		FirstMerge:   make([]int, 0),
	}

	for rows.Next() {
		var month string
		var fc, fp, fm int
		if err := rows.Scan(&month, &fc, &fp, &fm); err != nil {
			return nil, fmt.Errorf("failed to scan contributor funnel row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.FirstComment = append(s.FirstComment, fc)
		s.FirstPR = append(s.FirstPR, fp)
		s.FirstMerge = append(s.FirstMerge, fm)
	}

	return s, nil
}

func GetContributorMomentum(db *sql.DB, org, repo, entity *string, months int) (*MomentumSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectContributorMomentumSQL, since, org, repo, entity)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query contributor momentum: %w", err)
	}
	defer rows.Close()

	s := &MomentumSeries{
		Months: make([]string, 0),
		Active: make([]int, 0),
		Delta:  make([]int, 0),
	}

	for rows.Next() {
		var month string
		var active int
		if err := rows.Scan(&month, &active); err != nil {
			return nil, fmt.Errorf("failed to scan contributor momentum row: %w", err)
		}
		s.Months = append(s.Months, month)
		s.Active = append(s.Active, active)
	}

	// Compute month-over-month delta
	for i := range s.Active {
		if i == 0 {
			s.Delta = append(s.Delta, 0)
		} else {
			s.Delta = append(s.Delta, s.Active[i]-s.Active[i-1])
		}
	}

	return s, nil
}
