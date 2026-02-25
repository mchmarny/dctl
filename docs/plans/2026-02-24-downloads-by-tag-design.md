# Downloads by Release Tag

## Summary

Add a horizontal bar chart showing download counts per release tag. Shows the 9 most recently published tags plus the all-time most downloaded tag (if not already in the recent 9). No new migration needed -- data exists in `release_asset` + `release` tables.

## Data Layer

**SQL** (`pkg/data/release.go`): Single query using UNION -- first CTE selects 9 most recent tags by `published_at DESC`, second CTE selects top 1 by `SUM(download_count) DESC`. Outer query deduplicates and orders by `published_at`.

**Type:**

```go
type ReleaseDownloadsByTagSeries struct {
    Tags      []string `json:"tags"`
    Downloads []int    `json:"downloads"`
}
```

**Function:** `GetReleaseDownloadsByTag(db *sql.DB, org, repo *string, months int)` -- same signature pattern as `GetReleaseDownloads`.

## API

**Handler** (`pkg/cli/data.go`): `insightsReleaseDownloadsByTagAPIHandler(db)` -- same pattern as `insightsReleaseDownloadsAPIHandler`.

**Route** (`pkg/cli/server.go`): `GET /data/insights/release-downloads-by-tag`

## Dashboard

**Panel** (`pkg/cli/templates/home.html`): New `<article>` immediately after "Release Downloads" panel. Title: "Downloads by Release".

**Chart** (`pkg/cli/assets/js/app.js`): Horizontal bar chart (`type: 'bar'`, `indexAxis: 'y'`). New variable `releaseDownloadsByTagChart`, added to `destroyCharts()` and `loadAllInsightCharts()`.

## Tests

`pkg/data/release_test.go`: Three tests following existing pattern:

- `TestGetReleaseDownloadsByTag_NilDB` -- nil DB returns error
- `TestGetReleaseDownloadsByTag_EmptyDB` -- empty DB returns empty slices
- `TestGetReleaseDownloadsByTag_WithData` -- verifies recent-9 + top-1 logic and dedup when top overlaps
