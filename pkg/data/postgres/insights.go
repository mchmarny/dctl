package postgres

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	// selectBusFactorSQL: $1=org, $2=repo, $3=entity, $4=since
	selectBusFactorSQL = `WITH dev_counts AS (
			SELECT e.username, COUNT(*) AS cnt
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE($1, e.org)
			  AND e.repo = COALESCE($2, e.repo)
			  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
			  AND e.date >= $4
			  ` + botExcludeSQL + `
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

	// selectPonyFactorSQL: $1=org, $2=repo, $3=entity, $4=since
	selectPonyFactorSQL = `WITH ent_counts AS (
			SELECT d.entity, COUNT(*) AS cnt
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE($1, e.org)
			  AND e.repo = COALESCE($2, e.repo)
			  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
			  AND e.date >= $4
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

	// selectRetentionSQL: $1=org, $2=repo, $3=entity, $4=since, $5=org, $6=repo, $7=entity, $8=since
	selectRetentionSQL = `WITH first_seen AS (
			SELECT e.username, MIN(SUBSTRING(e.date, 1, 7)) AS first_month
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE($1, e.org)
			  AND e.repo = COALESCE($2, e.repo)
			  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
			  AND e.date >= $4
			  ` + botExcludeSQL + `
			GROUP BY e.username
		),
		monthly AS (
			SELECT DISTINCT e.username, SUBSTRING(e.date, 1, 7) AS month
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE($5, e.org)
			  AND e.repo = COALESCE($6, e.repo)
			  AND COALESCE(d.entity, '') = COALESCE($7, COALESCE(d.entity, ''))
			  AND e.date >= $8
			  ` + botExcludeSQL + `
		)
		SELECT m.month,
			SUM(CASE WHEN f.first_month = m.month THEN 1 ELSE 0 END) AS new_contributors,
			SUM(CASE WHEN f.first_month < m.month THEN 1 ELSE 0 END) AS returning_contributors
		FROM monthly m
		JOIN first_seen f ON m.username = f.username
		GROUP BY m.month
		ORDER BY m.month
	`

	// selectTimeToMergeSQL: $1=org, $2=repo, $3=entity, $4=since
	selectTimeToMergeSQL = `SELECT
			SUBSTRING(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(EXTRACT(EPOCH FROM (e.merged_at::timestamp - e.created_at::timestamp)) / 86400.0) AS avg_days
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.merged_at IS NOT NULL
		  AND e.created_at IS NOT NULL
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.created_at >= $4
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	// selectTimeToRestoreBugsSQL: $1=org, $2=repo, $3=entity, $4=since
	selectTimeToRestoreBugsSQL = `SELECT
			SUBSTRING(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(EXTRACT(EPOCH FROM (e.closed_at::timestamp - e.created_at::timestamp)) / 86400.0) AS avg_days
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
		        AND EXTRACT(EPOCH FROM (e.created_at::timestamp - r.published_at::timestamp)) / 86400.0 BETWEEN 0 AND 7
		  )
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.created_at >= $4
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	// selectTimeToCloseSQL: $1=org, $2=repo, $3=entity, $4=since
	selectTimeToCloseSQL = `SELECT
			SUBSTRING(e.created_at, 1, 7) AS month,
			COUNT(*) AS cnt,
			AVG(EXTRACT(EPOCH FROM (e.closed_at::timestamp - e.created_at::timestamp)) / 86400.0) AS avg_days
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.closed_at IS NOT NULL
		  AND e.created_at IS NOT NULL
		  AND e.state = 'closed'
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.created_at >= $4
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	// selectForksAndActivitySQL: $1=org, $2=repo, $3=entity, $4=since
	selectForksAndActivitySQL = `SELECT
			SUBSTRING(e.date, 1, 7) AS month,
			SUM(CASE WHEN e.type = 'fork' THEN 1 ELSE 0 END) AS forks,
			COUNT(*) AS events
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.date >= $4
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	// selectPRReviewRatioSQL: $1=pr_type, $2=review_type, $3=org, $4=repo, $5=entity, $6=since, $7=pr_type, $8=review_type
	selectPRReviewRatioSQL = `SELECT
			SUBSTRING(e.date, 1, 7) AS month,
			SUM(CASE WHEN e.type = $1 THEN 1 ELSE 0 END) AS prs,
			SUM(CASE WHEN e.type = $2 THEN 1 ELSE 0 END) AS reviews
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE($3, e.org)
		  AND e.repo = COALESCE($4, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($5, COALESCE(d.entity, ''))
		  AND e.date >= $6
		  AND e.type IN ($7, $8)
		  ` + botExcludeSQL + `
		GROUP BY month
		ORDER BY month
	`

	// selectChangeFailuresSQL: $1=org, $2=repo, $3=entity, $4=since
	selectChangeFailuresSQL = `SELECT
		SUBSTRING(e.created_at, 1, 7) AS month,
		COUNT(*) AS failures
	FROM event e
	JOIN developer d ON e.username = d.username
	WHERE (
	    (e.type = 'issue' AND LOWER(e.labels) LIKE '%bug%'
	     AND EXISTS (
	        SELECT 1 FROM release r
	        WHERE r.org = e.org AND r.repo = e.repo
	          AND EXTRACT(EPOCH FROM (e.created_at::timestamp - r.published_at::timestamp)) / 86400.0 BETWEEN 0 AND 7
	     ))
	    OR
	    (e.type = 'pr' AND LOWER(e.title) LIKE '%revert%')
	)
	  AND e.org = COALESCE($1, e.org)
	  AND e.repo = COALESCE($2, e.repo)
	  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
	  AND e.created_at >= $4
	  ` + botExcludeSQL + `
	GROUP BY month
	ORDER BY month
	`

	// selectDeploymentCountSQL: $1=org, $2=repo, $3=since
	selectDeploymentCountSQL = `SELECT
		SUBSTRING(published_at, 1, 7) AS month,
		COUNT(*) AS cnt
	FROM release
	WHERE org = COALESCE($1, org)
	  AND repo = COALESCE($2, repo)
	  AND published_at >= $3
	GROUP BY month
	ORDER BY month
	`

	// selectReviewLatencySQL: $1=since, $2=org, $3=repo, $4=entity, $5=since
	selectReviewLatencySQL = `WITH months AS (
		SELECT DISTINCT SUBSTRING(date, 1, 7) AS month
		FROM event
		WHERE date >= $1
	),
	latency AS (
		SELECT
			SUBSTRING(pr.created_at, 1, 7) AS month,
			(EXTRACT(EPOCH FROM (MIN(rev.created_at::timestamp) - MIN(pr.created_at::timestamp))) / 3600.0) AS hours
		FROM event pr
		JOIN event rev ON pr.org = rev.org AND pr.repo = rev.repo AND pr.number = rev.number
			AND rev.type = 'pr_review'
		JOIN developer d ON pr.username = d.username
		WHERE pr.type = 'pr'
		  AND pr.number IS NOT NULL
		  AND pr.created_at IS NOT NULL
		  AND rev.created_at IS NOT NULL
		  AND pr.org = COALESCE($2, pr.org)
		  AND pr.repo = COALESCE($3, pr.repo)
		  AND COALESCE(d.entity, '') = COALESCE($4, COALESCE(d.entity, ''))
		  AND pr.created_at >= $5
		  ` + botExcludePrSQL + `
		GROUP BY pr.org, pr.repo, pr.number, pr.created_at
	)
	SELECT
		m.month,
		COALESCE(COUNT(l.hours), 0) AS cnt,
		COALESCE(AVG(l.hours), 0) AS avg_hours
	FROM months m
	LEFT JOIN latency l ON m.month = l.month
	GROUP BY m.month
	ORDER BY m.month
	`

	// selectPRSizeDistributionSQL: $1=org, $2=repo, $3=entity, $4=since
	selectPRSizeDistributionSQL = `SELECT
		SUBSTRING(e.created_at, 1, 7) AS month,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) < 50 THEN 1 ELSE 0 END) AS small,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 50 AND 249 THEN 1 ELSE 0 END) AS medium,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 250 AND 999 THEN 1 ELSE 0 END) AS large,
		SUM(CASE WHEN COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) >= 1000 THEN 1 ELSE 0 END) AS xlarge
	FROM event e
	JOIN developer d ON e.username = d.username
	WHERE e.type = 'pr'
	  AND e.created_at IS NOT NULL
	  AND e.org = COALESCE($1, e.org)
	  AND e.repo = COALESCE($2, e.repo)
	  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
	  AND e.created_at >= $4
	  ` + botExcludeSQL + `
	GROUP BY month
	ORDER BY month
	`

	// selectContributorMomentumSQL: $1=since, $2=org, $3=repo, $4=entity
	selectContributorMomentumSQL = `WITH months AS (
		SELECT DISTINCT SUBSTRING(date, 1, 7) AS month
		FROM event
		WHERE date >= $1
	)
	SELECT
		m.month,
		COUNT(DISTINCT e.username) AS active
	FROM months m
	JOIN event e ON SUBSTRING(e.date, 1, 7) >= TO_CHAR((m.month || '-01')::date - INTERVAL '2 months', 'YYYY-MM')
		AND SUBSTRING(e.date, 1, 7) <= m.month
	JOIN developer d ON e.username = d.username
	WHERE e.org = COALESCE($2, e.org)
	  AND e.repo = COALESCE($3, e.repo)
	  AND COALESCE(d.entity, '') = COALESCE($4, COALESCE(d.entity, ''))
	  ` + botExcludeSQL + `
	GROUP BY m.month
	ORDER BY m.month
	`

	// selectContributorFunnelSQL: $1=org, $2=repo, $3=entity, $4=since
	selectContributorFunnelSQL = `WITH firsts AS (
		SELECT
			e.username,
			MIN(CASE WHEN e.type = 'issue_comment' THEN e.date END) AS first_comment,
			MIN(CASE WHEN e.type = 'pr' THEN e.date END) AS first_pr,
			MIN(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN e.date END) AS first_merge
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  ` + botExcludeSQL + `
		GROUP BY e.username
	),
	months AS (
		SELECT DISTINCT SUBSTRING(date, 1, 7) AS month FROM event WHERE date >= $4
	)
	SELECT
		m.month,
		SUM(CASE WHEN f.first_comment IS NOT NULL AND SUBSTRING(f.first_comment, 1, 7) = m.month THEN 1 ELSE 0 END) AS fc,
		SUM(CASE WHEN f.first_pr IS NOT NULL AND SUBSTRING(f.first_pr, 1, 7) = m.month THEN 1 ELSE 0 END) AS fp,
		SUM(CASE WHEN f.first_merge IS NOT NULL AND SUBSTRING(f.first_merge, 1, 7) = m.month THEN 1 ELSE 0 END) AS fm
	FROM months m
	CROSS JOIN firsts f
	WHERE SUBSTRING(COALESCE(f.first_comment, f.first_pr, f.first_merge), 1, 7) >= (SELECT MIN(month) FROM months)
	GROUP BY m.month
	HAVING SUM(CASE WHEN f.first_comment IS NOT NULL AND SUBSTRING(f.first_comment, 1, 7) = m.month THEN 1 ELSE 0 END) > 0
	    OR SUM(CASE WHEN f.first_pr IS NOT NULL AND SUBSTRING(f.first_pr, 1, 7) = m.month THEN 1 ELSE 0 END) > 0
	    OR SUM(CASE WHEN f.first_merge IS NOT NULL AND SUBSTRING(f.first_merge, 1, 7) = m.month THEN 1 ELSE 0 END) > 0
	ORDER BY m.month
	`

	// selectContributorProfileSQL:
	// user_counts: $1=username, $2=org, $3=repo, $4=entity, $5=since
	// avg_counts:  $6=org, $7=repo, $8=entity, $9=since
	selectContributorProfileSQL = `WITH user_counts AS (
		SELECT
			SUM(CASE WHEN e.type = 'pr' THEN 1 ELSE 0 END) AS prs_opened,
			SUM(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN 1 ELSE 0 END) AS prs_merged,
			SUM(CASE WHEN e.type = 'pr_review' THEN 1 ELSE 0 END) AS pr_reviews,
			SUM(CASE WHEN e.type = 'issue' THEN 1 ELSE 0 END) AS issues_opened,
			SUM(CASE WHEN e.type = 'issue_comment' THEN 1 ELSE 0 END) AS issue_comments,
			SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) < 50 THEN 1 ELSE 0 END) AS pr_small,
			SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 50 AND 249 THEN 1 ELSE 0 END) AS pr_medium,
			SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 250 AND 999 THEN 1 ELSE 0 END) AS pr_large,
			SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) >= 1000 THEN 1 ELSE 0 END) AS pr_xlarge
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.username = $1
		  AND e.org = COALESCE($2, e.org)
		  AND e.repo = COALESCE($3, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($4, COALESCE(d.entity, ''))
		  AND e.date >= $5
	),
	avg_counts AS (
		SELECT
			AVG(prs_opened) AS prs_opened,
			AVG(prs_merged) AS prs_merged,
			AVG(pr_reviews) AS pr_reviews,
			AVG(issues_opened) AS issues_opened,
			AVG(issue_comments) AS issue_comments,
			AVG(pr_small) AS pr_small,
			AVG(pr_medium) AS pr_medium,
			AVG(pr_large) AS pr_large,
			AVG(pr_xlarge) AS pr_xlarge
		FROM (
			SELECT
				e.username,
				SUM(CASE WHEN e.type = 'pr' THEN 1 ELSE 0 END) AS prs_opened,
				SUM(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN 1 ELSE 0 END) AS prs_merged,
				SUM(CASE WHEN e.type = 'pr_review' THEN 1 ELSE 0 END) AS pr_reviews,
				SUM(CASE WHEN e.type = 'issue' THEN 1 ELSE 0 END) AS issues_opened,
				SUM(CASE WHEN e.type = 'issue_comment' THEN 1 ELSE 0 END) AS issue_comments,
				SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) < 50 THEN 1 ELSE 0 END) AS pr_small,
				SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 50 AND 249 THEN 1 ELSE 0 END) AS pr_medium,
				SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) BETWEEN 250 AND 999 THEN 1 ELSE 0 END) AS pr_large,
				SUM(CASE WHEN e.type = 'pr' AND COALESCE(e.additions, 0) + COALESCE(e.deletions, 0) >= 1000 THEN 1 ELSE 0 END) AS pr_xlarge
			FROM event e
			JOIN developer d ON e.username = d.username
			WHERE e.org = COALESCE($6, e.org)
			  AND e.repo = COALESCE($7, e.repo)
			  AND COALESCE(d.entity, '') = COALESCE($8, COALESCE(d.entity, ''))
			  AND e.date >= $9
			  ` + botExcludeSQL + `
			GROUP BY e.username
		) sub
	)
	SELECT
		COALESCE(u.prs_opened, 0), COALESCE(u.prs_merged, 0), COALESCE(u.pr_reviews, 0),
		COALESCE(u.issues_opened, 0), COALESCE(u.issue_comments, 0),
		COALESCE(u.pr_small, 0), COALESCE(u.pr_medium, 0), COALESCE(u.pr_large, 0), COALESCE(u.pr_xlarge, 0),
		COALESCE(a.prs_opened, 0), COALESCE(a.prs_merged, 0), COALESCE(a.pr_reviews, 0),
		COALESCE(a.issues_opened, 0), COALESCE(a.issue_comments, 0),
		COALESCE(a.pr_small, 0), COALESCE(a.pr_medium, 0), COALESCE(a.pr_large, 0), COALESCE(a.pr_xlarge, 0)
	FROM user_counts u, avg_counts a
	`

	// selectBannerStatsSQL: $1=org, $2=repo, $3=entity, $4=since
	selectBannerStatsSQL = `SELECT
		COUNT(DISTINCT e.org),
		COUNT(DISTINCT e.org || '/' || e.repo),
		COUNT(*),
		COUNT(DISTINCT e.username),
		COALESCE(MAX(e.date), '')
	FROM event e
	WHERE e.org = COALESCE($1, e.org)
	  AND e.repo = COALESCE($2, e.repo)
	  AND COALESCE((SELECT d.entity FROM developer d WHERE d.username = e.username), '') = COALESCE($3, COALESCE((SELECT d.entity FROM developer d WHERE d.username = e.username), ''))
	  AND e.date >= $4
	`

	// selectDailyActivitySQL: $1=org, $2=repo, $3=entity, $4=since
	selectDailyActivitySQL = `SELECT e.date, COUNT(*) AS cnt
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.date >= $4
		  ` + botExcludeSQL + `
		GROUP BY e.date
		ORDER BY e.date
	`
)

func (s *Store) GetInsightsSummary(org, repo, entity *string, months int) (*data.InsightsSummary, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)
	summary := &data.InsightsSummary{}

	if err := s.db.QueryRow(selectBusFactorSQL, org, repo, entity, since).Scan(&summary.BusFactor); err != nil {
		return nil, fmt.Errorf("failed to query bus factor: %w", err)
	}

	if err := s.db.QueryRow(selectPonyFactorSQL, org, repo, entity, since).Scan(&summary.PonyFactor); err != nil {
		return nil, fmt.Errorf("failed to query pony factor: %w", err)
	}

	if err := s.db.QueryRow(selectBannerStatsSQL, org, repo, entity, since).Scan(
		&summary.Orgs, &summary.Repos, &summary.Events, &summary.Contributors, &summary.LastImport,
	); err != nil {
		return nil, fmt.Errorf("failed to query banner stats: %w", err)
	}

	return summary, nil
}

func (s *Store) GetDailyActivity(org, repo, entity *string, months int) (*data.DailyActivitySeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectDailyActivitySQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily activity: %w", err)
	}
	defer rows.Close()

	series := &data.DailyActivitySeries{}
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan daily activity row: %w", err)
		}
		series.Dates = append(series.Dates, date)
		series.Counts = append(series.Counts, count)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return series, nil
}

func (s *Store) GetContributorRetention(org, repo, entity *string, months int) (*data.RetentionSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectRetentionSQL, org, repo, entity, since, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query contributor retention: %w", err)
	}
	defer rows.Close()

	sr := &data.RetentionSeries{
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
		sr.Months = append(sr.Months, month)
		sr.New = append(sr.New, newC)
		sr.Returning = append(sr.Returning, retC)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetPRReviewRatio(org, repo, entity *string, months int) (*data.PRReviewRatioSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectPRReviewRatioSQL,
		data.EventTypePR, data.EventTypePRReview,
		org, repo, entity, since,
		data.EventTypePR, data.EventTypePRReview)
	if err != nil {
		return nil, fmt.Errorf("failed to query PR review ratio: %w", err)
	}
	defer rows.Close()

	sr := &data.PRReviewRatioSeries{
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
		sr.Months = append(sr.Months, month)
		sr.PRs = append(sr.PRs, prs)
		sr.Reviews = append(sr.Reviews, reviews)

		var ratio float64
		if prs > 0 {
			ratio = float64(reviews) / float64(prs)
		}
		sr.Ratio = append(sr.Ratio, ratio)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetChangeFailureRate(org, repo, entity *string, months int) (*data.ChangeFailureRateSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	failureMap := make(map[string]int)

	rows, err := s.db.Query(selectChangeFailuresSQL, org, repo, entity, since)
	if err != nil {
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

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	deployMap := make(map[string]int)

	dRows, err := s.db.Query(selectDeploymentCountSQL, org, repo, since)
	if err != nil {
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

	if err := dRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	monthSet := make(map[string]bool)
	for m := range failureMap {
		monthSet[m] = true
	}
	for m := range deployMap {
		monthSet[m] = true
	}

	sortedMonths := make([]string, 0, len(monthSet))
	for m := range monthSet {
		sortedMonths = append(sortedMonths, m)
	}
	sort.Strings(sortedMonths)

	sr := &data.ChangeFailureRateSeries{
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
		sr.Months = append(sr.Months, m)
		sr.Failures = append(sr.Failures, f)
		sr.Deployments = append(sr.Deployments, d)
		sr.Rate = append(sr.Rate, rate)
	}

	return sr, nil
}

func (s *Store) GetReviewLatency(org, repo, entity *string, months int) (*data.ReviewLatencySeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectReviewLatencySQL, since, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query review latency: %w", err)
	}
	defer rows.Close()

	sr := &data.ReviewLatencySeries{
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
		sr.Months = append(sr.Months, month)
		sr.Count = append(sr.Count, cnt)
		sr.AvgHours = append(sr.AvgHours, avgHours)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) getVelocitySeries(query string, org, repo, entity *string, months int) (*data.VelocitySeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(query, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query velocity series: %w", err)
	}
	defer rows.Close()

	sr := &data.VelocitySeries{
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
		sr.Months = append(sr.Months, month)
		sr.Count = append(sr.Count, cnt)
		sr.AvgDays = append(sr.AvgDays, avgDays)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetTimeToMerge(org, repo, entity *string, months int) (*data.VelocitySeries, error) {
	return s.getVelocitySeries(selectTimeToMergeSQL, org, repo, entity, months)
}

func (s *Store) GetTimeToClose(org, repo, entity *string, months int) (*data.VelocitySeries, error) {
	return s.getVelocitySeries(selectTimeToCloseSQL, org, repo, entity, months)
}

func (s *Store) GetTimeToRestoreBugs(org, repo, entity *string, months int) (*data.VelocitySeries, error) {
	return s.getVelocitySeries(selectTimeToRestoreBugsSQL, org, repo, entity, months)
}

func (s *Store) GetPRSizeDistribution(org, repo, entity *string, months int) (*data.PRSizeSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectPRSizeDistributionSQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query PR size distribution: %w", err)
	}
	defer rows.Close()

	sr := &data.PRSizeSeries{
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
		sr.Months = append(sr.Months, month)
		sr.Small = append(sr.Small, small)
		sr.Medium = append(sr.Medium, medium)
		sr.Large = append(sr.Large, large)
		sr.XLarge = append(sr.XLarge, xlarge)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetForksAndActivity(org, repo, entity *string, months int) (*data.ForksAndActivitySeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectForksAndActivitySQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query forks and activity: %w", err)
	}
	defer rows.Close()

	sr := &data.ForksAndActivitySeries{
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
		sr.Months = append(sr.Months, month)
		sr.Forks = append(sr.Forks, forks)
		sr.Events = append(sr.Events, events)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetContributorFunnel(org, repo, entity *string, months int) (*data.ContributorFunnelSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectContributorFunnelSQL, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query contributor funnel: %w", err)
	}
	defer rows.Close()

	sr := &data.ContributorFunnelSeries{
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
		sr.Months = append(sr.Months, month)
		sr.FirstComment = append(sr.FirstComment, fc)
		sr.FirstPR = append(sr.FirstPR, fp)
		sr.FirstMerge = append(sr.FirstMerge, fm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}

func (s *Store) GetContributorMomentum(org, repo, entity *string, months int) (*data.MomentumSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectContributorMomentumSQL, since, org, repo, entity)
	if err != nil {
		return nil, fmt.Errorf("failed to query contributor momentum: %w", err)
	}
	defer rows.Close()

	sr := &data.MomentumSeries{
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
		sr.Months = append(sr.Months, month)
		sr.Active = append(sr.Active, active)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	for i := range sr.Active {
		if i == 0 {
			sr.Delta = append(sr.Delta, 0)
		} else {
			sr.Delta = append(sr.Delta, sr.Active[i]-sr.Active[i-1])
		}
	}

	return sr, nil
}

func (s *Store) GetContributorProfile(username string, org, repo, entity *string, months int) (*data.ContributorProfileSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	since := sinceDate(months)

	var prs, prsMerged, reviews, issues, comments int
	var prSmall, prMedium, prLarge, prXLarge int
	var avgPrs, avgMerged, avgReviews, avgIssues, avgComments float64
	var avgSmall, avgMedium, avgLarge, avgXLarge float64

	err := s.db.QueryRow(selectContributorProfileSQL,
		username, org, repo, entity, since,
		org, repo, entity, since,
	).Scan(
		&prs, &prsMerged, &reviews, &issues, &comments,
		&prSmall, &prMedium, &prLarge, &prXLarge,
		&avgPrs, &avgMerged, &avgReviews, &avgIssues, &avgComments,
		&avgSmall, &avgMedium, &avgLarge, &avgXLarge,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query contributor profile: %w", err)
	}

	result := &data.ContributorProfileSeries{
		Metrics: []string{"PRs Opened", "PRs Merged", "PR Reviews", "Issues Opened", "Issue Comments",
			"PR Size S", "PR Size M", "PR Size L", "PR Size XL"},
		Values:   []int{prs, prsMerged, reviews, issues, comments, prSmall, prMedium, prLarge, prXLarge},
		Averages: []float64{avgPrs, avgMerged, avgReviews, avgIssues, avgComments, avgSmall, avgMedium, avgLarge, avgXLarge},
	}

	var rep sql.NullFloat64
	if scanErr := s.db.QueryRow(`SELECT reputation FROM developer WHERE username = $1`, username).Scan(&rep); scanErr == nil && rep.Valid {
		result.Reputation = &rep.Float64
	}

	return result, nil
}
