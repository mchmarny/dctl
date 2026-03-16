# Contributor Profile Panel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a "Contributor Profile" panel to the Community tab that shows per-contributor metrics (PRs Opened, PRs Merged, PR Reviews, Issues Opened, Issue Comments, Forks) as grouped horizontal bars comparing the selected user vs. the average across all contributors.

**Architecture:** New SQL query counts events by type for a given user and computes averages across all contributors in the same scope. A search endpoint enables typeahead username lookup. The frontend pre-populates suggestions from the already-loaded `/data/developer` response and falls back to the search endpoint. Chart.js horizontal bar with two datasets (contributor vs. average).

**Tech Stack:** Go (data layer + HTTP handlers), SQLite, Chart.js, jQuery

---

### Task 1: Add SQL query and Go function for contributor profile

**Files:**
- Modify: `pkg/data/insights.go` (append SQL const + struct + function at end of file)

**Step 1: Add the SQL constant, struct, and function**

Append to the `const` block at the top of `insights.go` (after `selectContributorFunnelSQL`):

```go
// Contributor profile: per-user event counts and cross-contributor averages.
selectContributorProfileSQL = `WITH user_counts AS (
    SELECT
        SUM(CASE WHEN e.type = 'pr' THEN 1 ELSE 0 END) AS prs_opened,
        SUM(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN 1 ELSE 0 END) AS prs_merged,
        SUM(CASE WHEN e.type = 'pr_review' THEN 1 ELSE 0 END) AS pr_reviews,
        SUM(CASE WHEN e.type = 'issue' THEN 1 ELSE 0 END) AS issues_opened,
        SUM(CASE WHEN e.type = 'issue_comment' THEN 1 ELSE 0 END) AS issue_comments,
        SUM(CASE WHEN e.type = 'fork' THEN 1 ELSE 0 END) AS forks
    FROM event e
    JOIN developer d ON e.username = d.username
    WHERE e.username = ?
      AND e.org = COALESCE(?, e.org)
      AND e.repo = COALESCE(?, e.repo)
      AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
      AND e.date >= ?
),
avg_counts AS (
    SELECT
        AVG(prs_opened) AS prs_opened,
        AVG(prs_merged) AS prs_merged,
        AVG(pr_reviews) AS pr_reviews,
        AVG(issues_opened) AS issues_opened,
        AVG(issue_comments) AS issue_comments,
        AVG(forks) AS forks
    FROM (
        SELECT
            e.username,
            SUM(CASE WHEN e.type = 'pr' THEN 1 ELSE 0 END) AS prs_opened,
            SUM(CASE WHEN e.type = 'pr' AND e.state = 'merged' THEN 1 ELSE 0 END) AS prs_merged,
            SUM(CASE WHEN e.type = 'pr_review' THEN 1 ELSE 0 END) AS pr_reviews,
            SUM(CASE WHEN e.type = 'issue' THEN 1 ELSE 0 END) AS issues_opened,
            SUM(CASE WHEN e.type = 'issue_comment' THEN 1 ELSE 0 END) AS issue_comments,
            SUM(CASE WHEN e.type = 'fork' THEN 1 ELSE 0 END) AS forks
        FROM event e
        JOIN developer d ON e.username = d.username
        WHERE e.org = COALESCE(?, e.org)
          AND e.repo = COALESCE(?, e.repo)
          AND IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))
          AND e.date >= ?
          ` + botExcludeSQL + `
        GROUP BY e.username
    )
)
SELECT
    u.prs_opened, u.prs_merged, u.pr_reviews,
    u.issues_opened, u.issue_comments, u.forks,
    COALESCE(a.prs_opened, 0), COALESCE(a.prs_merged, 0), COALESCE(a.pr_reviews, 0),
    COALESCE(a.issues_opened, 0), COALESCE(a.issue_comments, 0), COALESCE(a.forks, 0)
FROM user_counts u, avg_counts a
`
```

Append after the `ContributorFunnelSeries` struct (after line ~781):

```go
type ContributorProfileSeries struct {
	Metrics  []string  `json:"metrics"`
	Values   []int     `json:"values"`
	Averages []float64 `json:"averages"`
}
```

Append after `GetContributorFunnel` function (after line ~857):

```go
func GetContributorProfile(db *sql.DB, username string, org, repo, entity *string, months int) (*ContributorProfileSeries, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	since := sinceDate(months)

	var prs, prsMerged, reviews, issues, comments, forks int
	var avgPrs, avgMerged, avgReviews, avgIssues, avgComments, avgForks float64

	err := db.QueryRow(selectContributorProfileSQL,
		username, org, repo, entity, since,
		org, repo, entity, since,
	).Scan(
		&prs, &prsMerged, &reviews, &issues, &comments, &forks,
		&avgPrs, &avgMerged, &avgReviews, &avgIssues, &avgComments, &avgForks,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query contributor profile: %w", err)
	}

	return &ContributorProfileSeries{
		Metrics:  []string{"PRs Opened", "PRs Merged", "PR Reviews", "Issues Opened", "Issue Comments", "Forks"},
		Values:   []int{prs, prsMerged, reviews, issues, comments, forks},
		Averages: []float64{avgPrs, avgMerged, avgReviews, avgIssues, avgComments, avgForks},
	}, nil
}
```

**Step 2: Run tests to verify no compilation errors**

Run: `go build ./pkg/data/...`
Expected: success (no output)

**Step 3: Commit**

```
feat: add contributor profile SQL query and Go function
```

---

### Task 2: Add developer username search function

**Files:**
- Modify: `pkg/data/org.go` (add SQL const + function)

**Step 1: Add the SQL constant and search function**

Add to the `const` block in `org.go` (after `selectAllOrgRepos`, line 65):

```go
selectDeveloperSearch = `SELECT DISTINCT d.username
    FROM developer d
    JOIN event e ON d.username = e.username
    WHERE d.username LIKE ?
      AND e.org = COALESCE(?, e.org)
      AND e.repo = COALESCE(?, e.repo)
      AND d.username NOT LIKE '%[bot]'
      AND e.date >= ?
    ORDER BY d.username
    LIMIT ?
`
```

Add after `GetEntityPercentages` (after line 174):

```go
func SearchDevelopers(db *sql.DB, query string, org, repo *string, months, limit int) ([]string, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	since := sinceDate(months)
	pattern := fmt.Sprintf("%%%s%%", query)

	rows, err := db.Query(selectDeveloperSearch, pattern, org, repo, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search developers: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("failed to scan developer row: %w", err)
		}
		list = append(list, username)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return list, nil
}
```

**Step 2: Run build to verify**

Run: `go build ./pkg/data/...`
Expected: success

**Step 3: Commit**

```
feat: add developer username search function
```

---

### Task 3: Add unit tests for contributor profile and developer search

**Files:**
- Modify: `pkg/data/insights_test.go` (append tests)
- Modify: `pkg/data/org_test.go` (append tests)

**Step 1: Add contributor profile tests to `insights_test.go`**

Append at end of file (before the `padDay` helper):

```go
func TestGetContributorProfile_NilDB(t *testing.T) {
	_, err := GetContributorProfile(nil, "alice", nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorProfile_EmptyUsername(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetContributorProfile(db, "", nil, nil, nil, 6)
	assert.Error(t, err)
}

func TestGetContributorProfile_EmptyDB(t *testing.T) {
	db := setupTestDB(t)

	// Insert one developer so the query has a valid username
	_, err := db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES ('alice', 'Alice', 'ACME')`)
	require.NoError(t, err)

	series, err := GetContributorProfile(db, "alice", nil, nil, nil, 6)
	require.NoError(t, err)
	assert.Len(t, series.Metrics, 6)
	assert.Len(t, series.Values, 6)
	assert.Len(t, series.Averages, 6)
	// All zeros since no events
	for _, v := range series.Values {
		assert.Equal(t, 0, v)
	}
}

func TestGetContributorProfile_WithData(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`INSERT INTO developer (username, full_name, entity) VALUES
		('alice', 'Alice', 'ACME'),
		('bob', 'Bob', 'ACME')`)
	require.NoError(t, err)

	// alice: 3 PRs, 1 merged PR, 2 issues
	// bob: 1 PR, 1 issue_comment
	for i := 0; i < 3; i++ {
		_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels, state)
			VALUES ('org1', 'repo1', 'alice', 'pr', ?, 'http://a', '', '', 'open')`,
			"2026-01-"+padDay(i))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels, state)
		VALUES ('org1', 'repo1', 'alice', 'pr', '2026-01-20', 'http://a2', '', '', 'merged')`)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
			VALUES ('org1', 'repo1', 'alice', 'issue', ?, 'http://a3', '', '')`,
			"2026-02-"+padDay(i))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'pr', '2026-01-10', 'http://b', '', '')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO event (org, repo, username, type, date, url, mentions, labels)
		VALUES ('org1', 'repo1', 'bob', 'issue_comment', '2026-01-11', 'http://b2', '', '')`)
	require.NoError(t, err)

	series, err := GetContributorProfile(db, "alice", nil, nil, nil, 24)
	require.NoError(t, err)
	assert.Len(t, series.Metrics, 6)

	// alice's PRs opened: 3 open + 1 merged = 4
	assert.Equal(t, 4, series.Values[0])
	// alice's PRs merged: 1
	assert.Equal(t, 1, series.Values[1])
	// alice's issues opened: 2
	assert.Equal(t, 3, series.Values[3])
	// Averages should be > 0
	assert.Greater(t, series.Averages[0], float64(0))
}
```

**Step 2: Add developer search tests to `org_test.go`**

Append at end of file:

```go
func TestSearchDevelopers_NilDB(t *testing.T) {
	_, err := SearchDevelopers(nil, "dev", nil, nil, 6, 10)
	assert.Error(t, err)
}

func TestSearchDevelopers_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := SearchDevelopers(db, "", nil, nil, 6, 10)
	assert.Error(t, err)
}

func TestSearchDevelopers_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	results, err := SearchDevelopers(db, "dev", nil, nil, 6, 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchDevelopers_WithData(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)
	results, err := SearchDevelopers(db, "dev", nil, nil, 12, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	for _, r := range results {
		assert.Contains(t, r, "dev")
	}
}
```

**Step 3: Run tests**

Run: `make test`
Expected: all pass

**Step 4: Commit**

```
test: add contributor profile and developer search tests
```

---

### Task 4: Add HTTP handlers and routes

**Files:**
- Modify: `pkg/cli/data.go` (add two handler functions)
- Modify: `pkg/cli/server.go` (register two routes)

**Step 1: Add handler functions to `data.go`**

Append after `insightsContributorFunnelAPIHandler` (after line 531):

```go
func insightsContributorProfileAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		username := r.URL.Query().Get("u")
		if username == "" {
			writeError(w, http.StatusBadRequest, "username parameter (u) is required")
			return
		}
		res, err := data.GetContributorProfile(db, username, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor profile", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor profile")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func developerSearchAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		q := r.URL.Query().Get("q")
		if q == "" {
			writeError(w, http.StatusBadRequest, "query parameter (q) is required")
			return
		}
		res, err := data.SearchDevelopers(db, q, p.org, p.repo, p.months, 10)
		if err != nil {
			slog.Error("failed to search developers", "error", err)
			writeError(w, http.StatusInternalServerError, "error searching developers")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Register routes in `server.go`**

Add after line 138 (after the `contributor-funnel` route):

```go
mux.HandleFunc("GET /data/insights/contributor-profile", insightsContributorProfileAPIHandler(db))
mux.HandleFunc("GET /data/developer/search", developerSearchAPIHandler(db))
```

**Step 3: Run build to verify**

Run: `go build ./pkg/cli/...`
Expected: success

**Step 4: Commit**

```
feat: add contributor profile and developer search API endpoints
```

---

### Task 5: Update HTML template

**Files:**
- Modify: `pkg/cli/templates/home.html` (lines 355-367, replace full-width article)

**Step 1: Replace the Top Collaborators full-width article**

Replace lines 355-367 (the `<article class="grid-full-width">` block) with two equal-width articles:

```html
            <article>
                <div class="tbl">
                    <div class="tbl-header">
                         <div class="content-header">
                            Top Colaborators
                        </div>
                    </div>
                    <div class="tbl-content tbl-home">
                        <canvas class="chart" id="right-chart"></canvas>
                    </div>
                    <span class="insight-desc">Ranked by total event count (PRs, issues, reviews, comments) in the selected period. Click a slice to view GitHub profile.</span>
                </div>
            </article>
            <article>
                <div class="tbl">
                    <div class="content-header">
                        Contributor Profile
                    </div>
                    <div class="contributor-search-wrap">
                        <input type="text" id="contributor-search" class="contributor-search" placeholder="Search username..." autocomplete="off" />
                        <ul id="contributor-suggestions" class="contributor-suggestions"></ul>
                    </div>
                    <div class="tbl-chart tbl-home">
                        <canvas class="chart" id="contributor-profile-chart"></canvas>
                    </div>
                    <span class="insight-desc">Per-contributor metrics vs. average across all contributors in the selected period.</span>
                </div>
            </article>
```

**Step 2: Verify template renders**

Run: `go build ./...`
Expected: success (templates are embedded at build time)

**Step 3: Commit**

```
feat: add contributor profile panel HTML to community tab
```

---

### Task 6: Add CSS for contributor search

**Files:**
- Modify: `pkg/cli/assets/css/app.css` (append styles)

**Step 1: Add styles**

Append to end of `app.css`:

```css
/* Contributor search typeahead */
.contributor-search-wrap {
    position: relative;
    padding: 0.5rem 1rem;
}

.contributor-search {
    width: 100%;
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
    color: var(--fg);
    font-size: 0.85rem;
    box-sizing: border-box;
}

.contributor-search:focus {
    outline: none;
    border-color: var(--accent);
}

.contributor-suggestions {
    display: none;
    position: absolute;
    left: 1rem;
    right: 1rem;
    top: 100%;
    background: var(--bg);
    border: 1px solid var(--border);
    border-top: none;
    border-radius: 0 0 4px 4px;
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 200px;
    overflow-y: auto;
    z-index: 100;
}

.contributor-suggestions.visible {
    display: block;
}

.contributor-suggestions li {
    padding: 0.4rem 0.6rem;
    cursor: pointer;
    font-size: 0.85rem;
}

.contributor-suggestions li:hover,
.contributor-suggestions li.active {
    background: var(--accent);
    color: var(--bg);
}
```

**Step 2: Commit**

```
feat: add contributor search typeahead CSS
```

---

### Task 7: Add JavaScript for typeahead and chart rendering

**Files:**
- Modify: `pkg/cli/assets/js/app.js`

**Step 1: Add chart variable declaration**

Near the top of `app.js`, where other chart variables are declared (e.g., `var leftChart, rightChart`), add:

```javascript
var contributorProfileChart;
```

**Step 2: Add the `loadContributorProfileChart` function**

Add after the `loadContributorFunnelChart` function:

```javascript
function loadContributorProfileChart(url) {
    $.get(url, function (data) {
        if (contributorProfileChart) contributorProfileChart.destroy();
        if (!data || !data.metrics) return;

        var barColor = colors[0];
        var avgColor = barColor.replace('rgb', 'rgba').replace(')', ', 0.35)');
        if (barColor.startsWith('#')) {
            var r = parseInt(barColor.slice(1,3), 16);
            var g = parseInt(barColor.slice(3,5), 16);
            var b = parseInt(barColor.slice(5,7), 16);
            avgColor = 'rgba(' + r + ',' + g + ',' + b + ',0.35)';
            barColor = 'rgba(' + r + ',' + g + ',' + b + ',1)';
        }

        contributorProfileChart = new Chart($("#contributor-profile-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.metrics,
                datasets: [
                    {
                        label: 'Contributor',
                        data: data.values,
                        backgroundColor: barColor,
                        borderRadius: 3
                    },
                    {
                        label: 'Average',
                        data: data.averages.map(function(v) { return Math.round(v * 10) / 10; }),
                        backgroundColor: avgColor,
                        borderRadius: 3
                    }
                ]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'top' }
                },
                scales: {
                    x: { beginAtZero: true, grid: { display: false } },
                    y: { grid: { display: false } }
                }
            }
        });
    });
}
```

**Step 3: Add typeahead wiring**

Add after the `loadContributorProfileChart` function:

```javascript
function initContributorSearch(q) {
    var $input = $("#contributor-search");
    var $suggestions = $("#contributor-suggestions");
    var knownUsers = [];
    var debounceTimer;

    // Pre-populate from already-loaded developer data (right chart)
    $.get('/data/developer?' + q, function(data) {
        if (data && data.labels) {
            knownUsers = data.labels.filter(function(l) { return l !== 'ALL OTHERS'; });
        }
    });

    function showSuggestions(list) {
        $suggestions.empty();
        if (!list.length) {
            $suggestions.removeClass('visible');
            return;
        }
        list.forEach(function(name) {
            $suggestions.append($('<li>').text(name));
        });
        $suggestions.addClass('visible');
    }

    function selectUser(username) {
        $input.val(username);
        $suggestions.removeClass('visible');
        loadContributorProfileChart('/data/insights/contributor-profile?u=' + encodeURIComponent(username) + '&' + q);
    }

    $input.on('input', function() {
        var val = $input.val().trim().toLowerCase();
        if (val.length < 2) {
            $suggestions.removeClass('visible');
            return;
        }

        // Filter known users first
        var local = knownUsers.filter(function(u) {
            return u.toLowerCase().indexOf(val) !== -1;
        });

        if (local.length > 0) {
            showSuggestions(local.slice(0, 10));
        }

        // Also search server for non-top users
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(function() {
            $.get('/data/developer/search?q=' + encodeURIComponent(val) + '&' + q, function(results) {
                if (!results || !results.length) {
                    if (!local.length) $suggestions.removeClass('visible');
                    return;
                }
                // Merge with local, deduplicate
                var merged = local.slice();
                results.forEach(function(u) {
                    if (merged.indexOf(u) === -1) merged.push(u);
                });
                showSuggestions(merged.slice(0, 10));
            });
        }, 250);
    });

    $suggestions.on('click', 'li', function() {
        selectUser($(this).text());
    });

    // Keyboard navigation
    $input.on('keydown', function(e) {
        var $items = $suggestions.find('li');
        var $active = $items.filter('.active');
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (!$active.length) { $items.first().addClass('active'); }
            else { $active.removeClass('active').next().addClass('active'); }
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            if ($active.length) { $active.removeClass('active').prev().addClass('active'); }
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if ($active.length) { selectUser($active.text()); }
        } else if (e.key === 'Escape') {
            $suggestions.removeClass('visible');
        }
    });

    // Close suggestions on outside click
    $(document).on('click', function(e) {
        if (!$(e.target).closest('.contributor-search-wrap').length) {
            $suggestions.removeClass('visible');
        }
    });
}
```

**Step 4: Wire into the Community tab activation**

In the `case 'community':` block (around line 274), add at the end (after the right-chart IIFE, around line 293):

```javascript
            initContributorSearch(q);
```

**Step 5: Commit**

```
feat: add contributor profile chart and typeahead search JS
```

---

### Task 8: Run full qualification and fix issues

**Step 1: Run qualify**

Run: `make qualify`
Expected: all tests pass, lint clean, no vulnerabilities

**Step 2: Fix any issues found**

Address lint warnings, test failures, or build errors.

**Step 3: Final commit**

```
feat: contributor profile panel on community tab
```

---

### Summary of all changes

| File | Type | Description |
|------|------|-------------|
| `pkg/data/insights.go` | Modify | SQL const, `ContributorProfileSeries` struct, `GetContributorProfile()` |
| `pkg/data/org.go` | Modify | SQL const, `SearchDevelopers()` |
| `pkg/data/insights_test.go` | Modify | 4 tests for contributor profile |
| `pkg/data/org_test.go` | Modify | 4 tests for developer search |
| `pkg/cli/data.go` | Modify | `insightsContributorProfileAPIHandler`, `developerSearchAPIHandler` |
| `pkg/cli/server.go` | Modify | 2 new route registrations |
| `pkg/cli/templates/home.html` | Modify | Split full-width row, add profile panel HTML |
| `pkg/cli/assets/css/app.css` | Modify | Typeahead dropdown styles |
| `pkg/cli/assets/js/app.js` | Modify | `loadContributorProfileChart()`, `initContributorSearch()`, tab wiring |
