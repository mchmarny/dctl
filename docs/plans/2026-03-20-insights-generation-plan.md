# Automated Insights Generation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an LLM-powered insights generation step to the sync pipeline that produces structured observations/actions per repo, stored in the DB, served on a new "Insights" dashboard tab.

**Architecture:** After the scoring phase in the sync job, check if insights are stale. If stale and `ANTHROPIC_API_KEY` is set, gather repo metrics via Store methods, call Claude Messages API via raw HTTP POST, parse the structured JSON response, and upsert into a `repo_insights` table. A new dashboard tab renders the stored insights.

**Tech Stack:** Go, SQLite/PostgreSQL, Claude Messages API (raw HTTP), Chart.js dashboard

---

### Task 1: Migration and Data Types

**Files:**
- Create: `pkg/data/sqlite/sql/migrations/017_generated_insights.sql`
- Create: `pkg/data/postgres/sql/migrations/017_generated_insights.sql`
- Modify: `pkg/data/types.go` (add structs after line 487)
- Modify: `pkg/data/store.go` (add interface after line 137, add to Store composite)

**Step 1: Create SQLite migration**

```sql
CREATE TABLE repo_insights (
    org TEXT NOT NULL,
    repo TEXT NOT NULL,
    insights_json TEXT NOT NULL,
    period_months INTEGER NOT NULL DEFAULT 3,
    model TEXT NOT NULL DEFAULT '',
    generated_at TEXT NOT NULL,
    PRIMARY KEY (org, repo)
);
```

**Step 2: Create identical PostgreSQL migration**

Same SQL.

**Step 3: Add types to pkg/data/types.go**

Add after `RepoOverview`:

```go
type InsightBullet struct {
	Headline string `json:"headline" yaml:"headline"`
	Detail   string `json:"detail" yaml:"detail"`
}

type GeneratedInsights struct {
	Observations []InsightBullet `json:"observations" yaml:"observations"`
	Actions      []InsightBullet `json:"actions" yaml:"actions"`
}

type RepoInsights struct {
	Org           string             `json:"org" yaml:"org"`
	Repo          string             `json:"repo" yaml:"repo"`
	Insights      *GeneratedInsights `json:"insights" yaml:"insights"`
	PeriodMonths  int                `json:"period_months" yaml:"periodMonths"`
	Model         string             `json:"model" yaml:"model"`
	GeneratedAt   string             `json:"generated_at" yaml:"generatedAt"`
}
```

**Step 4: Add interface to pkg/data/store.go**

Add after ReputationStore (line 137):

```go
type InsightsGenerationStore interface {
	GetRepoInsights(org, repo *string) ([]*RepoInsights, error)
	SaveRepoInsights(org, repo string, insights *RepoInsights) error
	GetRepoInsightsGeneratedAt(org, repo string) (string, error)
}
```

Add `InsightsGenerationStore` to the Store composite interface.

**Step 5: Verify compilation fails (expected — implementations missing)**

Run: `go build ./...`
Expected: FAIL

**Step 6: Commit**

```bash
git commit -S -m "feat: add repo_insights schema, types, and Store interface"
```

---

### Task 2: SQLite Store Implementation

**Files:**
- Create: `pkg/data/sqlite/repo_insights.go`
- Create: `pkg/data/sqlite/repo_insights_test.go`

**Step 1: Write tests**

```go
func TestGetRepoInsights_NilDB(t *testing.T) {
	s := &Store{}
	_, err := s.GetRepoInsights(nil, nil)
	require.ErrorIs(t, err, data.ErrDBNotInitialized)
}

func TestGetRepoInsights_EmptyDB(t *testing.T) {
	store := setupTestDB(t)
	list, err := store.GetRepoInsights(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSaveAndGetRepoInsights(t *testing.T) {
	store := setupTestDB(t)
	ri := &data.RepoInsights{
		Insights: &data.GeneratedInsights{
			Observations: []data.InsightBullet{{Headline: "Test", Detail: "detail"}},
			Actions:      []data.InsightBullet{{Headline: "Act", Detail: "do it"}},
		},
		PeriodMonths: 3,
		Model:        "claude-haiku-4-5-20251001",
		GeneratedAt:  "2025-01-15T00:00:00Z",
	}
	err := store.SaveRepoInsights("org1", "repo1", ri)
	require.NoError(t, err)

	list, err := store.GetRepoInsights(strPtr("org1"), strPtr("repo1"))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "org1", list[0].Org)
	assert.Equal(t, "Test", list[0].Insights.Observations[0].Headline)
	assert.Equal(t, "claude-haiku-4-5-20251001", list[0].Model)
}

func TestGetRepoInsightsGeneratedAt(t *testing.T) {
	store := setupTestDB(t)
	// No insights yet
	ts, err := store.GetRepoInsightsGeneratedAt("org1", "repo1")
	require.NoError(t, err)
	assert.Empty(t, ts)

	// Save insights
	ri := &data.RepoInsights{
		Insights:     &data.GeneratedInsights{},
		PeriodMonths: 3,
		Model:        "test",
		GeneratedAt:  "2025-01-15T00:00:00Z",
	}
	require.NoError(t, store.SaveRepoInsights("org1", "repo1", ri))

	ts, err = store.GetRepoInsightsGeneratedAt("org1", "repo1")
	require.NoError(t, err)
	assert.Equal(t, "2025-01-15T00:00:00Z", ts)
}

func strPtr(s string) *string { return &s }
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/data/sqlite/ -run TestGetRepoInsights -v`

**Step 3: Implement repo_insights.go**

```go
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	upsertRepoInsightsSQL = `INSERT INTO repo_insights (org, repo, insights_json, period_months, model, generated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo) DO UPDATE SET
			insights_json = ?, period_months = ?, model = ?, generated_at = ?
	`

	selectRepoInsightsSQL = `SELECT org, repo, insights_json, period_months, model, generated_at
		FROM repo_insights
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo
	`

	selectRepoInsightsGeneratedAtSQL = `SELECT COALESCE(generated_at, '')
		FROM repo_insights
		WHERE org = ? AND repo = ?
	`
)

func (s *Store) SaveRepoInsights(org, repo string, ri *data.RepoInsights) error {
	if s.db == nil {
		return data.ErrDBNotInitialized
	}

	b, err := json.Marshal(ri.Insights)
	if err != nil {
		return fmt.Errorf("marshaling insights: %w", err)
	}
	j := string(b)

	_, err = s.db.Exec(upsertRepoInsightsSQL,
		org, repo, j, ri.PeriodMonths, ri.Model, ri.GeneratedAt,
		j, ri.PeriodMonths, ri.Model, ri.GeneratedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting repo insights %s/%s: %w", org, repo, err)
	}
	return nil
}

func (s *Store) GetRepoInsights(org, repo *string) ([]*data.RepoInsights, error) {
	if s.db == nil {
		return nil, data.ErrDBNotInitialized
	}

	rows, err := s.db.Query(selectRepoInsightsSQL, org, repo)
	if err != nil {
		return nil, fmt.Errorf("querying repo insights: %w", err)
	}
	defer rows.Close()

	var list []*data.RepoInsights
	for rows.Next() {
		ri := &data.RepoInsights{}
		var j string
		if err := rows.Scan(&ri.Org, &ri.Repo, &j, &ri.PeriodMonths, &ri.Model, &ri.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scanning repo insights row: %w", err)
		}
		ri.Insights = &data.GeneratedInsights{}
		if err := json.Unmarshal([]byte(j), ri.Insights); err != nil {
			return nil, fmt.Errorf("unmarshaling insights JSON: %w", err)
		}
		list = append(list, ri)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}
	return list, nil
}

func (s *Store) GetRepoInsightsGeneratedAt(org, repo string) (string, error) {
	if s.db == nil {
		return "", data.ErrDBNotInitialized
	}
	var ts string
	err := s.db.QueryRow(selectRepoInsightsGeneratedAtSQL, org, repo).Scan(&ts)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying insights generated_at: %w", err)
	}
	return ts, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/data/sqlite/ -run TestGetRepoInsights -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git commit -S -m "feat: implement repo insights Store for SQLite"
```

---

### Task 3: PostgreSQL Store Implementation

**Files:**
- Create: `pkg/data/postgres/repo_insights.go`

Same as SQLite but with `$1-$N` placeholders:
- upsert: `$1-$10`
- select: `$1, $2`
- generated_at: `$1, $2`

**Step 1: Implement and verify build**

Run: `go build ./...`
Expected: PASS

**Step 2: Commit**

```bash
git commit -S -m "feat: implement repo insights Store for PostgreSQL"
```

---

### Task 4: Claude API Client (Metrics Assembly + LLM Call)

**Files:**
- Create: `pkg/data/insights_gen.go`
- Create: `pkg/data/insights_gen_test.go`

This is the core new file — gathers metrics, builds the prompt, calls the Claude API, parses the response.

**Step 1: Write test for metrics assembly**

```go
func TestBuildInsightsPrompt(t *testing.T) {
	metrics := &InsightsMetrics{
		Summary: &InsightsSummary{BusFactor: 3, PonyFactor: 1, Contributors: 19},
		// ... minimal fields
	}
	prompt := buildInsightsPrompt(metrics, 3)
	assert.Contains(t, prompt, "bus_factor")
	assert.Contains(t, prompt, "3 months")
}
```

**Step 2: Write test for response parsing**

```go
func TestParseInsightsResponse(t *testing.T) {
	raw := `{"observations":[{"headline":"Test","detail":"detail"}],"actions":[{"headline":"Act","detail":"do it"}]}`
	result, err := parseInsightsResponse(raw)
	require.NoError(t, err)
	require.Len(t, result.Observations, 1)
	assert.Equal(t, "Test", result.Observations[0].Headline)
}
```

**Step 3: Implement insights_gen.go**

Key components:

```go
package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	claudeAPIURL       = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion   = "2023-06-01"
	defaultModel       = "claude-haiku-4-5-20251001"
	insightsMaxTokens  = 4096
)

// InsightsMetrics holds all data needed for the prompt.
type InsightsMetrics struct {
	Summary          *InsightsSummary        `json:"summary"`
	Momentum         *MomentumSeries         `json:"momentum"`
	PRRatio          *PRReviewRatioSeries    `json:"pr_ratio"`
	TimeToMerge      *VelocitySeries         `json:"time_to_merge"`
	Retention        *RetentionSeries        `json:"retention"`
	Funnel           *ContributorFunnelSeries `json:"funnel"`
	ChangeFailure    *ChangeFailureRateSeries `json:"change_failure"`
	ReviewLatency    *ReviewLatencySeries     `json:"review_latency"`
	PRSize           *PRSizeSeries           `json:"pr_size"`
	ForksAndActivity *ForksAndActivitySeries  `json:"forks_and_activity"`
	RepoMeta         []*RepoMeta             `json:"repo_meta"`
	IssueRatio       *IssueRatioSeries       `json:"issue_ratio"`
	FirstResponse    *FirstResponseSeries     `json:"first_response"`
	ReleaseCadence   *ReleaseCadenceSeries    `json:"release_cadence"`
	ReleaseDownloads *ReleaseDownloadsSeries  `json:"release_downloads"`
}

func buildInsightsPrompt(metrics *InsightsMetrics, months int) string {
	data, _ := json.Marshal(metrics)
	return fmt.Sprintf(`You are analyzing project health data for the last %d months.

Here is the structured metrics data:
%s

DORA Benchmarks:
- Change failure rate: Elite <5%%, High 5-10%%, Medium 10-15%%, Low >15%%
- Time to merge: Elite <1 day, High 1-7 days
- Review latency: Elite <4h, High 4-24h

Produce exactly 5 Key Observations and 3 Action Items.
Each observation must cite specific numbers and cross-reference multiple metrics.
Each action must connect to a specific observation.

Respond with ONLY valid JSON in this exact format:
{"observations":[{"headline":"...","detail":"..."}],"actions":[{"headline":"...","detail":"..."}]}`, months, string(data))
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func GenerateInsights(ctx context.Context, apiKey, model string, metrics *InsightsMetrics, months int) (*GeneratedInsights, string, error) {
	if model == "" {
		model = defaultModel
	}

	prompt := buildInsightsPrompt(metrics, months)

	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: insightsMaxTokens,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, model, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(b))
	if err != nil {
		return nil, model, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, model, fmt.Errorf("calling Claude API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, model, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, model, fmt.Errorf("Claude API error %d: %s", resp.StatusCode, string(body))
	}

	var cr claudeResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, model, fmt.Errorf("parsing response: %w", err)
	}

	if len(cr.Content) == 0 {
		return nil, model, fmt.Errorf("empty response from Claude API")
	}

	insights, err := parseInsightsResponse(cr.Content[0].Text)
	if err != nil {
		return nil, model, fmt.Errorf("parsing insights: %w", err)
	}

	return insights, model, nil
}

func parseInsightsResponse(text string) (*GeneratedInsights, error) {
	var gi GeneratedInsights
	if err := json.Unmarshal([]byte(text), &gi); err != nil {
		return nil, fmt.Errorf("invalid JSON in response: %w", err)
	}
	return &gi, nil
}

// GatherInsightsMetrics collects all metrics from the Store for a given repo.
func GatherInsightsMetrics(store Store, org, repo string, months int) (*InsightsMetrics, error) {
	o, r := &org, &repo
	m := &InsightsMetrics{}
	var err error

	if m.Summary, err = store.GetInsightsSummary(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get summary", "error", err)
	}
	if m.Momentum, err = store.GetContributorMomentum(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get momentum", "error", err)
	}
	if m.PRRatio, err = store.GetPRReviewRatio(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get PR ratio", "error", err)
	}
	if m.TimeToMerge, err = store.GetTimeToMerge(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get time to merge", "error", err)
	}
	if m.Retention, err = store.GetContributorRetention(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get retention", "error", err)
	}
	if m.Funnel, err = store.GetContributorFunnel(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get funnel", "error", err)
	}
	if m.ChangeFailure, err = store.GetChangeFailureRate(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get change failure rate", "error", err)
	}
	if m.ReviewLatency, err = store.GetReviewLatency(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get review latency", "error", err)
	}
	if m.PRSize, err = store.GetPRSizeDistribution(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get PR size", "error", err)
	}
	if m.ForksAndActivity, err = store.GetForksAndActivity(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get forks and activity", "error", err)
	}
	if m.RepoMeta, err = store.GetRepoMetas(o, r); err != nil {
		slog.Warn("insights: failed to get repo meta", "error", err)
	}
	if m.IssueRatio, err = store.GetIssueOpenCloseRatio(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get issue ratio", "error", err)
	}
	if m.FirstResponse, err = store.GetTimeToFirstResponse(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get first response", "error", err)
	}
	if m.ReleaseCadence, err = store.GetReleaseCadence(o, r, nil, months); err != nil {
		slog.Warn("insights: failed to get release cadence", "error", err)
	}
	if m.ReleaseDownloads, err = store.GetReleaseDownloads(o, r, months); err != nil {
		slog.Warn("insights: failed to get release downloads", "error", err)
	}

	return m, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/data/ -run TestBuildInsightsPrompt -v && go test ./pkg/data/ -run TestParseInsightsResponse -v`
Expected: PASS

**Step 5: Commit**

```bash
git commit -S -m "feat: add Claude API client for insights generation"
```

---

### Task 5: Sync Pipeline Integration

**Files:**
- Modify: `pkg/cli/sync.go` (add flags + insights step after scoring, line 224)

**Step 1: Add CLI flags**

Add near other sync flags (after `syncStaleFlag`):

```go
syncInsightsStaleFlag = &urfave.StringFlag{
	Name:    "insights-stale",
	Usage:   "Duration before insights are regenerated (e.g. 7d, 1w)",
	Value:   "7d",
	Sources: urfave.EnvVars("DEVPULSE_INSIGHTS_STALE"),
}

syncInsightsPeriodFlag = &urfave.IntFlag{
	Name:    "insights-period",
	Usage:   "Number of months of data to analyze for insights",
	Value:   3,
	Sources: urfave.EnvVars("DEVPULSE_INSIGHTS_PERIOD"),
}
```

Add these flags to the `syncCmd.Flags` slice.

**Step 2: Add insights generation step after scoring (line 224)**

```go
// Insights (LLM-generated, requires ANTHROPIC_API_KEY)
phaseStart = time.Now()
insightsGenerated := false
if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
	insightsStaleHours, isErr := parseDurationHours(cmd.String(syncInsightsStaleFlag.Name))
	if isErr != nil {
		slog.Error("invalid --insights-stale value", "error", isErr)
	} else {
		genAt, _ := cfg.Store.GetRepoInsightsGeneratedAt(target.Org, target.Repo)
		stale := true
		if genAt != "" {
			if t, pErr := time.Parse("2006-01-02T15:04:05Z", genAt); pErr == nil {
				stale = time.Since(t) > time.Duration(insightsStaleHours)*time.Hour
			}
		}
		if stale {
			period := int(cmd.Int(syncInsightsPeriodFlag.Name))
			slog.Info("generating insights", "org", target.Org, "repo", target.Repo, "period_months", period)
			metrics, gatherErr := data.GatherInsightsMetrics(cfg.Store, target.Org, target.Repo, period)
			if gatherErr != nil {
				slog.Error("failed to gather metrics", "error", gatherErr)
			} else {
				model := os.Getenv("ANTHROPIC_MODEL")
				gi, usedModel, genErr := data.GenerateInsights(ctx, apiKey, model, metrics, period)
				if genErr != nil {
					errors++
					slog.Error("insights generation failed", "error", genErr)
				} else {
					now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
					ri := &data.RepoInsights{
						Insights:     gi,
						PeriodMonths: period,
						Model:        usedModel,
						GeneratedAt:  now,
					}
					if saveErr := cfg.Store.SaveRepoInsights(target.Org, target.Repo, ri); saveErr != nil {
						errors++
						slog.Error("failed to save insights", "error", saveErr)
					} else {
						insightsGenerated = true
					}
				}
			}
		} else {
			slog.Debug("insights fresh, skipping", "org", target.Org, "repo", target.Repo, "generated_at", genAt)
		}
	}
} else {
	slog.Debug("ANTHROPIC_API_KEY not set, skipping insights generation")
}
insightsSec := time.Since(phaseStart).Seconds()
```

Add `"insights_sec", insightsSec, "insights_generated", insightsGenerated` to the `sync_summary` log.

**Step 3: Add `"os"` to imports if not already present**

**Step 4: Verify build**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git commit -S -m "feat: integrate insights generation into sync pipeline"
```

---

### Task 6: HTTP Handler and Route

**Files:**
- Modify: `pkg/cli/data.go` (add handler)
- Modify: `pkg/cli/server.go` (add route)

**Step 1: Add handler to data.go**

```go
func insightsGeneratedAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetRepoInsights(p.org, p.repo)
		if err != nil {
			slog.Error("failed to get generated insights", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying generated insights")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
```

**Step 2: Add route to server.go**

After the last insights route:

```go
mux.HandleFunc("GET /data/insights/generated", insightsGeneratedAPIHandler(store))
```

**Step 3: Verify build**

Run: `go build ./...`

**Step 4: Commit**

```bash
git commit -S -m "feat: add generated insights API endpoint"
```

---

### Task 7: Dashboard UI — Insights Tab

**Files:**
- Modify: `pkg/cli/templates/home.html` (add tab button + content section)
- Modify: `pkg/cli/assets/js/app.js` (add chart loader + tab case)
- Modify: `pkg/cli/assets/css/app.css` (add insight card styles)

**Step 1: Add tab button to home.html**

Insert before the events button in the tab bar (line 78):

```html
<button class="tab-btn" data-tab="insights">Insights</button>
```

**Step 2: Add tab content section**

After the Events tab `</div>` (before "End Middle" comment):

```html
<!-- Insights Tab -->
<div class="tab-content" data-tab="insights">
    <section class="grid">
        <article class="grid-full-width">
            <div class="tbl">
                <div class="content-header">
                    Key Observations
                </div>
                <div id="insights-observations" class="insights-list">
                    <span class="insight-label">No insights generated yet. Insights are generated during sync when ANTHROPIC_API_KEY is set.</span>
                </div>
            </div>
        </article>
        <article class="grid-full-width">
            <div class="tbl">
                <div class="content-header">
                    Action Items
                </div>
                <div id="insights-actions" class="insights-list">
                    <span class="insight-label">—</span>
                </div>
            </div>
        </article>
        <div id="insights-meta" class="insights-meta"></div>
    </section>
</div>
```

**Step 3: Add CSS styles**

Append to `app.css`:

```css
.insights-list { padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; }
.insights-bullet { padding: 0.75rem 1rem; border-left: 3px solid var(--accent); background: var(--bg-muted, #f6f8fa10); border-radius: 0 4px 4px 0; }
.insights-bullet-headline { font-weight: 700; color: var(--fg); margin-bottom: 0.25rem; }
.insights-bullet-detail { font-size: 0.9rem; color: var(--fg-muted); line-height: 1.5; }
.insights-bullet.action { border-left-color: var(--accent-green, #2ea043); }
.insights-meta { padding: 0.5rem 1rem; font-size: 0.8rem; color: var(--gray); text-align: right; }
```

**Step 4: Add JS chart loader**

Add function in `app.js`:

```javascript
function loadGeneratedInsights(url) {
    $.get(url, function (data) {
        var obsContainer = $("#insights-observations");
        var actContainer = $("#insights-actions");
        var metaContainer = $("#insights-meta");
        obsContainer.empty();
        actContainer.empty();
        metaContainer.empty();

        if (!data || data.length === 0 || !data[0].insights) {
            obsContainer.html('<span class="insight-label">No insights generated yet. Insights are generated during sync when ANTHROPIC_API_KEY is set.</span>');
            actContainer.html('<span class="insight-label">—</span>');
            return;
        }

        var item = data[0];
        var insights = item.insights;

        if (insights.observations && insights.observations.length > 0) {
            $.each(insights.observations, function (i, o) {
                $('<div class="insights-bullet">')
                    .append('<div class="insights-bullet-headline">' + o.headline + '</div>')
                    .append('<div class="insights-bullet-detail">' + o.detail + '</div>')
                    .appendTo(obsContainer);
            });
        }

        if (insights.actions && insights.actions.length > 0) {
            $.each(insights.actions, function (i, a) {
                $('<div class="insights-bullet action">')
                    .append('<div class="insights-bullet-headline">' + a.headline + '</div>')
                    .append('<div class="insights-bullet-detail">' + a.detail + '</div>')
                    .appendTo(actContainer);
            });
        }

        metaContainer.text('Generated ' + item.generated_at + ' · ' + item.model + ' · ' + item.period_months + ' month period');
    });
}
```

**Step 5: Add case to loadTabCharts**

Before `case 'events':`:

```javascript
case 'insights':
    loadGeneratedInsights('/data/insights/generated?o=' + org + '&r=' + repo);
    break;
```

**Step 6: Verify build**

Run: `go build ./...`

**Step 7: Commit**

```bash
git commit -S -m "feat: add Insights dashboard tab"
```

---

### Task 8: Full Qualification

**Step 1: Run full qualification**

Run: `make qualify`
Expected: All tests pass, 0 lint issues, no vulnerabilities.

**Step 2: Fix any findings**

**Step 3: Commit fixes if needed**

```bash
git commit -S -m "fix: address qualify findings"
```

---

## Unresolved Questions

1. **JSON cleanup:** Claude may wrap JSON in markdown code fences. The `parseInsightsResponse` function should strip leading/trailing ` ```json ` and ` ``` ` if present.
2. **Retry on parse failure:** If JSON parsing fails, should we retry the API call with a stronger instruction? Or just log the error and skip?
3. **Prompt tuning:** The prompt template may need iteration based on real output quality. Start simple, refine later.
