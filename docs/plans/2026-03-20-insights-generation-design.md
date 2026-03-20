# Automated Insights Generation — Design

## Goal

Add an LLM-powered insights generation step to the sync pipeline that produces 5 key observations and 3 action items per repo, stored in the DB, served on a new "Insights" dashboard tab.

## Data Flow

```
Sync job (hourly, per repo)
  -> Import -> Affiliations -> Substitutions -> Extras -> Reputation -> Score
  -> [NEW] Insights: check staleness -> if stale + API key present -> gather metrics -> call Claude -> store JSON
```

## Configuration

| Setting | Source | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | env var | empty (skip step gracefully) |
| `ANTHROPIC_MODEL` | env var | `claude-haiku-4-5-20251001` |
| `--insights-stale` | CLI flag + `DEVPULSE_INSIGHTS_STALE` | `7d` |
| `--insights-period` | CLI flag + `DEVPULSE_INSIGHTS_PERIOD` | `3` (months) |

When `ANTHROPIC_API_KEY` is empty, log debug and return nil.

## Storage

Table `repo_insights`:

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

Upsert per repo. Single row (latest only).

JSON schema:

```json
{
  "observations": [
    {"headline": "...", "detail": "..."}
  ],
  "actions": [
    {"headline": "...", "detail": "..."}
  ]
}
```

## Prompt Assembly

Gather metrics via Store methods (no HTTP calls):
- GetInsightsSummary, GetContributorMomentum, GetPRReviewRatio
- GetTimeToMerge, GetContributorRetention, GetContributorFunnel
- GetChangeFailureRate, GetReviewLatency, GetPRSizeDistribution
- GetForksAndActivity, GetRepoMetas, GetIssueOpenCloseRatio
- GetTimeToFirstResponse, GetReleaseCadence, GetReleaseDownloads

Marshal into JSON, embed in prompt template with DORA benchmarks.
Request structured JSON output (5 observations, 3 actions).

## Claude API Integration

Raw HTTP POST to `https://api.anthropic.com/v1/messages` — no new SDK dependency.
Matches project pattern of using `net/http` directly.

## Store Interface

```go
type InsightsGenerationStore interface {
    GetRepoInsights(org, repo *string) (*RepoInsights, error)
    SaveRepoInsights(org, repo string, insights *RepoInsights) error
    GetRepoInsightsAge(org, repo string) (time.Duration, error)
}
```

## HTTP Endpoint

`GET /data/insights/repo-insights?o={org}&r={repo}`

## Dashboard

New "Insights" tab after Events:
- Generated date + model (small text)
- Observations: styled bullet cards with bold headline + detail
- Actions: styled bullet cards (different accent color)
- Placeholder when empty

## Files

| Layer | Files |
|---|---|
| Migration | `017_repo_insights.sql` (sqlite + postgres) |
| Types | `pkg/data/types.go` |
| Store interface | `pkg/data/store.go` |
| SQLite | `pkg/data/sqlite/repo_insights.go` (new) |
| PostgreSQL | `pkg/data/postgres/repo_insights.go` (new) |
| LLM client | `pkg/data/insights.go` (new) |
| Sync | `pkg/cli/sync.go` |
| Handler | `pkg/cli/data.go` |
| Routes | `pkg/cli/server.go` |
| Template | `pkg/cli/templates/home.html` |
| JS | `pkg/cli/assets/js/app.js` |
| CSS | `pkg/cli/assets/css/app.css` |

## Decisions

- Frequency/period parameterized via CLI flags with env var fallback
- Graceful skip when API key absent (no error)
- Raw HTTP for Claude API (no SDK dependency)
- Haiku default, configurable to Sonnet via env var
- Structured JSON output for frontend rendering flexibility
