# Repo Metric History Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track daily star and fork counts per repo with 30-day backfill and three dashboard charts.

**Architecture:** New `repo_metric_history` table stores one row per repo per day. Backfill walks GitHub's ListStargazers and ListForks APIs backward from newest, stopping at 30 days. Forward recording appends today's snapshot during `ImportRepoMeta`. Dashboard gets a sparkline in the metadata panel plus two dedicated trend panels.

**Tech Stack:** Go, SQLite, go-github v83 (ListStargazers, ListForks), Chart.js line charts, jQuery AJAX.

---

### Task 1: Database Migration

**Files:**
- Create: `pkg/data/sql/migrations/009_repo_metric_history.sql`

**Step 1: Create migration file**

```sql
CREATE TABLE IF NOT EXISTS repo_metric_history (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    date TEXT NOT NULL,
    stars INTEGER NOT NULL DEFAULT 0,
    forks INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (org, repo, date)
);
```

**Step 2: Verify migration applies**

Run: `make test`
Expected: PASS (migration runs via `setupTestDB` which calls `runMigrations`)

**Step 3: Commit**

```bash
git add pkg/data/sql/migrations/009_repo_metric_history.sql
git commit -S -m "feat: add repo_metric_history migration"
```

---

### Task 2: Data Layer - Query Function

**Files:**
- Create: `pkg/data/metric_history.go`
- Create: `pkg/data/metric_history_test.go`

**Step 1: Write the failing tests**

```go
package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRepoMetricHistory_NilDB(t *testing.T) {
	_, err := GetRepoMetricHistory(nil, nil, nil)
	assert.Error(t, err)
}

func TestGetRepoMetricHistory_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	list, err := GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetRepoMetricHistory_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org1', 'repo1', '2026-03-11', 105, 52),
		('org1', 'repo1', '2026-03-12', 110, 55)`)
	require.NoError(t, err)

	list, err := GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, "2026-03-10", list[0].Date)
	assert.Equal(t, 110, list[2].Stars)
	assert.Equal(t, 55, list[2].Forks)
}

func TestGetRepoMetricHistory_WithFilter(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES
		('org1', 'repo1', '2026-03-10', 100, 50),
		('org2', 'repo2', '2026-03-10', 200, 80)`)
	require.NoError(t, err)

	org := "org1"
	list, err := GetRepoMetricHistory(db, &org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/data/ -run TestGetRepoMetricHistory -v`
Expected: FAIL (function not defined)

**Step 3: Write the query function**

```go
package data

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	upsertRepoMetricHistorySQL = `INSERT INTO repo_metric_history (org, repo, date, stars, forks)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(org, repo, date) DO UPDATE SET
			stars = ?, forks = ?
	`

	selectRepoMetricHistorySQL = `SELECT org, repo, date, stars, forks
		FROM repo_metric_history
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo, date
	`
)

type RepoMetricHistory struct {
	Org   string `json:"org"`
	Repo  string `json:"repo"`
	Date  string `json:"date"`
	Stars int    `json:"stars"`
	Forks int    `json:"forks"`
}

func GetRepoMetricHistory(db *sql.DB, org, repo *string) ([]*RepoMetricHistory, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectRepoMetricHistorySQL, org, repo)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query repo metric history: %w", err)
	}
	defer rows.Close()

	list := make([]*RepoMetricHistory, 0)
	for rows.Next() {
		m := &RepoMetricHistory{}
		if err := rows.Scan(&m.Org, &m.Repo, &m.Date, &m.Stars, &m.Forks); err != nil {
			return nil, fmt.Errorf("failed to scan repo metric history row: %w", err)
		}
		list = append(list, m)
	}

	return list, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/data/ -run TestGetRepoMetricHistory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/data/metric_history.go pkg/data/metric_history_test.go
git commit -S -m "feat: add GetRepoMetricHistory query function"
```

---

### Task 3: Data Layer - Backfill Import Function

**Files:**
- Modify: `pkg/data/metric_history.go` (add import functions)

**Step 1: Add the backfill function**

Add to `metric_history.go`:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/devpulse/pkg/net"
)

const backfillDays = 30

// ImportRepoMetricHistory backfills daily star and fork counts for the last 30 days.
func ImportRepoMetricHistory(dbPath, token, owner, repo string) error {
	ctx := context.Background()
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	// Get current totals from GitHub.
	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	checkRateLimit(resp)

	currentStars := r.GetStargazersCount()
	currentForks := r.GetForksCount()
	cutoff := time.Now().AddDate(0, 0, -backfillDays).UTC()

	// Count new stars per day in the last 30 days.
	starsByDay, err := countRecentStarsByDay(ctx, client, owner, repo, cutoff)
	if err != nil {
		return fmt.Errorf("error counting stars: %w", err)
	}

	// Count new forks per day in the last 30 days.
	forksByDay, err := countRecentForksByDay(ctx, client, owner, repo, cutoff)
	if err != nil {
		return fmt.Errorf("error counting forks: %w", err)
	}

	// Build daily totals working backward from today.
	history := buildDailyTotals(currentStars, currentForks, starsByDay, forksByDay, backfillDays)

	// Persist to DB.
	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	return upsertMetricHistory(db, owner, repo, history)
}

// countRecentStarsByDay pages through ListStargazers and counts stars per day.
// Stops once all entries on a page are older than cutoff.
func countRecentStarsByDay(ctx context.Context, client *github.Client, owner, repo string, cutoff time.Time) (map[string]int, error) {
	counts := make(map[string]int)
	opt := &github.ListOptions{PerPage: 100, Page: 1}

	// ListStargazers returns oldest first. We need to find total pages first,
	// then page backward from the last page.
	_, resp, err := client.Activity.ListStargazers(ctx, owner, repo, &github.ListOptions{PerPage: 100, Page: 1})
	if err != nil {
		return nil, fmt.Errorf("error listing stargazers: %w", err)
	}
	checkRateLimit(resp)

	lastPage := resp.LastPage
	if lastPage == 0 {
		// Only one page of results (or none).
		lastPage = 1
	}

	for page := lastPage; page >= 1; page-- {
		opt.Page = page
		stargazers, resp, err := client.Activity.ListStargazers(ctx, owner, repo, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing stargazers page %d: %w", page, err)
		}
		checkRateLimit(resp)

		if len(stargazers) == 0 {
			break
		}

		allOlder := true
		for _, sg := range stargazers {
			if sg.StarredAt == nil {
				continue
			}
			t := sg.StarredAt.Time
			if t.Before(cutoff) {
				continue
			}
			allOlder = false
			day := t.Format("2006-01-02")
			counts[day]++
		}

		if allOlder {
			break
		}
	}

	return counts, nil
}

// countRecentForksByDay pages through ListForks (newest first) and counts forks per day.
func countRecentForksByDay(ctx context.Context, client *github.Client, owner, repo string, cutoff time.Time) (map[string]int, error) {
	counts := make(map[string]int)
	opt := &github.RepositoryListForksOptions{
		Sort:        "newest",
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}

	for {
		forks, resp, err := client.Repositories.ListForks(ctx, owner, repo, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing forks: %w", err)
		}
		checkRateLimit(resp)

		if len(forks) == 0 {
			break
		}

		allOlder := true
		for _, f := range forks {
			t := f.GetCreatedAt().Time
			if t.Before(cutoff) {
				continue
			}
			allOlder = false
			day := t.Format("2006-01-02")
			counts[day]++
		}

		if allOlder {
			break
		}

		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return counts, nil
}

// buildDailyTotals reconstructs daily cumulative totals from per-day deltas.
func buildDailyTotals(currentStars, currentForks int, starsByDay, forksByDay map[string]int, days int) []*RepoMetricHistory {
	now := time.Now().UTC()
	dates := make([]string, days+1)
	for i := 0; i <= days; i++ {
		dates[days-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	// Work backward: today = current totals, each prior day subtracts that day's new count.
	result := make([]*RepoMetricHistory, len(dates))
	stars := currentStars
	forks := currentForks

	for i := len(dates) - 1; i >= 0; i-- {
		result[i] = &RepoMetricHistory{
			Date:  dates[i],
			Stars: stars,
			Forks: forks,
		}
		// Subtract today's new stars/forks to get yesterday's total.
		stars -= starsByDay[dates[i]]
		forks -= forksByDay[dates[i]]
		// Floor at zero in case of unstars/deleted forks.
		if stars < 0 {
			stars = 0
		}
		if forks < 0 {
			forks = 0
		}
	}

	return result
}

func upsertMetricHistory(db *sql.DB, owner, repo string, history []*RepoMetricHistory) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(upsertRepoMetricHistorySQL)
	if err != nil {
		rollbackTransaction(tx)
		return fmt.Errorf("failed to prepare metric history statement: %w", err)
	}

	for _, h := range history {
		if _, err := stmt.Exec(owner, repo, h.Date, h.Stars, h.Forks, h.Stars, h.Forks); err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("failed to upsert metric history %s: %w", h.Date, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit metric history: %w", err)
	}

	slog.Debug("metric history done", "org", owner, "repo", repo, "days", len(history))
	return nil
}

// ImportAllRepoMetricHistory backfills metric history for all tracked repos.
func ImportAllRepoMetricHistory(dbPath, token string) error {
	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := ImportRepoMetricHistory(dbPath, token, r.Org, r.Repo); err != nil {
			slog.Error("metric history failed", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}
```

**Step 2: Add unit test for buildDailyTotals**

Add to `metric_history_test.go`:

```go
func TestBuildDailyTotals(t *testing.T) {
	starsByDay := map[string]int{
		time.Now().UTC().Format("2006-01-02"):                    5,
		time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"): 3,
	}
	forksByDay := map[string]int{
		time.Now().UTC().Format("2006-01-02"): 2,
	}

	result := buildDailyTotals(100, 50, starsByDay, forksByDay, 3)

	require.Len(t, result, 4) // days+1
	// Last entry (today) should have current totals.
	assert.Equal(t, 100, result[3].Stars)
	assert.Equal(t, 50, result[3].Forks)
	// Yesterday: 100 - 5 = 95 stars, 50 - 2 = 48 forks.
	assert.Equal(t, 95, result[2].Stars)
	assert.Equal(t, 48, result[2].Forks)
	// Day before: 95 - 3 = 92 stars.
	assert.Equal(t, 92, result[1].Stars)
}

func TestUpsertMetricHistory(t *testing.T) {
	db := setupTestDB(t)

	history := []*RepoMetricHistory{
		{Date: "2026-03-10", Stars: 100, Forks: 50},
		{Date: "2026-03-11", Stars: 105, Forks: 52},
	}

	err := upsertMetricHistory(db, "org1", "repo1", history)
	require.NoError(t, err)

	list, err := GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 100, list[0].Stars)

	// Upsert same dates with new values.
	history[0].Stars = 101
	err = upsertMetricHistory(db, "org1", "repo1", history)
	require.NoError(t, err)

	list, err = GetRepoMetricHistory(db, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, 101, list[0].Stars)
}
```

**Step 3: Run tests**

Run: `go test ./pkg/data/ -run "TestBuildDailyTotals|TestUpsertMetricHistory|TestGetRepoMetricHistory" -v`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/data/metric_history.go pkg/data/metric_history_test.go
git commit -S -m "feat: add repo metric history backfill import"
```

---

### Task 4: Forward Recording in ImportRepoMeta

**Files:**
- Modify: `pkg/data/repo_meta.go:59-81` (add upsert after existing upsert)

**Step 1: Add forward recording**

After the existing `db.Exec(upsertRepoMetaSQL, ...)` block in `ImportRepoMeta` (after line 78), add:

```go
	// Record today's snapshot in metric history.
	today := time.Now().UTC().Format("2006-01-02")
	_, err = db.Exec(upsertRepoMetricHistorySQL,
		owner, repo, today, r.GetStargazersCount(), r.GetForksCount(),
		r.GetStargazersCount(), r.GetForksCount(),
	)
	if err != nil {
		return fmt.Errorf("error upserting repo metric history %s/%s: %w", owner, repo, err)
	}
```

Note: `upsertRepoMetricHistorySQL` is defined in `metric_history.go` and is accessible since both files are in package `data`.

**Step 2: Run existing tests to verify no regression**

Run: `make test`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/data/repo_meta.go
git commit -S -m "feat: record daily metric snapshot in ImportRepoMeta"
```

---

### Task 5: CLI Integration - Call Backfill During Import

**Files:**
- Modify: `pkg/cli/import.go:259-272` (add backfill call to `importRepoExtras`)

**Step 1: Add backfill to importRepoExtras**

Add a third loop after the releases loop in `importRepoExtras`:

```go
	for _, r := range repos {
		slog.Info("metric history", "org", org, "repo", r)
		if err := data.ImportRepoMetricHistory(dbPath, token, org, r); err != nil {
			slog.Error("failed to import metric history", "org", org, "repo", r, "error", err)
		}
	}
```

**Step 2: Run tests**

Run: `make test`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/cli/import.go
git commit -S -m "feat: add metric history backfill to import command"
```

---

### Task 6: API Endpoint

**Files:**
- Modify: `pkg/cli/server.go:~129` (register new route)
- Modify: `pkg/cli/data.go:~314` (add handler after insightsRepoMetaAPIHandler)

**Step 1: Add handler function to data.go**

After `insightsRepoMetaAPIHandler` (~line 314):

```go
func insightsRepoMetricHistoryAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := data.GetRepoMetricHistory(db, p.org, p.repo)
		if err != nil {
			slog.Error("failed to get repo metric history", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying repo metric history")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Register route in server.go**

After the `repo-meta` route registration (~line 129), add:

```go
	mux.HandleFunc("GET /data/insights/repo-metric-history", insightsRepoMetricHistoryAPIHandler(db))
```

**Step 3: Run tests**

Run: `make test`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/cli/server.go pkg/cli/data.go
git commit -S -m "feat: add repo metric history API endpoint"
```

---

### Task 7: Dashboard - Stars Trend Panel

**Files:**
- Modify: `pkg/cli/templates/home.html` (add canvas panels after forks-activity panel, ~line 221)
- Modify: `pkg/cli/assets/js/app.js` (add chart load functions, call from loadAllCharts)

**Step 1: Add HTML panels to home.html**

After the Forks & Activity panel (~line 221), add:

```html
			<article>
				<div class="tbl">
					<div class="content-header">Stars Trend</div>
					<div class="tbl-chart">
						<canvas id="stars-trend-chart"></canvas>
					</div>
					<span class="insight-desc">Daily star count over the last 30 days.</span>
				</div>
			</article>
			<article>
				<div class="tbl">
					<div class="content-header">Forks Trend</div>
					<div class="tbl-chart">
						<canvas id="forks-trend-chart"></canvas>
					</div>
					<span class="insight-desc">Daily fork count over the last 30 days.</span>
				</div>
			</article>
```

**Step 2: Add chart functions to app.js**

Add near the other chart load functions (after `loadRepoMeta`):

```javascript
var starsTrendChart;
var forksTrendChart;

function loadStarsTrendChart(url) {
    $.get(url, function(data) {
        if (!data || data.length === 0) return;
        var labels = [];
        var stars = [];
        $.each(data, function(i, d) {
            labels.push(d.date);
            stars.push(d.stars);
        });
        if (starsTrendChart) starsTrendChart.destroy();
        starsTrendChart = new Chart($("#stars-trend-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Stars',
                    data: stars,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { ticks: { maxTicksToShow: 10 } },
                    y: { beginAtZero: false, ticks: { precision: 0 } }
                }
            }
        });
    });
}

function loadForksTrendChart(url) {
    $.get(url, function(data) {
        if (!data || data.length === 0) return;
        var labels = [];
        var forks = [];
        $.each(data, function(i, d) {
            labels.push(d.date);
            forks.push(d.forks);
        });
        if (forksTrendChart) forksTrendChart.destroy();
        forksTrendChart = new Chart($("#forks-trend-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Forks',
                    data: forks,
                    borderColor: colors[1],
                    backgroundColor: colors[1] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { ticks: { maxTicksToShow: 10 } },
                    y: { beginAtZero: false, ticks: { precision: 0 } }
                }
            }
        });
    });
}
```

**Step 3: Call from loadAllCharts**

In `loadAllCharts` function, add after the `loadRepoMeta` call:

```javascript
    var metricHistoryUrl = "/data/insights/repo-metric-history?" + params;
    loadStarsTrendChart(metricHistoryUrl);
    loadForksTrendChart(metricHistoryUrl);
```

**Step 4: Run tests and lint**

Run: `make qualify`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/cli/templates/home.html pkg/cli/assets/js/app.js
git commit -S -m "feat: add stars and forks trend dashboard panels"
```

---

### Task 8: Dashboard - Sparkline in Metadata Panel

**Files:**
- Modify: `pkg/cli/assets/js/app.js` (update `loadRepoMeta` function)
- Modify: `pkg/cli/templates/home.html` (add sparkline canvas to repo-meta panel)

**Step 1: Add sparkline canvas to home.html**

In the repo metadata panel (the article containing `repo-meta-container`), add a canvas after the container div:

```html
				<canvas id="repo-meta-sparkline" height="60"></canvas>
```

**Step 2: Add sparkline rendering to loadRepoMeta**

At the end of the `loadRepoMeta` function, after the existing card rendering, add a fetch for metric history and render a small sparkline:

```javascript
    // Load sparkline for metadata panel.
    var sparkUrl = "/data/insights/repo-metric-history?" + buildParams();
    $.get(sparkUrl, function(hist) {
        if (!hist || hist.length === 0) return;
        var labels = [];
        var stars = [];
        var forks = [];
        $.each(hist, function(i, d) {
            labels.push(d.date);
            stars.push(d.stars);
            forks.push(d.forks);
        });
        new Chart($("#repo-meta-sparkline")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Stars',
                        data: stars,
                        borderColor: colors[0],
                        borderWidth: 1.5,
                        pointRadius: 0,
                        tension: 0.3,
                        fill: false
                    },
                    {
                        label: 'Forks',
                        data: forks,
                        borderColor: colors[1],
                        borderWidth: 1.5,
                        pointRadius: 0,
                        tension: 0.3,
                        fill: false
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true, position: 'bottom', labels: { boxWidth: 10, font: { size: 10 } } }
                },
                scales: {
                    x: { display: false },
                    y: { display: false }
                }
            }
        });
    });
```

Note: `buildParams()` must be extracted or the params must be passed. Check how other chart loaders receive their URL params in `loadAllCharts` — the sparkline fetch should use the same `metricHistoryUrl` variable. Wire this by passing the URL into `loadRepoMeta` or by fetching inside `loadAllCharts` after `loadRepoMeta` completes.

**Step 3: Run qualify**

Run: `make qualify`
Expected: PASS

**Step 4: Commit**

```bash
git add pkg/cli/templates/home.html pkg/cli/assets/js/app.js
git commit -S -m "feat: add sparkline to repo metadata panel"
```

---

### Task 9: Final Verification

**Step 1: Run full qualify**

Run: `make qualify`
Expected: All tests pass, lint clean, no vulnerabilities.

**Step 2: Manual smoke test**

Run: `make server`

Verify:
- Stars Trend panel renders a line chart
- Forks Trend panel renders a line chart
- Repo Metadata panel shows sparkline below the cards
- All three respond to org/repo filter changes

---

## Unresolved Questions

1. **Sparkline wiring**: The exact mechanism for passing the metric history URL into `loadRepoMeta` depends on how `buildParams()` or the URL variable is scoped in `loadAllCharts`. May need to read the full function to wire correctly.
2. **`colors` array**: The exact indices for the color palette need to match the existing theme. Verify `colors[0]` and `colors[1]` are appropriate for stars/forks.
3. **Aggregate across repos**: The dedicated trend panels may need to aggregate (sum) stars/forks across multiple repos when no repo filter is set. The current query returns per-repo rows — the JS may need to group and sum, or the SQL query could be extended with a grouped variant.
