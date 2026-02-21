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

## Charts

All charts are interactive. Click on data points to filter the event search results.

### Monthly Activity
Stacked bar chart of event types (PRs, reviews, issues, comments, forks) with a total line and 3-month moving average trend line.

### Top Entities / Top Collaborators
Entity and developer contribution distribution. Click an entity to see its affiliated developers in a popover. Legend items can be clicked to exclude them.

### Project Health
Bus factor and pony factor: the minimum number of developers (or organizations) producing 50% of all contributions.

### Contributor Retention
Stacked bar chart showing new vs returning contributors per month.

### PR Review Ratio
Monthly PR and review counts with the ratio on a secondary axis.

### Repository Metadata
Aggregated stars, forks, open issues, primary language, license, and repo count.

### Release Cadence
Monthly release counts split by total vs stable (non-prerelease).

### Time to Close / Time to Merge
Average days to close issues and merge PRs per month, with volume overlay.

## Events table

Filtered by scope, search, and chart clicks. Each row links directly to the PR, issue, or developer profile on GitHub. Paginated with prev/next controls.
