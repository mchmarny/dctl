# DORA-Inspired Metrics Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 8 developer velocity metrics (4 DORA proxies + 4 GitHub-native) to DevPulse dashboard.

**Architecture:** New SQL queries against existing `event` and `release` tables, exposed as JSON API endpoints, rendered as Chart.js panels. One migration adds `title` column to `event` table for revert PR detection.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Chart.js, testify

**Design doc:** `docs/plans/2026-03-13-dora-metrics-design.md`

---

### Task 1: Migration — Add `title` column to `event` table

**Files:**
- Create: `pkg/data/sql/migrations/010_event_title.sql`

**Step 1: Create migration file**

```sql
ALTER TABLE event ADD COLUMN title TEXT NOT NULL DEFAULT '';
```

**Step 2: Run tests to verify migration applies cleanly**

Run: `make test`
Expected: PASS — `setupTestDB(t)` runs all migrations including the new one.

**Step 3: Commit**

```
feat: add title column to event table (migration 010)
```

---

### Task 2: Import — Capture title during PR and issue import

**Files:**
- Modify: `pkg/data/event.go` — add `Title` field to `Event` struct, update `insertEventSQL`, update `importPREvents` and `importIssueEvents`

**Step 1: Write failing test**

File: `pkg/data/event_test.go`

Add a test that inserts an event with a title and verifies it's stored:

```go
func TestEventTitleStorage(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, title)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'Add new feature')`)
	require.NoError(t, err)

	var title string
	err = db.QueryRow(`SELECT title FROM event WHERE org='org1' AND repo='repo1' AND username='alice'`).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Add new feature", title)
}
```

**Step 2: Run test to verify it passes** (migration already added the column)

Run: `make test`
Expected: PASS

**Step 3: Update Event struct and SQL**

In `pkg/data/event.go`:

1. Add `Title` field to `Event` struct:
```go
Title     string  `json:"title,omitempty" yaml:"title,omitempty"`
```

2. Update `insertEventSQL` to include `title` in INSERT columns and VALUES, and in ON CONFLICT DO UPDATE SET clause.

3. In `importPREvents` function, set `Title` from `items[i].GetTitle()`.

4. In `importIssueEvents` function, set `Title` from `items[i].GetTitle()`.

5. Update the `insertEvent` function's `db.Exec` call to pass `e.Title` as a parameter.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```
feat: capture PR and issue titles during import
```

---

### Task 3: Deployment Frequency — Extend release cadence with merge fallback

**Files:**
- Modify: `pkg/data/release.go` — add `Deployments` field to `ReleaseCadenceSeries`, add fallback query
- Modify: `pkg/data/release_test.go` — add tests

**Step 1: Write failing tests**

File: `pkg/data/release_test.go`

```go
func TestGetReleaseCadence_WithDeployments(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Insert releases
	_, err = db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES ('org1', 'repo1', 'v1.0', 'v1.0', '2025-01-15T00:00:00Z', 0)`)
	require.NoError(t, err)

	series, err := GetReleaseCadence(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Equal(t, 1, series.Deployments[0])
}

func TestGetReleaseCadence_MergeFallback(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// No releases — insert merged PRs instead
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, merged_at)
		VALUES ('org2', 'repo2', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'merged', '2025-01-10T10:00:00Z', '2025-01-10T12:00:00Z'),
		       ('org2', 'repo2', 'alice', 'pr', '2025-01-11', 'http://b', '', '', 'merged', '2025-01-11T10:00:00Z', '2025-01-11T12:00:00Z')`)
	require.NoError(t, err)

	series, err := GetReleaseCadence(db, strPtr("org2"), strPtr("repo2"), nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Equal(t, 2, series.Deployments[0])
}
```

Add a `strPtr` helper if not already present:
```go
func strPtr(s string) *string { return &s }
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `Deployments` field doesn't exist yet.

**Step 3: Implement**

In `pkg/data/release.go`:

1. Add `Deployments []int` field to `ReleaseCadenceSeries`:
```go
type ReleaseCadenceSeries struct {
	Months      []string `json:"months" yaml:"months"`
	Total       []int    `json:"total" yaml:"total"`
	Stable      []int    `json:"stable" yaml:"stable"`
	Deployments []int    `json:"deployments" yaml:"deployments"`
}
```

2. Add a new SQL query for merged PR deployments:
```go
selectMergedPRDeploymentsSQL = `SELECT
	substr(e.merged_at, 1, 7) AS month,
	COUNT(*) AS cnt
FROM event e
JOIN developer d ON e.username = d.username
WHERE e.type = 'pr'
  AND e.state = 'merged'
  AND e.merged_at IS NOT NULL
  AND e.org = COALESCE(?, e.org)
  AND e.repo = COALESCE(?, e.repo)
  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
  AND e.merged_at >= ?
  AND e.username NOT LIKE '%[bot]'
  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
GROUP BY month
ORDER BY month
`
```

3. Update `GetReleaseCadence` to accept `entity *string` parameter and:
   - Run the existing release cadence query first
   - If releases exist, set `Deployments = Total` (each release = 1 deployment)
   - If no releases exist for the org/repo, run `selectMergedPRDeploymentsSQL` as fallback
   - Merge the deployment counts into the series by month

4. Update the handler in `pkg/cli/data.go` to pass entity param.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Run qualify**

Run: `make qualify`
Expected: PASS

**Step 6: Commit**

```
feat: add deployment frequency with merge-to-main fallback
```

---

### Task 4: Change Failure Rate — New endpoint

**Files:**
- Modify: `pkg/data/insights.go` — add SQL, response type, query function
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — add handler
- Modify: `pkg/cli/server.go` — add route

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetChangeFailureRate_NilDB(t *testing.T) {
	_, err := GetChangeFailureRate(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetChangeFailureRate_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetChangeFailureRate(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetChangeFailureRate_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Insert a release (deployment)
	_, err = db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES ('org1', 'repo1', 'v1.0', 'v1.0', '2025-01-15T00:00:00Z', 0)`)
	require.NoError(t, err)

	// Insert a bug issue within 7 days of release
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, title)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-17', 'http://a', '', 'bug', 'open', '2025-01-17T10:00:00Z', 'Bug in feature')`)
	require.NoError(t, err)

	// Insert a revert PR
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, title)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-16', 'http://b', '', '', 'merged', '2025-01-16T10:00:00Z', 'Revert "Add feature"')`)
	require.NoError(t, err)

	series, err := GetChangeFailureRate(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Greater(t, series.Failures[0], 0)
	assert.Greater(t, series.Deployments[0], 0)
	assert.Greater(t, series.Rate[0], 0.0)
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetChangeFailureRate` not defined.

**Step 3: Implement response type and query**

In `pkg/data/insights.go`:

```go
type ChangeFailureRateSeries struct {
	Months      []string  `json:"months" yaml:"months"`
	Failures    []int     `json:"failures" yaml:"failures"`
	Deployments []int     `json:"deployments" yaml:"deployments"`
	Rate        []float64 `json:"rate" yaml:"rate"`
}
```

SQL for failures (bug issues within 7 days of a release + revert PRs):

```go
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
```

SQL for deployment counts (releases, with merged PR fallback — reuse logic from Task 3):

```go
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
```

Function `GetChangeFailureRate(db *sql.DB, org, repo, entity *string, months int)`:
1. Query failures by month
2. Query deployments by month (releases, with merged PR fallback)
3. Merge into `ChangeFailureRateSeries` with rate = failures/deployments * 100

**Step 4: Add handler and route**

In `pkg/cli/data.go`:
```go
func insightsChangeFailureRateAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetChangeFailureRate(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get change failure rate", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying change failure rate")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

In `pkg/cli/server.go`, add route:
```go
mux.HandleFunc("GET /data/insights/change-failure-rate", insightsChangeFailureRateAPIHandler(db))
```

**Step 5: Run tests**

Run: `make test`
Expected: PASS

**Step 6: Run qualify**

Run: `make qualify`
Expected: PASS

**Step 7: Commit**

```
feat: add change failure rate metric
```

---

### Task 5: Time to Restore — Extend time-to-close with bug-only filter

**Files:**
- Modify: `pkg/data/insights.go` — add bug-only SQL variant, update function signature
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — update handler to read `bug_only` param

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetTimeToClose_BugOnly(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Insert a release
	_, err = db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease)
		VALUES ('org1', 'repo1', 'v1.0', 'v1.0', '2025-01-15T00:00:00Z', 0)`)
	require.NoError(t, err)

	// Bug issue near release, closed
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-17', 'http://a', '', 'bug', 'closed', '2025-01-17T10:00:00Z', '2025-01-18T10:00:00Z')`)
	require.NoError(t, err)

	// Non-bug issue, closed
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES ('org1', 'repo1', 'alice', 'issue', '2025-01-17', 'http://b', '', 'enhancement', 'closed', '2025-01-17T10:00:00Z', '2025-01-20T10:00:00Z')`)
	require.NoError(t, err)

	series, err := GetTimeToClose(db, nil, nil, nil, 24, true)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	// Should only include the bug issue (1 day), not the enhancement (3 days)
	assert.InDelta(t, 1.0, series.AvgDays[0], 0.01)
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetTimeToClose` doesn't accept `bugOnly` param.

**Step 3: Implement**

In `pkg/data/insights.go`:

1. Add a new SQL constant `selectTimeToCloseBugOnlySQL` that adds:
```sql
AND LOWER(e.labels) LIKE '%bug%'
AND EXISTS (
    SELECT 1 FROM release r
    WHERE r.org = e.org AND r.repo = e.repo
      AND julianday(e.created_at) - julianday(r.published_at) BETWEEN 0 AND 7
)
```

2. Update `GetTimeToClose` signature to `GetTimeToClose(db *sql.DB, org, repo, entity *string, months int, bugOnly bool)`:
   - If `bugOnly` is true, use the bug-only SQL variant
   - Otherwise use the existing SQL

3. Update existing callers (handler) to pass `bugOnly = false` by default.

4. Update handler to read `bug_only` query param:
```go
bugOnly := r.URL.Query().Get("bug_only") == "true"
```

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```
feat: add bug-only filter to time-to-close for DORA time-to-restore
```

---

### Task 6: Review Latency — New endpoint

**Files:**
- Modify: `pkg/data/insights.go` — add SQL, function
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — add handler
- Modify: `pkg/cli/server.go` — add route

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetReviewLatency_NilDB(t *testing.T) {
	_, err := GetReviewLatency(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReviewLatency_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetReviewLatency(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetReviewLatency_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// PR created
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, number, created_at)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'open', 42, '2025-01-10T10:00:00Z')`)
	require.NoError(t, err)

	// PR review 6 hours later
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, number, created_at)
		VALUES ('org1', 'repo1', 'bob', 'pr_review', '2025-01-10', 'http://b', '', '', 42, '2025-01-10T16:00:00Z')`)
	require.NoError(t, err)

	series, err := GetReviewLatency(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.InDelta(t, 6.0, series.AvgHours[0], 0.1)
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetReviewLatency` not defined.

**Step 3: Implement**

In `pkg/data/insights.go`:

Response type:
```go
type ReviewLatencySeries struct {
	Months   []string  `json:"months" yaml:"months"`
	Count    []int     `json:"count" yaml:"count"`
	AvgHours []float64 `json:"avg_hours" yaml:"avgHours"`
}
```

SQL:
```go
selectReviewLatencySQL = `SELECT
	substr(pr.created_at, 1, 7) AS month,
	COUNT(*) AS cnt,
	AVG((julianday(MIN(rev.created_at)) - julianday(pr.created_at)) * 24) AS avg_hours
FROM event pr
JOIN event rev ON pr.org = rev.org AND pr.repo = rev.repo AND pr.number = rev.number
    AND rev.type = 'pr_review'
JOIN developer d ON pr.username = d.username
WHERE pr.type = 'pr'
  AND pr.created_at IS NOT NULL
  AND rev.created_at IS NOT NULL
  AND pr.org = COALESCE(?, pr.org)
  AND pr.repo = COALESCE(?, pr.repo)
  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
  AND pr.created_at >= ?
  AND pr.username NOT LIKE '%[bot]'
  AND pr.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
GROUP BY month
ORDER BY month
`
```

Note: The MIN(rev.created_at) gets the first review per PR. The GROUP BY needs to include the PR identifier to get MIN per PR, then aggregate by month. This may need a subquery:

```go
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
```

Function:
```go
func GetReviewLatency(db *sql.DB, org, repo, entity *string, months int) (*ReviewLatencySeries, error)
```

Follow the `getVelocitySeries` pattern but scan into `ReviewLatencySeries`.

**Step 4: Add handler and route**

Handler in `pkg/cli/data.go`, route in `pkg/cli/server.go`:
```go
mux.HandleFunc("GET /data/insights/review-latency", insightsReviewLatencyAPIHandler(db))
```

**Step 5: Run tests**

Run: `make test`
Expected: PASS

**Step 6: Commit**

```
feat: add review latency metric
```

---

### Task 7: PR Size Distribution — New endpoint

**Files:**
- Modify: `pkg/data/insights.go` — add SQL, response type, query function
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — add handler
- Modify: `pkg/cli/server.go` — add route

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetPRSizeDistribution_NilDB(t *testing.T) {
	_, err := GetPRSizeDistribution(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetPRSizeDistribution_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetPRSizeDistribution(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetPRSizeDistribution_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	// Small PR (20 lines)
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', '', 'merged', '2025-01-10T10:00:00Z', 15, 5)`)
	require.NoError(t, err)

	// Large PR (500 lines)
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, additions, deletions)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-11', 'http://b', '', '', 'merged', '2025-01-11T10:00:00Z', 400, 100)`)
	require.NoError(t, err)

	series, err := GetPRSizeDistribution(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Equal(t, 1, series.Small[0])
	assert.Equal(t, 0, series.Medium[0])
	assert.Equal(t, 1, series.Large[0])
	assert.Equal(t, 0, series.XLarge[0])
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetPRSizeDistribution` not defined.

**Step 3: Implement**

In `pkg/data/insights.go`:

Response type:
```go
type PRSizeSeries struct {
	Months []string `json:"months" yaml:"months"`
	Small  []int    `json:"small" yaml:"small"`
	Medium []int    `json:"medium" yaml:"medium"`
	Large  []int    `json:"large" yaml:"large"`
	XLarge []int    `json:"xlarge" yaml:"xlarge"`
}
```

SQL:
```go
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
```

Function:
```go
func GetPRSizeDistribution(db *sql.DB, org, repo, entity *string, months int) (*PRSizeSeries, error)
```

**Step 4: Add handler and route**

```go
mux.HandleFunc("GET /data/insights/pr-size", insightsPRSizeAPIHandler(db))
```

**Step 5: Run tests**

Run: `make test`
Expected: PASS

**Step 6: Commit**

```
feat: add PR size distribution metric
```

---

### Task 8: Contributor Momentum — New endpoint

**Files:**
- Modify: `pkg/data/insights.go` — add SQL, response type, query function
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — add handler
- Modify: `pkg/cli/server.go` — add route

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetContributorMomentum_NilDB(t *testing.T) {
	_, err := GetContributorMomentum(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorMomentum_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetContributorMomentum(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorMomentum_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Both active in Jan
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://a', '', ''),
		       ('org1', 'repo1', 'bob', 'pr', '2025-01-11', 'http://b', '', '')`)
	require.NoError(t, err)

	// Only alice in Feb
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2025-02-10', 'http://c', '', '')`)
	require.NoError(t, err)

	series, err := GetContributorMomentum(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 2)
	assert.Equal(t, 2, series.Active[0]) // Jan: 2 contributors
	assert.Equal(t, 1, series.Active[1]) // Feb: 1 contributor
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetContributorMomentum` not defined.

**Step 3: Implement**

In `pkg/data/insights.go`:

Response type:
```go
type MomentumSeries struct {
	Months []string `json:"months" yaml:"months"`
	Active []int    `json:"active" yaml:"active"`
	Delta  []int    `json:"delta" yaml:"delta"`
}
```

SQL (rolling 3-month window):
```go
selectContributorMomentumSQL = `SELECT
	m.month,
	COUNT(DISTINCT e.username) AS active
FROM (
    SELECT DISTINCT substr(date, 1, 7) AS month FROM event WHERE date >= ?
) m
JOIN event e ON substr(e.date, 1, 7) BETWEEN
    substr(date(m.month || '-01', '-2 months'), 1, 7) AND m.month
JOIN developer d ON e.username = d.username
WHERE e.org = COALESCE(?, e.org)
  AND e.repo = COALESCE(?, e.repo)
  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
  AND e.username NOT LIKE '%[bot]'
  AND e.username NOT IN ('copilot','github-copilot','claude','anthropic-claude')
GROUP BY m.month
ORDER BY m.month
`
```

Function computes `Delta` as month-over-month difference after querying.

```go
func GetContributorMomentum(db *sql.DB, org, repo, entity *string, months int) (*MomentumSeries, error)
```

**Step 4: Add handler and route**

```go
mux.HandleFunc("GET /data/insights/contributor-momentum", insightsContributorMomentumAPIHandler(db))
```

**Step 5: Run tests**

Run: `make test`
Expected: PASS

**Step 6: Commit**

```
feat: add contributor momentum metric
```

---

### Task 9: First-Time Contributor Funnel — New endpoint

**Files:**
- Modify: `pkg/data/insights.go` — add SQL, response type, query function
- Modify: `pkg/data/insights_test.go` — add tests
- Modify: `pkg/cli/data.go` — add handler
- Modify: `pkg/cli/server.go` — add route

**Step 1: Write failing tests**

File: `pkg/data/insights_test.go`

```go
func TestGetContributorFunnel_NilDB(t *testing.T) {
	_, err := GetContributorFunnel(nil, nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorFunnel_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetContributorFunnel(db, nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetContributorFunnel_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Alice: first comment in Jan, first PR in Jan, first merge in Jan
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, merged_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue_comment', '2025-01-05', 'http://a', '', '', NULL, '2025-01-05T10:00:00Z', NULL),
		('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://b', '', '', 'merged', '2025-01-10T10:00:00Z', '2025-01-10T12:00:00Z')`)
	require.NoError(t, err)

	// Bob: first comment in Jan only
	_, err = db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'issue_comment', '2025-01-08', 'http://c', '', '')`)
	require.NoError(t, err)

	series, err := GetContributorFunnel(db, nil, nil, nil, 24)
	require.NoError(t, err)
	require.NotEmpty(t, series.Months)
	assert.Equal(t, 2, series.FirstComment[0]) // Alice + Bob
	assert.Equal(t, 1, series.FirstPR[0])      // Alice only
	assert.Equal(t, 1, series.FirstMerge[0])   // Alice only
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `GetContributorFunnel` not defined.

**Step 3: Implement**

In `pkg/data/insights.go`:

Response type:
```go
type ContributorFunnelSeries struct {
	Months       []string `json:"months" yaml:"months"`
	FirstComment []int    `json:"first_comment" yaml:"firstComment"`
	FirstPR      []int    `json:"first_pr" yaml:"firstPR"`
	FirstMerge   []int    `json:"first_merge" yaml:"firstMerge"`
}
```

SQL (uses MIN to find first event per type per user, then counts by month):
```go
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
)
SELECT
    month,
    SUM(CASE WHEN first_comment IS NOT NULL AND substr(first_comment, 1, 7) = month THEN 1 ELSE 0 END) AS fc,
    SUM(CASE WHEN first_pr IS NOT NULL AND substr(first_pr, 1, 7) = month THEN 1 ELSE 0 END) AS fp,
    SUM(CASE WHEN first_merge IS NOT NULL AND substr(first_merge, 1, 7) = month THEN 1 ELSE 0 END) AS fm
FROM firsts
CROSS JOIN (SELECT DISTINCT substr(date, 1, 7) AS month FROM event WHERE date >= ?)
WHERE (first_comment IS NOT NULL AND substr(first_comment, 1, 7) = month)
   OR (first_pr IS NOT NULL AND substr(first_pr, 1, 7) = month)
   OR (first_merge IS NOT NULL AND substr(first_merge, 1, 7) = month)
GROUP BY month
ORDER BY month
`
```

Function:
```go
func GetContributorFunnel(db *sql.DB, org, repo, entity *string, months int) (*ContributorFunnelSeries, error)
```

**Step 4: Add handler and route**

```go
mux.HandleFunc("GET /data/insights/contributor-funnel", insightsContributorFunnelAPIHandler(db))
```

**Step 5: Run tests**

Run: `make test`
Expected: PASS

**Step 6: Run qualify**

Run: `make qualify`
Expected: PASS

**Step 7: Commit**

```
feat: add first-time contributor funnel metric
```

---

### Task 10: Dashboard — Update existing panels

**Files:**
- Modify: `pkg/cli/templates/home.html` — update panel labels, add new data series to existing charts

**Step 1: Update Release Cadence panel**

Add "Deployments" line series to the Release Cadence chart. Fetch from the extended endpoint. Use a dashed line style to distinguish from Total and Stable.

**Step 2: Update Time to Merge panel**

Rename header from "Time to Merge (PRs)" to "Lead Time (PR to Merge)". Add tooltip text explaining DORA context.

**Step 3: Update Time to Close panel**

Add a second fetch to `/data/insights/time-to-close?bug_only=true`. Render as an additional "Bug Resolution" line series on the same chart (different color).

**Step 4: Update PR Review Ratio panel**

Add a second Y-axis (right) for review latency hours. Fetch from `/data/insights/review-latency`. Render as a line series on the right axis.

**Step 5: Update Contributor Retention panel**

Add contributor momentum as a line overlay. Fetch from `/data/insights/contributor-momentum`. Render on a second Y-axis showing active count + delta annotation.

**Step 6: Run server and visually verify**

Run: `make server`
Expected: All panels render correctly with new data series.

**Step 7: Commit**

```
feat: update existing dashboard panels with DORA metrics
```

---

### Task 11: Dashboard — Add new panels

**Files:**
- Modify: `pkg/cli/templates/home.html` — add 3 new chart panels

**Step 1: Add Change Failure Rate panel**

```html
<article>
    <div class="tbl">
        <div class="content-header">
            Change Failure Rate
        </div>
        <div class="tbl-chart tbl-home">
            <canvas class="chart" id="change-failure-rate-chart"></canvas>
        </div>
        <span class="insight-desc">Percentage of deployments causing failures. Based on bug issues near releases and revert PRs.</span>
    </div>
</article>
```

Chart type: Line chart showing rate% by month. Add JavaScript to fetch `/data/insights/change-failure-rate` and render.

**Step 2: Add PR Size Distribution panel**

```html
<article>
    <div class="tbl">
        <div class="content-header">
            PR Size Distribution
        </div>
        <div class="tbl-chart tbl-home">
            <canvas class="chart" id="pr-size-chart"></canvas>
        </div>
        <span class="insight-desc">Pull request size by lines changed. S (&lt;50), M (50-250), L (250-1000), XL (&gt;1000).</span>
    </div>
</article>
```

Chart type: Stacked bar chart showing S/M/L/XL counts per month.

**Step 3: Add First-Time Contributor Funnel panel**

```html
<article>
    <div class="tbl">
        <div class="content-header">
            First-Time Contributors
        </div>
        <div class="tbl-chart tbl-home">
            <canvas class="chart" id="contributor-funnel-chart"></canvas>
        </div>
        <span class="insight-desc">New contributor milestones per month: first comment, first PR, first merged PR.</span>
    </div>
</article>
```

Chart type: Grouped bar chart showing 3 bars per month (comment/PR/merge).

**Step 4: Add JavaScript for all 3 new charts**

Follow the existing pattern: `fetch()` → parse JSON → `new Chart()` with consistent color scheme and responsive options.

**Step 5: Run server and visually verify all panels**

Run: `make server`
Expected: All 14 panels render correctly.

**Step 6: Run qualify**

Run: `make qualify`
Expected: PASS

**Step 7: Commit**

```
feat: add change failure rate, PR size, and contributor funnel panels
```

---

### Task 12: Final verification

**Step 1: Run full qualify**

Run: `make qualify`
Expected: PASS — all tests, lint, vulnerability scan clean.

**Step 2: Run server with test data and verify all panels**

Run: `make server`
Expected: Dashboard loads, all 14 panels render, no console errors.

**Step 3: Final commit if any cleanup needed**

---

## Unresolved Questions

1. **Contributor momentum rolling window SQL** — The 3-month rolling window join may need tuning for performance on large datasets. Consider materializing month boundaries if queries are slow.
2. **Revert PR detection accuracy** — `LOWER(title) LIKE '%revert%'` may have false positives (e.g., "Revert to old API" that isn't a revert PR). Could tighten with `LIKE 'Revert "%'` pattern to match GitHub's default revert title format.
3. **Change failure rate with no deployments** — When a month has failures but zero deployments, rate should be 0 (not divide-by-zero). Guard in SQL with `CASE WHEN deployments = 0 THEN 0 ELSE ...`.
