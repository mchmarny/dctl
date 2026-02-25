# Downloads by Release Tag — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a dashboard panel showing download counts per release tag (last 9 recent + all-time top, deduplicated).

**Architecture:** New SQL query using UNION of recent-9 and top-1 CTEs, new Go query function, HTTP handler, and horizontal bar chart on the dashboard. No schema changes — data already exists in `release_asset` + `release` tables.

**Tech Stack:** Go, SQLite, Chart.js, jQuery

---

### Task 1: Write failing tests for GetReleaseDownloadsByTag

**Files:**
- Modify: `pkg/data/release_test.go`

**Step 1: Add three test functions**

Append to `pkg/data/release_test.go`:

```go
func TestGetReleaseDownloadsByTag_NilDB(t *testing.T) {
	_, err := GetReleaseDownloadsByTag(nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetReleaseDownloadsByTag_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	series, err := GetReleaseDownloadsByTag(db, nil, nil, 6)
	require.NoError(t, err)
	assert.Empty(t, series.Tags)
}

func TestGetReleaseDownloadsByTag_WithData(t *testing.T) {
	db := setupTestDB(t)

	// Insert 11 releases spanning recent months
	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('org1', 'repo1', 'v0.1.0', 'R0.1', '2025-01-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.2.0', 'R0.2', '2025-02-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.3.0', 'R0.3', '2025-03-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.4.0', 'R0.4', '2025-04-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.5.0', 'R0.5', '2025-05-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.6.0', 'R0.6', '2025-06-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.7.0', 'R0.7', '2025-07-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.8.0', 'R0.8', '2025-08-01T00:00:00Z', 0),
		('org1', 'repo1', 'v0.9.0', 'R0.9', '2025-09-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.0.0', 'R1.0', '2025-10-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0', 'R1.1', '2025-11-01T00:00:00Z', 0)`)
	require.NoError(t, err)

	// v0.1.0 is the all-time top by downloads (5000), but NOT in the last 9 by date
	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('org1', 'repo1', 'v0.1.0', 'app.tar.gz', 'application/gzip', 100, 5000),
		('org1', 'repo1', 'v0.2.0', 'app.tar.gz', 'application/gzip', 100, 10),
		('org1', 'repo1', 'v0.3.0', 'app.tar.gz', 'application/gzip', 100, 20),
		('org1', 'repo1', 'v0.4.0', 'app.tar.gz', 'application/gzip', 100, 30),
		('org1', 'repo1', 'v0.5.0', 'app.tar.gz', 'application/gzip', 100, 40),
		('org1', 'repo1', 'v0.6.0', 'app.tar.gz', 'application/gzip', 100, 50),
		('org1', 'repo1', 'v0.7.0', 'app.tar.gz', 'application/gzip', 100, 60),
		('org1', 'repo1', 'v0.8.0', 'app.tar.gz', 'application/gzip', 100, 70),
		('org1', 'repo1', 'v0.9.0', 'app.tar.gz', 'application/gzip', 100, 80),
		('org1', 'repo1', 'v1.0.0', 'app.tar.gz', 'application/gzip', 100, 90),
		('org1', 'repo1', 'v1.1.0', 'app.tar.gz', 'application/gzip', 100, 100)`)
	require.NoError(t, err)

	series, err := GetReleaseDownloadsByTag(db, nil, nil, 24)
	require.NoError(t, err)

	// 9 recent (v0.3.0..v1.1.0) + top (v0.1.0) = 10
	require.Len(t, series.Tags, 10)

	// First entry should be v0.1.0 (oldest by published_at, pulled in as top)
	assert.Equal(t, "v0.1.0", series.Tags[0])
	assert.Equal(t, 5000, series.Downloads[0])

	// Last entry should be v1.1.0 (most recent)
	assert.Equal(t, "v1.1.0", series.Tags[9])
	assert.Equal(t, 100, series.Downloads[9])
}

func TestGetReleaseDownloadsByTag_Dedup(t *testing.T) {
	db := setupTestDB(t)

	// Only 3 releases — the top downloaded IS in the recent 9
	_, err := db.Exec(`INSERT INTO release (org, repo, tag, name, published_at, prerelease) VALUES
		('org1', 'repo1', 'v1.0.0', 'R1', '2025-10-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.1.0', 'R2', '2025-11-01T00:00:00Z', 0),
		('org1', 'repo1', 'v1.2.0', 'R3', '2025-12-01T00:00:00Z', 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO release_asset (org, repo, tag, name, content_type, size, download_count) VALUES
		('org1', 'repo1', 'v1.0.0', 'app.tar.gz', 'application/gzip', 100, 10),
		('org1', 'repo1', 'v1.1.0', 'app.tar.gz', 'application/gzip', 100, 500),
		('org1', 'repo1', 'v1.2.0', 'app.tar.gz', 'application/gzip', 100, 20)`)
	require.NoError(t, err)

	series, err := GetReleaseDownloadsByTag(db, nil, nil, 24)
	require.NoError(t, err)

	// Only 3 entries — no duplicate for v1.1.0
	require.Len(t, series.Tags, 3)
}
```

**Step 2: Run tests to verify they fail**

Run: `make test`
Expected: Compilation error — `GetReleaseDownloadsByTag` undefined.

**Step 3: Commit**

```
git add pkg/data/release_test.go
git commit -S -m "test: add failing tests for GetReleaseDownloadsByTag"
```

---

### Task 2: Implement GetReleaseDownloadsByTag

**Files:**
- Modify: `pkg/data/release.go`

**Step 1: Add SQL constant**

In `pkg/data/release.go`, add to the `const` block after `selectReleaseDownloadsSQL`:

```go
	selectReleaseDownloadsByTagSQL = `WITH recent AS (
			SELECT r.org, r.repo, r.tag, r.published_at
			FROM release r
			WHERE r.org = COALESCE(?, r.org)
			  AND r.repo = COALESCE(?, r.repo)
			  AND r.published_at >= ?
			ORDER BY r.published_at DESC
			LIMIT 9
		), top AS (
			SELECT ra.org, ra.repo, ra.tag, r.published_at
			FROM release_asset ra
			JOIN release r ON ra.org = r.org AND ra.repo = r.repo AND ra.tag = r.tag
			WHERE ra.org = COALESCE(?, ra.org)
			  AND ra.repo = COALESCE(?, ra.repo)
			  AND r.published_at >= ?
			GROUP BY ra.org, ra.repo, ra.tag
			ORDER BY SUM(ra.download_count) DESC
			LIMIT 1
		), combined AS (
			SELECT org, repo, tag, published_at FROM recent
			UNION
			SELECT org, repo, tag, published_at FROM top
		)
		SELECT c.tag, COALESCE(SUM(ra.download_count), 0) AS downloads
		FROM combined c
		LEFT JOIN release_asset ra ON c.org = ra.org AND c.repo = ra.repo AND c.tag = ra.tag
		GROUP BY c.tag, c.published_at
		ORDER BY c.published_at
	`
```

**Step 2: Add the type**

After `ReleaseDownloadsSeries`, add:

```go
type ReleaseDownloadsByTagSeries struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Downloads []int    `json:"downloads" yaml:"downloads"`
}
```

**Step 3: Add the query function**

After `GetReleaseDownloads`, add:

```go
func GetReleaseDownloadsByTag(db *sql.DB, org, repo *string, months int) (*ReleaseDownloadsByTagSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	since := time.Now().UTC().AddDate(0, -months, 0).Format("2006-01-02")

	rows, err := db.Query(selectReleaseDownloadsByTagSQL, org, repo, since, org, repo, since)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query release downloads by tag: %w", err)
	}
	defer rows.Close()

	s := &ReleaseDownloadsByTagSeries{
		Tags:      make([]string, 0),
		Downloads: make([]int, 0),
	}

	for rows.Next() {
		var tag string
		var downloads int
		if err := rows.Scan(&tag, &downloads); err != nil {
			return nil, fmt.Errorf("failed to scan release downloads by tag row: %w", err)
		}
		s.Tags = append(s.Tags, tag)
		s.Downloads = append(s.Downloads, downloads)
	}

	return s, nil
}
```

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass including the 4 new ones.

**Step 5: Commit**

```
git add pkg/data/release.go
git commit -S -m "feat: add GetReleaseDownloadsByTag query"
```

---

### Task 3: Add API handler and route

**Files:**
- Modify: `pkg/cli/data.go`
- Modify: `pkg/cli/server.go`

**Step 1: Add handler in `pkg/cli/data.go`**

After `insightsReleaseDownloadsAPIHandler`, add:

```go
func insightsReleaseDownloadsByTagAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := data.GetReleaseDownloadsByTag(db, p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get release downloads by tag", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying release downloads by tag")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Register route in `pkg/cli/server.go`**

After the `release-downloads` route (line 131), add:

```go
	mux.HandleFunc("GET /data/insights/release-downloads-by-tag", insightsReleaseDownloadsByTagAPIHandler(db))
```

**Step 3: Run qualify**

Run: `make qualify`
Expected: Pass (lint + test + vuln scan).

**Step 4: Commit**

```
git add pkg/cli/data.go pkg/cli/server.go
git commit -S -m "feat: add release downloads by tag API endpoint"
```

---

### Task 4: Add dashboard panel and chart

**Files:**
- Modify: `pkg/cli/templates/home.html`
- Modify: `pkg/cli/assets/js/app.js`

**Step 1: Add HTML panel in `pkg/cli/templates/home.html`**

After the "Release Downloads" `</article>` (line 199), add:

```html
        <article>
            <div class="tbl">
                <div class="content-header">
                    Downloads by Release
                </div>
                <div class="tbl-chart tbl-home">
                    <canvas class="chart" id="release-downloads-by-tag-chart"></canvas>
                </div>
                <span class="insight-desc">Download counts per release tag. Last 9 releases plus the all-time most downloaded.</span>
            </div>
        </article>
```

**Step 2: Add chart variable in `pkg/cli/assets/js/app.js`**

After `let releaseDownloadsChart;` (line 111), add:

```javascript
let releaseDownloadsByTagChart;
```

**Step 3: Add destroy call in `destroyCharts()`**

After the `releaseDownloadsChart` destroy block (line 396-398), add:

```javascript
    if (releaseDownloadsByTagChart) {
        releaseDownloadsByTagChart.destroy();
    }
```

**Step 4: Add load call in `loadAllInsightCharts()`**

After the `loadReleaseDownloadsChart` call (line 204), add:

```javascript
    loadReleaseDownloadsByTagChart(`/data/insights/release-downloads-by-tag?m=${months}&o=${org}&r=${repo}`);
```

**Step 5: Add chart function**

After `loadReleaseDownloadsChart` function (after line 992), add:

```javascript
function loadReleaseDownloadsByTagChart(url) {
    $.get(url, function (data) {
        releaseDownloadsByTagChart = new Chart($("#release-downloads-by-tag-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.tags,
                datasets: [{
                    label: 'Downloads',
                    data: data.downloads,
                    backgroundColor: colors[0] + '80',
                    borderColor: colors[0],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                indexAxis: 'y',
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: {
                        beginAtZero: true,
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Downloads' }
                    },
                    y: { ticks: { font: { size: 14 } } }
                }
            }
        });
    });
}
```

**Step 6: Run qualify**

Run: `make qualify`
Expected: Pass.

**Step 7: Commit**

```
git add pkg/cli/templates/home.html pkg/cli/assets/js/app.js
git commit -S -m "feat: add release downloads by tag dashboard panel"
```

---

### Task 5: Manual verification

**Step 1: Start dev server**

Run: `make server`

**Step 2: Verify in browser**

- Check that "Downloads by Release" panel appears after "Release Downloads"
- Horizontal bar chart shows tags on Y-axis, downloads on X-axis
- Filter by org/repo works via query params
- Month selector affects the results

**Step 3: Run full qualify one more time**

Run: `make qualify`
Expected: All green.
