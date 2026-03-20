# devpulse

[![Build Status](https://github.com/mchmarny/devpulse/actions/workflows/test-on-push.yaml/badge.svg)](https://github.com/mchmarny/devpulse/actions/workflows/test-on-push.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mchmarny/devpulse)](https://goreportcard.com/report/github.com/mchmarny/devpulse)
[![Go Reference](https://pkg.go.dev/badge/github.com/mchmarny/devpulse.svg)](https://pkg.go.dev/github.com/mchmarny/devpulse)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Community health analytics for GitHub organizations and repositories. `devpulse` imports contribution data from the GitHub API, enriches it with developer affiliations, and surfaces project health insights through a local dashboard. 

> Motivation and the drivers behind this project are covered in the blog post [here](https://blog.chmarny.com/posts/devpulse-community-health-analytics-for-the-rest-of-us/)

## Features

**Project Health**
- **Bus factor / pony factor** -- minimum developers or organizations producing 50% of contributions
- **Repository Status** -- stars, forks, open issues, language, license with 30-day sparkline
- **Stars & forks trends** -- daily star and fork counts over the last 30 days with historical backfill
- **Community Profile** -- badges for README, Contributing, Code of Conduct, and issue/PR templates

![](docs/img/health.png)

**Activity & Code**
- **Activity trends** -- monthly event volume (PRs, reviews, issues, comments, forks) with total and 3-month moving average
- **PR size distribution** -- pull requests bucketed by lines changed (S/M/L/XL) per month
- **Forks & activity** -- monthly fork count vs total event activity
- **Issue Open/Close Ratio** -- monthly opened vs closed issues

![](docs/img/activity.png)

**Velocity**
- **Lead time (PR to merge)** -- average days from PR creation to merge
- **Change failure rate** -- percentage of deployments causing failures (bug issues near releases + revert PRs)
- **Release cadence** -- monthly release counts (total, stable, deployments) with merge-to-main fallback
- **Release downloads** -- monthly download trends and top releases by download count
- **Time to First Response** -- average hours to first review or comment on PRs

![](docs/img/velocity.png)

**Quality**
- **PR review ratio** -- PRs to reviews per month with ratio trend line
- **Review latency** -- average hours from PR creation to first review
- **Time to close** -- average days to close all issues vs bug issues near releases
- **Contributor reputation** -- two-tier scoring (shallow local, deep GitHub API) with known bot filtering

![](docs/img/quality.png)

**Community**
- **Contributor retention** -- new vs returning contributors per month
- **Contributor momentum** -- rolling 3-month active contributor count with month-over-month delta
- **First-time contributor funnel** -- new contributor milestones per month (first comment, first PR, first merge)
- **Entity affiliations** -- top contributing companies/orgs with drill-down to individual developers (GitHub profile + CNCF gitdm)

![](docs/img/community.png)

**Insights**
- **LLM-generated observations** -- AI-powered analysis of repository health, trends, and action items per repo

**Dashboard**
- **Global summary banner** -- organizations, repositories, events, contributors, and last import timestamp (GMT) at a glance
- **Tabbed layout** -- Health, Activity, Velocity, Quality, Community, Insights, and Events tabs with lazy-loaded charts
- **Event search filters** -- filter by type, date range, username, or entity from the Events tab
- **Adjustable time period** -- dropdown adapts to available data range per search scope
- **Unified search** -- `org:name` or `repo:name` prefix syntax; all panels respect scope

![](docs/img/global.png)

## Install

### Homebrew (macOS / Linux)

```shell
brew tap mchmarny/tap
brew install devpulse
```

### Binary releases

Visit [latest releases](https://github.com/mchmarny/devpulse/releases/latest) page.

### Build from source

Requires [Go](https://go.dev/) 1.26+.

```shell
git clone https://github.com/mchmarny/devpulse.git
cd devpulse
make build
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for details.

## Quick start

### 1. Authentication

`devpulse` uses GitHub's device flow for OAuth. By default the token requests the `repo` scope for private repository access. Use `--public` to skip the `repo` scope when you only work with public repos. The token is stored in your OS keychain.

```shell
devpulse auth            # private + public repo access (default)
devpulse auth --public   # public repo access only
```

Alternatively, set `GITHUB_TOKEN` to skip the auth command entirely:

```shell
export GITHUB_TOKEN=ghp_...
```

### 2. Data import

Import events, affiliations, metadata, releases, and reputation for an org:

```shell
devpulse import --org <org>
```

Or target specific repos:

```shell
devpulse import --org <org> --repo <repo1> --repo <repo2>
```

Use `--fresh` to clear pagination state and re-import from scratch:

```shell
devpulse import --org <org> --fresh
```

Update all previously imported data (no flags needed):

```shell
devpulse import
```

Control how many repos import in parallel (default: 3):

```shell
devpulse import --concurrency 2
```

See [docs/IMPORT.md](docs/IMPORT.md) for all import options.

### 3. Reputation score

Import computes basic reputation scores automatically. For deeper scoring using GitHub API signals (profile age, org membership, PR history, etc.):

```shell
devpulse score --org <org>                      # deep-score 5 lowest in org (default)
devpulse score --org <org> --repo <repo>        # scope to a specific repo
devpulse score --org <org> --count 20           # deep-score 20 lowest
```

Run incrementally — each invocation scores the next batch of lowest-reputation contributors. See [docs/SCORE.md](docs/SCORE.md) for details.

### 4. Scheduled sync

For automated, scheduled imports, `sync` reads a config file and imports + scores one repo per run using hour-based round-robin rotation:

```shell
devpulse sync --config sync.yaml
devpulse sync --config https://raw.githubusercontent.com/org/repo/main/config/sync.yaml

# Override round-robin to sync a specific repo
devpulse sync --config sync.yaml --org NVIDIA --repo DCGM
```

Config format:

```yaml
sources:
  - org: myorg
    repos:
      - repo1
      - repo2
  - org: mchmarny
    repos:
      - devpulse
score:
  count: 100
```

The `--config` flag (or `DEVPULSE_SYNC_CONFIG` env var) accepts a local file path or HTTP(S) URL. Each run picks one repo from the flattened list based on `UTC hour % total repos`, runs a full import, then deep-scores up to `score.count` lowest-reputation contributors. With 9 repos on an hourly schedule, each repo is imported 2-3 times per day while staying within GitHub's 5,000 requests/hour API rate limit.

Stale threshold controls how recently a contributor must have been scored to skip re-scoring (default: `3d`):

```shell
devpulse sync --config sync.yaml --stale 1w
```

### 5. Dashboard view

```shell
devpulse server
```

Opens your browser to `http://127.0.0.1:8080`. Use `--port` to change the port or `--no-browser` to suppress auto-open.

The dashboard shows a global summary banner (orgs, repos, events, contributors, last import timestamp in GMT) and organizes insights into seven tabs: **Health**, **Activity**, **Velocity**, **Quality**, **Community**, **Insights**, and **Events**. Charts load lazily per tab.

You can run `devpulse import` in a separate terminal or cron job while the server is running — the dashboard picks up new data immediately after each import transaction commits. See [docs/SERVER.md](docs/SERVER.md) for details.

Use the search bar with prefix syntax to scope the dashboard:

| Prefix | Example | Scope |
|--------|---------|-------|
| `org:` | `org:myorg` | All repos in an organization |
| `repo:` | `repo:skyhook` | Single repository |

No prefix defaults to org search.

### 6. Programmatic query

`devpulse` also exposes data as JSON for scripting:

```shell
devpulse query events --org knative --repo serving --type pr --since 2024-01-01
devpulse query developer list --like mark
devpulse query entity detail --name GOOGLE
```

See [docs/QUERY.md](docs/QUERY.md) for all query options.

### 7. Data deletion

Remove imported data for a specific org or repo:

```shell
devpulse delete --org <org>                          # delete all data for org
devpulse delete --org <org> --repo <repo>            # delete data for specific repo
devpulse delete --org <org> --repo <repo> --force    # skip confirmation prompt
```

### 8. Full reset

Delete all imported data and start fresh:

```shell
devpulse reset
```

Prompts for confirmation before deleting the database.

## Data sources

| Source | Data |
|--------|------|
| [GitHub API](https://docs.github.com/en/rest) | PRs, issues, comments, reviews, forks, repo metadata, releases |
| [cncf/gitdm](https://github.com/cncf/gitdm) | Developer-to-company affiliations |

Entity names are normalized automatically. Use `devpulse substitute` to correct misattributions:

```shell
devpulse substitute --type entity --old "INTERNATIONAL BUSINESS MACHINES" --new "IBM"
```

## Database

By default, data is stored locally in [SQLite](https://www.sqlite.org/) (`~/.devpulse/data.db`). No external services required.

### PostgreSQL

To use PostgreSQL instead, pass a `postgres://` connection URI via `--db` or `DEVPULSE_DB`:

```shell
devpulse import --db "postgres://user:pass@host:5432/dbname?sslmode=disable" --org <org> --repo <repo>
devpulse server --db "postgres://user:pass@host:5432/dbname?sslmode=disable"
```

Or via environment variable:

```shell
export DEVPULSE_DB="postgres://user:pass@host:5432/dbname?sslmode=disable"
devpulse import --org <org> --repo <repo>
```

Migrations run automatically on first connection. Special characters in the password must be URL-encoded (e.g., `/` → `%2F`, `@` → `%40`).

For Google Cloud AlloyDB, connect through the [AlloyDB Auth Proxy](https://cloud.google.com/alloydb/docs/auth-proxy/overview) with `--public-ip` and use `127.0.0.1` as the host.

### LLM (Insights tab)

The Insights tab uses an LLM to generate observations and action items. Configure via environment variables:

```shell
export ANTHROPIC_API_KEY="sk-ant-..."        # required for Insights tab
export ANTHROPIC_BASE_URL="https://..."      # optional, override API endpoint
export ANTHROPIC_MODEL="claude-..."          # optional, override model selection
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Verification

Release binaries are signed and attested in CI. No private keys — everything uses keyless [Sigstore](https://www.sigstore.dev/) OIDC via GitHub Actions.

### Verify checksum signature

```shell
cosign verify-blob \
  --bundle checksums-sha256.txt.sigstore.json \
  --certificate-identity-regexp 'github.com/mchmarny/devpulse' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums-sha256.txt
```

### Verify build provenance

```shell
gh attestation verify <binary> -R mchmarny/devpulse
```

### Inspect SBOM

Each binary has a corresponding SBOM (SPDX JSON) attached to the release.

## Contributing

Contributions are welcome. Please open an issue before submitting large changes. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines and [DEVELOPMENT.md](DEVELOPMENT.md) for setup.

1. Fork and clone the repository
2. Create a feature branch
3. Run `make qualify` (tests, lint, vulnerability scan)
4. Submit a pull request

## License

[Apache 2.0](LICENSE)
