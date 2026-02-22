# Dashboard

> Assumes you have already [imported data](IMPORT.md).

## Start

```shell
dctl server
```

This starts a local HTTP server and opens your browser to `http://127.0.0.1:8080`.

| Flag | Description |
|------|-------------|
| `--port` | Change the listen port (default: 8080) |
| `--no-browser` | Don't auto-open the browser |

## Search

The search bar uses prefix syntax to scope the dashboard:

| Prefix | Example | Scope |
|--------|---------|-------|
| `org:` | `org:nvidia` | All repos in an organization |
| `repo:` | `repo:skyhook` | Single repository |
| `entity:` | `entity:google` | Company/org affiliation |

Typing without a prefix defaults to org search. Clearing the search bar resets to the all-data view.

All panels respect the active search scope. For example, `entity:google` filters not just the entity chart but also retention, PR ratio, velocity, reputation, and all other contributor-based panels.

## Time period

The period dropdown (next to the search heading) adjusts the time window for all charts. Available options are computed from the earliest event matching the current search scope. Changing the period reloads all panels.

## Charts

All charts are interactive. Click on data points to filter the event search results. Each panel includes a brief description of its data source and methodology.

### Monthly Activity
GitHub events (PRs, reviews, issues, comments, forks) grouped by month with a total line and linear regression trend.

### Top Entities / Top Collaborators
Entity and developer contribution distribution. Entity affiliations come from GitHub profile company fields and CNCF gitdm data (self-reported, not verified). Click an entity to see its affiliated developers in a popover. Legend items can be clicked to exclude them.

### Project Health
Bus factor and pony factor: the minimum number of developers (or organizations) producing 50% of all contributions.

### Contributor Retention
New (first contribution that month) vs returning (contributed in a prior month) contributors per month.

### Lowest Reputation Contributors
Two-tier scoring: shallow scores use local data only; click a bar for a full GitHub API-enriched deep score. The chart refreshes automatically after a deep score is computed. Known bot accounts (GitHub Apps, Copilot, Claude) are excluded.

### PR Review Ratio
Monthly PR and review counts with the ratio on a secondary axis. Higher ratio suggests stronger code review culture.

### Repository Metadata
Snapshot from GitHub API at last import. Aggregated stars, forks, open issues, primary language, license, and repo count.

### Release Cadence
GitHub releases per month. Stable excludes pre-releases and drafts.

### Time to Close / Time to Merge
Average days from open to close/merge based on GitHub created/closed/merged timestamps, with volume overlay.

## Event search

The Event Search panel is always visible at the bottom of the dashboard. It provides:

- **Filter form** -- event type dropdown, from/to date inputs, username, and entity text fields
- **Chart integration** -- clicking chart data points populates the filter inputs and triggers a search
- **Pagination** -- prev/next controls for paging through results

Each result row links directly to the PR, issue, or developer profile on GitHub.
