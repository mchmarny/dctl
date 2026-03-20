# Dashboard Gap Closure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add three new dashboard features: issue open/close ratio (Activity tab), time to first response (Velocity tab), and community profile metadata (Health tab repo metadata card).

**Architecture:** Each feature follows the existing pattern: SQL query → Store method → HTTP handler → JS chart loader. Features 1 and 3 use existing data or simple API additions. Feature 2 requires a data collection fix (parsing issue number from comment URLs) plus a backfill migration.

**Tech Stack:** Go, SQLite/PostgreSQL, Chart.js, GitHub REST API (community profile endpoint)

---

## Task 1: Issue Open/Close Ratio — Data Types and Store Interface

**Files:**
- Modify: `pkg/data/types.go` (add struct near line 228, after VelocitySeries)
- Modify: `pkg/data/store.go` (add method to InsightsStore at line 94)

**Step 1: Add the response struct to types.go**

Add after the `VelocitySeries` struct (line 232):

```go
type IssueRatioSeries struct {
	Months []string `json:"months" yaml:"months"`
	Opened []int    `json:"opened" yaml:"opened"`
	Closed []int    `json:"closed" yaml:"closed"`
}
```

**Step 2: Add the Store method signature**

Add to `InsightsStore` interface (around line 94, before the closing brace):

```go
GetIssueOpenCloseRatio(org, repo, entity *string, months int) (*IssueRatioSeries, error)
```

**Step 3: Verify compilation**

Run: `go build ./...`
Expected: FAIL — Store implementations don't satisfy the interface yet.

**Step 4: Commit**

```bash
git add pkg/data/types.go pkg/data/store.go
git commit -S -m "feat: add IssueRatioSeries type and Store interface method"
```

---

## Task 2: Issue Open/Close Ratio — SQLite Implementation

**Files:**
- Modify: `pkg/data/sqlite/insights.go` (add SQL constant + method)
- Modify: `pkg/data/sqlite/insights_test.go` (add tests)

**Step 1: Write the failing tests**

Add to `insights_test.go`:

```go
func TestGetIssueOpenCloseRatio_NilDB(t *testing.T) {
	s := &Store{}
	_, err := s.GetIssueOpenCloseRatio(nil, nil, nil, 6)
	require.ErrorIs(t, err, data.ErrDBNotInitialized)
}

func TestGetIssueOpenCloseRatio_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetIssueOpenCloseRatio(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetIssueOpenCloseRatio_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice')`)
	require.NoError(t, err)

	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, created_at, closed_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue', '2025-01-10', 'http://a', '', '', 'open', '2025-01-10T00:00:00Z', NULL),
		('org1', 'repo1', 'alice', 'issue', '2025-01-12', 'http://b', '', '', 'closed', '2025-01-05T00:00:00Z', '2025-01-12T00:00:00Z'),
		('org1', 'repo1', 'alice', 'issue', '2025-01-15', 'http://c', '', '', 'closed', '2025-01-08T00:00:00Z', '2025-01-15T00:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetIssueOpenCloseRatio(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.Equal(t, 3, series.Opened[0])  // 3 issues created in Jan
	assert.Equal(t, 2, series.Closed[0])  // 2 issues closed in Jan
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/data/sqlite/ -run TestGetIssueOpenCloseRatio -v`
Expected: FAIL — method not defined.

**Step 3: Add the SQL constant and implementation**

Add SQL constant at top of `insights.go` (alongside other SELECT constants):

```go
selectIssueOpenCloseRatioSQL = `SELECT month, SUM(opened) AS opened, SUM(closed) AS closed
	FROM (
		SELECT substr(e.created_at, 1, 7) AS month, 1 AS opened, 0 AS closed
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.created_at IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  ` + botExcludeSQL + `
		UNION ALL
		SELECT substr(e.closed_at, 1, 7) AS month, 0 AS opened, 1 AS closed
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.closed_at IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.closed_at >= ?
		  ` + botExcludeSQL + `
	) sub
	GROUP BY month
	ORDER BY month
`
```

Add implementation method:

```go
func (s *Store) GetIssueOpenCloseRatio(org, repo, entity *string, months int) (*data.IssueRatioSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectIssueOpenCloseRatioSQL, org, repo, entity, since, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query issue open/close ratio: %w", err)
	}
	defer rows.Close()

	sr := &data.IssueRatioSeries{
		Months: make([]string, 0),
		Opened: make([]int, 0),
		Closed: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var opened, closed int
		if err := rows.Scan(&month, &opened, &closed); err != nil {
			return nil, fmt.Errorf("failed to scan issue ratio row: %w", err)
		}
		sr.Months = append(sr.Months, month)
		sr.Opened = append(sr.Opened, opened)
		sr.Closed = append(sr.Closed, closed)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/data/sqlite/ -run TestGetIssueOpenCloseRatio -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/data/sqlite/insights.go pkg/data/sqlite/insights_test.go
git commit -S -m "feat: implement issue open/close ratio query for SQLite"
```

---

## Task 3: Issue Open/Close Ratio — PostgreSQL Implementation

**Files:**
- Modify: `pkg/data/postgres/insights.go` (add SQL constant + method)

**Step 1: Add the SQL constant and implementation**

Same pattern as SQLite but with PostgreSQL syntax:

```go
selectIssueOpenCloseRatioSQL = `SELECT month, SUM(opened) AS opened, SUM(closed) AS closed
	FROM (
		SELECT SUBSTRING(e.created_at, 1, 7) AS month, 1 AS opened, 0 AS closed
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.created_at IS NOT NULL
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.created_at >= $4
		  ` + botExcludeSQL + `
		UNION ALL
		SELECT SUBSTRING(e.closed_at, 1, 7) AS month, 0 AS opened, 1 AS closed
		FROM event e
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.closed_at IS NOT NULL
		  AND e.org = COALESCE($5, e.org)
		  AND e.repo = COALESCE($6, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($7, COALESCE(d.entity, ''))
		  AND e.closed_at >= $8
		  ` + botExcludeSQL + `
	) sub
	GROUP BY month
	ORDER BY month
`
```

Go method — identical to SQLite version but with 8 params:

```go
func (s *Store) GetIssueOpenCloseRatio(org, repo, entity *string, months int) (*data.IssueRatioSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectIssueOpenCloseRatioSQL, org, repo, entity, since, org, repo, entity, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query issue open/close ratio: %w", err)
	}
	defer rows.Close()

	sr := &data.IssueRatioSeries{
		Months: make([]string, 0),
		Opened: make([]int, 0),
		Closed: make([]int, 0),
	}

	for rows.Next() {
		var month string
		var opened, closed int
		if err := rows.Scan(&month, &opened, &closed); err != nil {
			return nil, fmt.Errorf("failed to scan issue ratio row: %w", err)
		}
		sr.Months = append(sr.Months, month)
		sr.Opened = append(sr.Opened, opened)
		sr.Closed = append(sr.Closed, closed)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}
```

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: PASS — both implementations satisfy the interface.

**Step 3: Commit**

```bash
git add pkg/data/postgres/insights.go
git commit -S -m "feat: implement issue open/close ratio query for PostgreSQL"
```

---

## Task 4: Issue Open/Close Ratio — HTTP Handler and Route

**Files:**
- Modify: `pkg/cli/data.go` (add handler function)
- Modify: `pkg/cli/server.go` (add route, near line 199)

**Step 1: Add handler to data.go**

```go
func insightsIssueRatioAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetIssueOpenCloseRatio(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get issue open/close ratio", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying issue open/close ratio")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Register route in server.go**

Add after the existing insights routes (around line 199):

```go
mux.HandleFunc("GET /data/insights/issue-ratio", insightsIssueRatioAPIHandler(store))
```

**Step 3: Verify compilation**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/cli/data.go pkg/cli/server.go
git commit -S -m "feat: add issue open/close ratio API endpoint"
```

---

## Task 5: Issue Open/Close Ratio — Dashboard UI

**Files:**
- Modify: `pkg/cli/templates/home.html` (add chart canvas to Activity tab)
- Modify: `pkg/cli/assets/js/app.js` (add chart loader + wire into loadTabCharts)

**Step 1: Add chart canvas to home.html**

In the Activity tab section (after the Forks & Activity article, around line 208, before `</section>`), add:

```html
<article>
    <div class="tbl">
        <div class="content-header">
            Issue Open/Close Ratio
        </div>
        <div class="tbl-chart tbl-home">
            <canvas class="chart" id="issue-ratio-chart"></canvas>
        </div>
        <span class="insight-desc">Monthly opened vs closed issues. Growing gap between opened and closed may indicate backlog pressure.</span>
    </div>
</article>
```

**Step 2: Add JS chart loader function**

Add in `app.js` near the other chart loaders (after `loadForksAndActivityChart`):

```javascript
var issueRatioChart;
function loadIssueRatioChart(url) {
    $.get(url, function (data) {
        if (issueRatioChart) issueRatioChart.destroy();
        issueRatioChart = new Chart($("#issue-ratio-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Opened',
                    data: data.opened,
                    backgroundColor: colors[3],
                    borderWidth: 1,
                    order: 2
                }, {
                    label: 'Closed',
                    data: data.closed,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    order: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { stacked: true, ticks: { font: { size: 14 } } },
                    y: { stacked: true, beginAtZero: true, ticks: { font: { size: 14 } },
                        title: { display: true, text: 'Issues' } }
                }
            }
        });
    });
}
```

**Step 3: Wire into loadTabCharts**

In the `case 'activity':` block (around line 360), add:

```javascript
loadIssueRatioChart('/data/insights/issue-ratio?' + q);
```

**Step 4: Run the dev server and verify visually**

Run: `make server`
Navigate to the Activity tab and confirm the stacked bar chart renders.

**Step 5: Commit**

```bash
git add pkg/cli/templates/home.html pkg/cli/assets/js/app.js
git commit -S -m "feat: add issue open/close ratio chart to Activity tab"
```

---

## Task 6: Issue Comment Number Parsing — Import Fix

**Files:**
- Modify: `pkg/data/sqlite/event.go` (add `parseIssueNumberFromURL`, update `importIssueCommentEvents`)
- Modify: `pkg/data/postgres/event.go` (same changes)
- Modify: `pkg/data/sqlite/event_test.go` (add parser test)

Issue comment HTML URLs look like: `https://github.com/org/repo/issues/123#issuecomment-456`
The issue number is in the path segment after `/issues/`.

**Step 1: Write the failing test for the URL parser**

Add to `event_test.go`:

```go
func TestParseIssueNumberFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{"valid", "https://github.com/org/repo/issues/123#issuecomment-456", 123},
		{"no_fragment", "https://github.com/org/repo/issues/42", 42},
		{"pull_url", "https://github.com/org/repo/pull/99#issuecomment-789", 0},
		{"empty", "", 0},
		{"no_issues_segment", "https://github.com/org/repo/discussions/5", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseIssueNumberFromURL(tt.url))
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/data/sqlite/ -run TestParseIssueNumberFromURL -v`
Expected: FAIL — function not defined.

**Step 3: Add the parser function to sqlite/event.go**

Add near `parsePRNumberFromURL` (around line 449):

```go
func parseIssueNumberFromURL(url string) int {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		if p == "issues" && i+1 < len(parts) {
			// Strip fragment: "123#issuecomment-456" → "123"
			numStr := strings.SplitN(parts[i+1], "#", 2)[0]
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/data/sqlite/ -run TestParseIssueNumberFromURL -v`
Expected: PASS

**Step 5: Update importIssueCommentEvents in sqlite/event.go**

Change the loop body (line 753-756) from:

```go
if err := e.add(data.EventTypeIssueComment, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), ghutil.ParseUsers(items[i].Body), nil, nil); err != nil {
```

To:

```go
extra := &eventExtra{
    CreatedAt: timestampStr(items[i].CreatedAt),
}
if items[i].HTMLURL != nil {
    if n := parseIssueNumberFromURL(*items[i].HTMLURL); n > 0 {
        extra.Number = &n
    }
}
if err := e.add(data.EventTypeIssueComment, *items[i].HTMLURL, items[i].User, timestampToTime(items[i].UpdatedAt), ghutil.ParseUsers(items[i].Body), nil, extra); err != nil {
```

**Step 6: Copy the same parser + import changes to postgres/event.go**

Add `parseIssueNumberFromURL` near line 453 (after `parsePRNumberFromURL`). Update `importIssueCommentEvents` the same way.

**Step 7: Verify compilation**

Run: `go build ./...`
Expected: PASS

**Step 8: Commit**

```bash
git add pkg/data/sqlite/event.go pkg/data/sqlite/event_test.go pkg/data/postgres/event.go
git commit -S -m "feat: parse issue number from comment URLs during import"
```

---

## Task 7: Issue Comment Number Backfill — Migration

**Files:**
- Create: `pkg/data/sqlite/sql/migrations/015_backfill_issue_comment_number.sql`
- Create: `pkg/data/postgres/sql/migrations/015_backfill_issue_comment_number.sql`

**Step 1: Create SQLite migration**

SQLite does not support regex natively, but we can use `INSTR` and `SUBSTR`:

```sql
-- Backfill issue number from URL for existing issue_comment events.
-- URL pattern: https://github.com/org/repo/issues/123#issuecomment-456
-- Extract the number between /issues/ and # (or end of URL).
UPDATE event
SET number = CAST(
    SUBSTR(
        url,
        INSTR(url, '/issues/') + 8,
        CASE
            WHEN INSTR(SUBSTR(url, INSTR(url, '/issues/') + 8), '#') > 0
            THEN INSTR(SUBSTR(url, INSTR(url, '/issues/') + 8), '#') - 1
            ELSE LENGTH(url)
        END
    ) AS INTEGER
)
WHERE type = 'issue_comment'
  AND number IS NULL
  AND url LIKE '%/issues/%';
```

**Step 2: Create PostgreSQL migration**

```sql
-- Backfill issue number from URL for existing issue_comment events.
UPDATE event
SET number = CAST(
    SUBSTRING(
        url FROM '/issues/([0-9]+)'
    ) AS INTEGER
)
WHERE type = 'issue_comment'
  AND number IS NULL
  AND url LIKE '%/issues/%';
```

**Step 3: Run tests to verify migrations apply**

Run: `go test ./pkg/data/sqlite/ -run TestSetupTestDB -v`
Expected: PASS — migrations run without error.

**Step 4: Commit**

```bash
git add pkg/data/sqlite/sql/migrations/015_backfill_issue_comment_number.sql pkg/data/postgres/sql/migrations/015_backfill_issue_comment_number.sql
git commit -S -m "feat: backfill issue number on existing issue_comment events"
```

---

## Task 8: Time to First Response — Data Types and Store Interface

**Files:**
- Modify: `pkg/data/types.go` (add struct)
- Modify: `pkg/data/store.go` (add method to InsightsStore)

**Step 1: Add response struct to types.go**

Add after `IssueRatioSeries`:

```go
type FirstResponseSeries struct {
	Months   []string  `json:"months" yaml:"months"`
	IssueAvg []float64 `json:"issue_avg" yaml:"issueAvg"`
	PRAvg    []float64 `json:"pr_avg" yaml:"prAvg"`
}
```

**Step 2: Add Store method**

Add to `InsightsStore`:

```go
GetTimeToFirstResponse(org, repo, entity *string, months int) (*FirstResponseSeries, error)
```

**Step 3: Commit**

```bash
git add pkg/data/types.go pkg/data/store.go
git commit -S -m "feat: add FirstResponseSeries type and Store interface method"
```

---

## Task 9: Time to First Response — SQLite Implementation

**Files:**
- Modify: `pkg/data/sqlite/insights.go` (add SQL + method)
- Modify: `pkg/data/sqlite/insights_test.go` (add tests)

**Step 1: Write failing tests**

```go
func TestGetTimeToFirstResponse_NilDB(t *testing.T) {
	s := &Store{}
	_, err := s.GetTimeToFirstResponse(nil, nil, nil, 6)
	require.ErrorIs(t, err, data.ErrDBNotInitialized)
}

func TestGetTimeToFirstResponse_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	series, err := store.GetTimeToFirstResponse(nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Months)
}

func TestGetTimeToFirstResponse_WithData(t *testing.T) {
	store := setupTestDB(t)

	_, err := store.db.Exec(`INSERT INTO developer (username, full_name) VALUES ('alice', 'Alice'), ('bob', 'Bob')`)
	require.NoError(t, err)

	// Issue created Jan 10, first comment Jan 11 (24 hours later)
	_, err = store.db.Exec(`INSERT INTO event (org, repo, username, type, date, url, mentions, labels, state, number, created_at)
		VALUES
		('org1', 'repo1', 'alice', 'issue', '2025-01-10', 'http://i/1', '', '', 'open', 1, '2025-01-10T00:00:00Z'),
		('org1', 'repo1', 'bob', 'issue_comment', '2025-01-11', 'http://i/1#c1', '', '', NULL, 1, '2025-01-11T00:00:00Z'),
		('org1', 'repo1', 'alice', 'pr', '2025-01-10', 'http://p/2', '', '', 'open', 2, '2025-01-10T00:00:00Z'),
		('org1', 'repo1', 'bob', 'pr_review', '2025-01-10', 'http://p/2#r1', '', '', NULL, 2, '2025-01-10T12:00:00Z')`)
	require.NoError(t, err)

	series, err := store.GetTimeToFirstResponse(nil, nil, nil, 24)
	require.NoError(t, err)
	require.Len(t, series.Months, 1)
	assert.Equal(t, "2025-01", series.Months[0])
	assert.InDelta(t, 24.0, series.IssueAvg[0], 0.1)  // 24 hours
	assert.InDelta(t, 12.0, series.PRAvg[0], 0.1)      // 12 hours
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/data/sqlite/ -run TestGetTimeToFirstResponse -v`
Expected: FAIL

**Step 3: Add SQL and implementation**

SQL queries — two separate CTEs joined into one result:

```go
selectTimeToFirstResponseSQL = `WITH issue_first AS (
		SELECT
			e.org, e.repo, e.number,
			substr(e.created_at, 1, 7) AS month,
			MIN(
				(julianday(c.created_at) - julianday(e.created_at)) * 24
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'issue_comment' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	), pr_first AS (
		SELECT
			e.org, e.repo, e.number,
			substr(e.created_at, 1, 7) AS month,
			MIN(
				(julianday(c.created_at) - julianday(e.created_at)) * 24
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'pr_review' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	)
	SELECT
		COALESCE(i.month, p.month) AS month,
		COALESCE(AVG(i.hours_to_first), 0) AS issue_avg,
		COALESCE(AVG(p.hours_to_first), 0) AS pr_avg
	FROM issue_first i
	FULL OUTER JOIN pr_first p ON i.month = p.month
	GROUP BY COALESCE(i.month, p.month)
	ORDER BY month
`
```

Note: SQLite does not support `FULL OUTER JOIN`. Use `LEFT JOIN` + `UNION` instead:

```go
selectTimeToFirstResponseSQL = `WITH issue_first AS (
		SELECT
			e.org, e.repo, e.number,
			substr(e.created_at, 1, 7) AS month,
			MIN(
				(julianday(c.created_at) - julianday(e.created_at)) * 24
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'issue_comment' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	), pr_first AS (
		SELECT
			e.org, e.repo, e.number,
			substr(e.created_at, 1, 7) AS month,
			MIN(
				(julianday(c.created_at) - julianday(e.created_at)) * 24
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'pr_review' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE(?, e.org)
		  AND e.repo = COALESCE(?, e.repo)
		  AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
		  AND e.created_at >= ?
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	), all_months AS (
		SELECT month FROM issue_first
		UNION
		SELECT month FROM pr_first
	)
	SELECT
		m.month,
		COALESCE((SELECT AVG(hours_to_first) FROM issue_first WHERE month = m.month), 0),
		COALESCE((SELECT AVG(hours_to_first) FROM pr_first WHERE month = m.month), 0)
	FROM all_months m
	ORDER BY m.month
`
```

Go implementation:

```go
func (s *Store) GetTimeToFirstResponse(org, repo, entity *string, months int) (*data.FirstResponseSeries, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	since := sinceDate(months)

	rows, err := s.db.Query(selectTimeToFirstResponseSQL,
		org, repo, entity, since,
		org, repo, entity, since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query time to first response: %w", err)
	}
	defer rows.Close()

	sr := &data.FirstResponseSeries{
		Months:   make([]string, 0),
		IssueAvg: make([]float64, 0),
		PRAvg:    make([]float64, 0),
	}

	for rows.Next() {
		var month string
		var issueAvg, prAvg float64
		if err := rows.Scan(&month, &issueAvg, &prAvg); err != nil {
			return nil, fmt.Errorf("failed to scan first response row: %w", err)
		}
		sr.Months = append(sr.Months, month)
		sr.IssueAvg = append(sr.IssueAvg, issueAvg)
		sr.PRAvg = append(sr.PRAvg, prAvg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return sr, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/data/sqlite/ -run TestGetTimeToFirstResponse -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/data/sqlite/insights.go pkg/data/sqlite/insights_test.go
git commit -S -m "feat: implement time to first response query for SQLite"
```

---

## Task 10: Time to First Response — PostgreSQL Implementation

**Files:**
- Modify: `pkg/data/postgres/insights.go`

**Step 1: Add PostgreSQL SQL and method**

Same CTE approach with PostgreSQL syntax (`SUBSTRING`, `$N` params, `EXTRACT(EPOCH FROM ...)/3600.0`, supports `FULL OUTER JOIN`):

```go
selectTimeToFirstResponseSQL = `WITH issue_first AS (
		SELECT
			e.org, e.repo, e.number,
			SUBSTRING(e.created_at, 1, 7) AS month,
			MIN(
				EXTRACT(EPOCH FROM (c.created_at::timestamp - e.created_at::timestamp)) / 3600.0
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'issue_comment' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'issue'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE($1, e.org)
		  AND e.repo = COALESCE($2, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($3, COALESCE(d.entity, ''))
		  AND e.created_at >= $4
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	), pr_first AS (
		SELECT
			e.org, e.repo, e.number,
			SUBSTRING(e.created_at, 1, 7) AS month,
			MIN(
				EXTRACT(EPOCH FROM (c.created_at::timestamp - e.created_at::timestamp)) / 3600.0
			) AS hours_to_first
		FROM event e
		JOIN event c ON c.org = e.org AND c.repo = e.repo AND c.number = e.number
			AND c.type = 'pr_review' AND c.created_at > e.created_at
		JOIN developer d ON e.username = d.username
		WHERE e.type = 'pr'
		  AND e.created_at IS NOT NULL
		  AND e.number IS NOT NULL
		  AND e.org = COALESCE($5, e.org)
		  AND e.repo = COALESCE($6, e.repo)
		  AND COALESCE(d.entity, '') = COALESCE($7, COALESCE(d.entity, ''))
		  AND e.created_at >= $8
		  ` + botExcludeSQL + `
		GROUP BY e.org, e.repo, e.number, month
	)
	SELECT
		COALESCE(i.month, p.month) AS month,
		COALESCE(AVG(i.hours_to_first), 0) AS issue_avg,
		COALESCE(AVG(p.hours_to_first), 0) AS pr_avg
	FROM issue_first i
	FULL OUTER JOIN pr_first p ON i.month = p.month
	GROUP BY COALESCE(i.month, p.month)
	ORDER BY month
`
```

Go method — identical to SQLite version.

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/data/postgres/insights.go
git commit -S -m "feat: implement time to first response query for PostgreSQL"
```

---

## Task 11: Time to First Response — HTTP Handler, Route, and UI

**Files:**
- Modify: `pkg/cli/data.go` (add handler)
- Modify: `pkg/cli/server.go` (add route)
- Modify: `pkg/cli/templates/home.html` (add canvas to Velocity tab)
- Modify: `pkg/cli/assets/js/app.js` (add chart loader + wire in)

**Step 1: Add handler to data.go**

```go
func insightsTimeToFirstResponseAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetTimeToFirstResponse(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to first response", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to first response")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Register route in server.go**

```go
mux.HandleFunc("GET /data/insights/time-to-first-response", insightsTimeToFirstResponseAPIHandler(store))
```

**Step 3: Add canvas to home.html Velocity tab**

Insert as the first article in the Velocity tab (before Lead Time, around line 214):

```html
<article>
    <div class="tbl">
        <div class="content-header">
            Time to First Response
        </div>
        <div class="tbl-chart tbl-home">
            <canvas class="chart" id="time-to-first-response-chart"></canvas>
        </div>
        <span class="insight-desc">Average hours until first review (PRs) or comment (issues) from someone other than the author.</span>
    </div>
</article>
```

**Step 4: Add JS chart loader**

```javascript
var timeToFirstResponseChart;
function loadTimeToFirstResponseChart(url) {
    $.get(url, function (data) {
        if (timeToFirstResponseChart) timeToFirstResponseChart.destroy();
        timeToFirstResponseChart = new Chart($("#time-to-first-response-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Issues (avg hrs)',
                    data: data.issue_avg,
                    backgroundColor: colors[3],
                    borderWidth: 1,
                    order: 2
                }, {
                    label: 'PRs (avg hrs)',
                    data: data.pr_avg,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    order: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { font: { size: 14 } },
                        title: { display: true, text: 'Avg Hours' } }
                }
            }
        });
    });
}
```

**Step 5: Wire into loadTabCharts velocity case**

Add at the beginning of `case 'velocity':`:

```javascript
loadTimeToFirstResponseChart('/data/insights/time-to-first-response?' + q);
```

**Step 6: Run dev server and verify**

Run: `make server`
Check Velocity tab for new chart.

**Step 7: Commit**

```bash
git add pkg/cli/data.go pkg/cli/server.go pkg/cli/templates/home.html pkg/cli/assets/js/app.js
git commit -S -m "feat: add time to first response chart to Velocity tab"
```

---

## Task 12: Community Profile — Migration

**Files:**
- Create: `pkg/data/sqlite/sql/migrations/016_repo_meta_community_profile.sql`
- Create: `pkg/data/postgres/sql/migrations/016_repo_meta_community_profile.sql`

**Step 1: Create SQLite migration**

```sql
ALTER TABLE repo_meta ADD COLUMN has_coc INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN has_contributing INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN has_readme INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN has_security_policy INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN has_issue_template INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN has_pr_template INTEGER NOT NULL DEFAULT 0;
ALTER TABLE repo_meta ADD COLUMN community_health_pct INTEGER NOT NULL DEFAULT 0;
```

**Step 2: Create PostgreSQL migration (identical)**

Same SQL — both dialects support `ALTER TABLE ADD COLUMN`.

**Step 3: Verify migrations apply**

Run: `go test ./pkg/data/sqlite/ -run TestSetup -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/data/sqlite/sql/migrations/016_repo_meta_community_profile.sql pkg/data/postgres/sql/migrations/016_repo_meta_community_profile.sql
git commit -S -m "feat: add community profile columns to repo_meta table"
```

---

## Task 13: Community Profile — Data Types and Import

**Files:**
- Modify: `pkg/data/types.go` (extend RepoMeta struct)
- Modify: `pkg/data/sqlite/repo_meta.go` (update import function, SQL constants, scan)
- Modify: `pkg/data/postgres/repo_meta.go` (same changes)

**Step 1: Extend RepoMeta struct in types.go**

Add fields after `Archived`:

```go
type RepoMeta struct {
	Org               string `json:"org" yaml:"org"`
	Repo              string `json:"repo" yaml:"repo"`
	Stars             int    `json:"stars" yaml:"stars"`
	Forks             int    `json:"forks" yaml:"forks"`
	OpenIssues        int    `json:"open_issues" yaml:"openIssues"`
	Language          string `json:"language" yaml:"language"`
	License           string `json:"license" yaml:"license"`
	Archived          bool   `json:"archived" yaml:"archived"`
	UpdatedAt         string `json:"updated_at" yaml:"updatedAt"`
	HasCoC            bool   `json:"has_coc" yaml:"hasCoc"`
	HasContributing   bool   `json:"has_contributing" yaml:"hasContributing"`
	HasReadme         bool   `json:"has_readme" yaml:"hasReadme"`
	HasSecurityPolicy bool   `json:"has_security_policy" yaml:"hasSecurityPolicy"`
	HasIssueTemplate  bool   `json:"has_issue_template" yaml:"hasIssueTemplate"`
	HasPRTemplate     bool   `json:"has_pr_template" yaml:"hasPrTemplate"`
	CommunityHealthPct int   `json:"community_health_pct" yaml:"communityHealthPct"`
}
```

**Step 2: Update SQLite repo_meta.go**

Update `upsertRepoMetaSQL` to include new columns (both INSERT and ON CONFLICT).

Update `selectRepoMetaSQL` to SELECT the new columns.

Update `ImportRepoMeta` to call the GitHub community profile API after fetching repo metadata:

```go
// Fetch community profile
profile, profileResp, profileErr := client.Repositories.GetCommunityHealthMetrics(ctx, owner, repo)
if profileErr != nil {
    slog.Warn("failed to get community profile", "org", owner, "repo", repo, "error", profileErr)
}
if profileResp != nil {
    if rlErr := ghutil.CheckRateLimit(ctx, profileResp); rlErr != nil {
        return rlErr
    }
}

hasCoc, hasContrib, hasReadme, hasSecurity, hasIssueTmpl, hasPRTmpl, healthPct := 0, 0, 0, 0, 0, 0, 0
if profile != nil {
    if profile.Files != nil {
        if profile.Files.CodeOfConduct != nil { hasCoc = 1 }
        if profile.Files.Contributing != nil { hasContrib = 1 }
        if profile.Files.Readme != nil { hasReadme = 1 }
        if profile.Files.SecurityMdFile != nil { hasSecurity = 1 }
        if profile.Files.IssueTemplate != nil { hasIssueTmpl = 1 }
        if profile.Files.PullRequestTemplate != nil { hasPRTmpl = 1 }
    }
    if profile.HealthPercentage != nil { healthPct = *profile.HealthPercentage }
}
```

Then pass these values to the updated upsert SQL.

Update `GetRepoMetas` scan to include new fields (scanning into int vars, converting to bool like `Archived`).

**Step 3: Apply same changes to postgres/repo_meta.go**

Same logic, different SQL placeholder style.

**Step 4: Verify compilation and tests**

Run: `go build ./... && go test ./pkg/data/sqlite/ -run TestGetRepoMetas -v -race`
Expected: PASS (new columns default to 0, existing tests unaffected)

**Step 5: Commit**

```bash
git add pkg/data/types.go pkg/data/sqlite/repo_meta.go pkg/data/postgres/repo_meta.go
git commit -S -m "feat: import community profile data into repo_meta"
```

---

## Task 14: Community Profile — Dashboard UI

**Files:**
- Modify: `pkg/cli/assets/js/app.js` (extend `loadRepoMeta` function)

**Step 1: Update loadRepoMeta in app.js**

After the existing `items` array construction (around line 1244), add community profile badges:

```javascript
// Community profile badges
var communityFiles = [
    { key: 'has_readme', label: 'README' },
    { key: 'has_contributing', label: 'Contributing' },
    { key: 'has_coc', label: 'Code of Conduct' },
    { key: 'has_security_policy', label: 'Security Policy' },
    { key: 'has_issue_template', label: 'Issue Template' },
    { key: 'has_pr_template', label: 'PR Template' }
];
var present = 0, total = communityFiles.length;
$.each(data, function (i, m) {
    $.each(communityFiles, function (j, f) {
        if (m[f.key]) present++;
    });
});
// Only show if any profile data has been imported (health_pct > 0 for at least one repo)
var hasProfile = data.some(function(m) { return m.community_health_pct > 0; });
if (hasProfile) {
    var avgHealth = Math.round(data.reduce(function(s, m) { return s + m.community_health_pct; }, 0) / data.length);
    items.push({ label: 'Community Health', val: avgHealth + '%' });

    var badgeHtml = '<div class="community-badges">';
    $.each(communityFiles, function (j, f) {
        var count = data.filter(function(m) { return m[f.key]; }).length;
        var icon = count === data.length ? '&#10003;' : (count > 0 ? '~' : '&#10007;');
        var cls = count === data.length ? 'badge-ok' : (count > 0 ? 'badge-partial' : 'badge-missing');
        badgeHtml += '<span class="community-badge ' + cls + '">' + icon + ' ' + f.label + '</span>';
    });
    badgeHtml += '</div>';
    container.after(badgeHtml);
}
```

**Step 2: Add minimal CSS for badges**

Add to the existing CSS file (or inline in header.html) — find the appropriate stylesheet:

```css
.community-badges { display: flex; flex-wrap: wrap; gap: 0.4rem; padding: 0.5rem 1rem; }
.community-badge { font-size: 0.8rem; padding: 0.15rem 0.5rem; border-radius: 4px; }
.badge-ok { color: var(--fg); background: var(--accent-green, #2ea04370); }
.badge-partial { color: var(--fg); background: var(--accent-yellow, #d29a0070); }
.badge-missing { color: var(--fg-muted); background: var(--bg-muted, #88888830); }
```

**Step 3: Run dev server and verify**

Run: `make server`
Check Health tab Repo Metadata card for community profile badges.

**Step 4: Commit**

```bash
git add pkg/cli/assets/js/app.js pkg/cli/assets/css/style.css
git commit -S -m "feat: display community profile badges in repo metadata card"
```

---

## Task 15: Full Qualification

**Step 1: Run full qualification**

Run: `make qualify`
Expected: All tests pass, no lint errors, no vulnerabilities.

**Step 2: Fix any issues found**

Address any lint, test, or vulnerability findings.

**Step 3: Commit fixes if needed**

```bash
git add -A
git commit -S -m "fix: address qualify findings"
```

---

## Unresolved Questions

1. **Self-comments:** Should time-to-first-response exclude comments by the issue/PR author? The current SQL joins on `c.created_at > e.created_at` but doesn't filter `c.username != e.username`. May want to add that filter to avoid counting self-replies.
2. **Community profile API rate limit:** The `/community/profile` endpoint counts against the same rate limit. With many repos, this adds one API call per repo per sync cycle. Should we skip this call when metadata is fresh (24h cache already in place)?
3. **CSS file location:** Need to verify exact path of the main stylesheet to add badge CSS. Might be in `header.html` inline or a separate CSS file under `assets/css/`.
