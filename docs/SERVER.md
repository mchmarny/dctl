# Dashboard

> Assumes you have already [imported data](IMPORT.md).

## Start

```shell
devpulse server
```

This starts a local HTTP server and opens your browser to `http://127.0.0.1:8080`.

| Flag | Description |
|------|-------------|
| `--port` | Change the listen port (default: 8080) |
| `--no-browser` | Don't auto-open the browser |

## Layout

The dashboard has three sections:

1. **Top bar** — search input, period selector, and theme toggle on a single line
2. **Summary banner** — global counts (organizations, repositories, events, contributors, last import) that update with the active search scope
3. **Tabbed panels** — six tabs with lazy-loaded charts: Health, Activity, Velocity, Quality, Community, Events

## Search

The search bar uses prefix syntax to scope the dashboard:

| Prefix | Example | Scope |
|--------|---------|-------|
| `org:` | `org:myorg` | All repos in an organization |
| `repo:` | `repo:skyhook` | Single repository |

Typing without a prefix defaults to org search. Clearing the search bar resets to the all-data view.

All tabs and the summary banner respect the active search scope.

## Time period

The period dropdown (in the top bar) adjusts the time window for all charts. Available options are computed from the earliest event matching the current search scope. Changing the period reloads the summary banner and the active tab.

## Tabs

Charts load lazily — only the active tab's data is fetched. Switching tabs loads their charts on demand. URL hash fragments (`#health`, `#activity`, etc.) track the active tab, so browser back/forward and bookmarks work.

### Health

- **Project Health** — bus factor and pony factor with daily activity sparkline
- **Repository Metadata** — stars, forks, open issues, language, license, repo count with sparkline
- **Stars Trend** — daily star count over the last 30 days
- **Forks Trend** — daily fork count over the last 30 days

### Activity

- **Monthly Activity** — GitHub events grouped by month with total line and linear regression trend
- **PR Size Distribution** — pull requests bucketed by lines changed (S/M/L/XL) per month
- **Forks & Activity** — monthly fork count vs total event activity

### Velocity

- **Lead Time (PR to Merge)** — average days from PR creation to merge
- **Change Failure Rate** — percentage of deployments causing failures
- **Release Cadence** — monthly release counts (total, stable, deployments)
- **Release Downloads** — monthly download trends
- **Downloads by Release** — top releases by download count

### Quality

- **PR Review Ratio** — PRs to reviews per month with ratio trend line
- **Review Latency** — average hours from PR creation to first review
- **Time to Close** — average days to close all issues vs bug issues near releases
- **Contributor Reputation** — two-tier scoring with known bot filtering; click a bar for deep score

### Community

- **Contributor Retention** — new vs returning contributors per month
- **Contributor Momentum** — rolling 3-month active contributor count with delta
- **First-Time Contributors** — new contributor milestones per month
- **Top Entities** — contributing companies/orgs with drill-down to developers
- **Top Collaborators** — ranked by total event count

### Events

- **Event Search** — filter by type, date range, username, or entity
- **Chart integration** — clicking chart data points populates filters and navigates to the Events tab
- **Pagination** — prev/next controls for paging through results

Each result row links directly to the PR, issue, or developer profile on GitHub.

## Continuous Import

You can run the server and import in parallel. The server reads from SQLite in WAL mode, so an import running in a separate terminal (or cron job) will not block dashboard queries. The dashboard sees new data as soon as each import transaction commits — no server restart required.

### Example: server + hourly import

Terminal 1 — keep the dashboard running:

```shell
devpulse server
```

Terminal 2 — run a one-off update:

```shell
devpulse import
```

Or schedule it with cron (every hour):

```shell
# crontab -e
0 * * * * /usr/local/bin/devpulse import >> /tmp/devpulse-import.log 2>&1
```

The `--concurrency` flag controls how many repos import in parallel (default: 3). Higher values speed up imports but consume more GitHub API quota:

```shell
devpulse import --concurrency 2
```
