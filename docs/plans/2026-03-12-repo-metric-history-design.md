# Stars & Forks Time-Series Tracking

## Problem

The `repo_meta` table stores only the latest snapshot of stars, forks, and open issues. There is no historical data to chart trends over time.

## Goal

Track daily star and fork counts per repo over a rolling 30-day window. Backfill the last 30 days from GitHub on first import, then append a snapshot on each subsequent `import meta`.

## Approach

### Data Layer

**New migration `009_repo_metric_history.sql`:**

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

**Backfill function `ImportRepoMetricHistory(dbPath, token, owner, repo string) error`:**

- Get current star/fork totals from `Repositories.Get` (already done in `ImportRepoMeta`)
- Page through `Activity.ListStargazers` (newest first via page math) collecting `starred_at` timestamps
- Stop paging once all entries on a page are older than 30 days
- Page through `Repositories.ListForks` sorted by `newest`, collecting `created_at` timestamps
- Stop paging once all entries are older than 30 days
- Count new stars/forks per day, reconstruct daily totals by subtracting from current counts backward
- Upsert one row per repo per day into `repo_metric_history`

**Forward recording in `ImportRepoMeta`:**

After the existing `repo_meta` upsert, also upsert today's date into `repo_metric_history` with current star/fork counts.

**Query function `GetRepoMetricHistory(db *sql.DB, org, repo *string) ([]*RepoMetricHistory, error)`:**

Returns `(org, repo, date, stars, forks)` rows ordered by date.

### API Layer

New endpoint: `GET /data/insights/repo-metric-history?org=x&repo=y`

Returns JSON array of `{org, repo, date, stars, forks}` objects.

### Dashboard

1. **Metadata panel** - small inline sparkline chart showing stars + forks over 30 days alongside existing table
2. **Stars Trend panel** - dedicated line chart for star count over time
3. **Forks Trend panel** - dedicated line chart for fork count over time

All panels respect the existing org/repo filter query params.

### CLI Integration

Backfill runs automatically during `import meta`. Uses existing `checkRateLimit` pattern for rate-limit awareness.

### Rate Limits

`ListStargazers` paginates at 100/page. We page backward from newest and stop once past the 30-day window. For most repos this is a small number of pages. Same for forks.

## Non-Goals

- Full historical backfill beyond 30 days
- Tracking open issues over time (could be added later with same pattern)
- Per-entity filtering on these charts (stars/forks are repo-level, not developer-level)
