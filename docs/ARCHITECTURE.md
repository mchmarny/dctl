# Architecture

`devpulse` is a Go CLI that imports GitHub contribution data into a SQLite database and serves a browser-based analytics dashboard.

## High-Level Data Flow

```
GitHub API ──→ devpulse import ──→ SQLite (~/.devpulse/data.db)
                                       │
CNCF gitdm ──→ affiliations ──────────┘
                                       │
                                       ├──→ devpulse server ──→ localhost:8080 (Chart.js dashboard)
                                       ├──→ devpulse query  ──→ JSON (stdout)
                                       ├──→ devpulse score  ──→ GitHub API (deep reputation)
                                       └──→ devpulse sync   ──→ scheduled import + score (round-robin)
```

## Directory Structure

```
devpulse/
├── cmd/devpulse/       Main entrypoint (thin wrapper, delegates to pkg/cli)
├── pkg/
│   ├── cli/            CLI commands, HTTP handlers, templates, static assets
│   │   ├── assets/     Frontend: CSS, JS, images (embedded via go:embed)
│   │   └── templates/  HTML templates: header, home (tabbed dashboard), footer
│   ├── data/           Store interface, shared types, helpers
│   │   ├── sqlite/     SQLite Store implementation + migrations
│   │   └── ghutil/     Shared GitHub API helpers (rate limiting, user mapping)
│   ├── auth/           GitHub OAuth device flow + OS keychain token storage
│   ├── logging/        Structured logging setup (slog)
│   └── net/            HTTP client utilities with rate limit handling
├── tools/              Dev scripts (version bump, shared helpers)
├── docs/               Documentation
├── .github/            CI/CD workflows and composite actions
└── .settings.yaml      Centralized tool versions and quality thresholds
```

## CLI Commands

| Command | Purpose |
|---------|---------|
| `auth` | GitHub OAuth device flow, stores token in OS keychain |
| `import` | Fetch events, affiliations, metadata, releases, reputation from GitHub API |
| `score` | Deep-score lowest-reputation contributors via GitHub API |
| `sync` | Scheduled import + score for one repo from a config file (round-robin by hour) |
| `delete` | Remove imported data for an org or repo |
| `substitute` | Normalize entity names (e.g., rename company aliases) |
| `query` | Export data as JSON for scripting |
| `server` | Start local dashboard HTTP server |
| `reset` | Delete all data and start fresh |

## Data Layer

### Database

SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), pure Go, no CGO. Database file at `~/.devpulse/data.db`. Schema migrations are applied automatically on startup from `pkg/data/sqlite/sql/migrations/`.

### Key Tables

| Table | Purpose |
|-------|---------|
| `event` | Contribution events (PRs, reviews, issues, comments, forks) with timing metadata |
| `developer` | Developer profiles, entity affiliations, reputation scores (shallow + deep) |
| `repo_meta` | Repository metadata (stars, forks, language, license, last import timestamp, community profile: has_coc, has_contributing, has_readme, has_issue_template, has_pr_template, community_health_pct) |
| `repo_metric_history` | Daily star/fork counts for trend charts |
| `release` | Release tags, dates, and download counts |
| `release_asset` | Per-asset download counts |
| `container_package` | Container image versions |
| `state` | Import pagination state for incremental fetches |
| `sub` | Entity name substitution rules |
| `schema_version` | Migration tracking |

### Query Patterns

- **Optional filters**: `WHERE col = COALESCE(?, col)` — pass `nil` for no filter, a value to filter
- **Entity filter on nullable column**: `IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))`
- **Upserts**: `INSERT ... ON CONFLICT(...) DO UPDATE SET` for idempotent imports
- **Transactions**: Explicit `BEGIN`/`COMMIT` with rollback on error

## Import Pipeline

The import command runs these steps sequentially:

1. **Events** — fetch PRs, reviews, issues, comments, forks from GitHub API (concurrent, batched, with rate limit backoff and pagination state)
2. **Affiliations** — match developers to companies via CNCF gitdm data and GitHub profiles
3. **Substitutions** — apply user-defined entity name normalizations
4. **Metadata** — fetch repo stars, forks, open issues, language, license (updates `last_import_at` timestamp)
5. **Releases** — fetch release tags, dates, asset downloads
6. **Metric history** — backfill daily star/fork counts (30-day window)
7. **Reputation** — compute shallow reputation scores from local data (no API calls). Skips contributors who already have deep scores.

Running `import` with no flags re-runs all steps for every previously imported org/repo. Pagination state enables incremental imports — only new data since the last run is fetched.

### Sync Pipeline

The `sync` command is designed for scheduled (e.g., hourly) execution:

1. Loads a config file listing org/repo targets
2. Picks one repo via round-robin (`UTC hour % total repos`)
3. Runs the full import pipeline for that repo
4. Deep-scores lowest-reputation contributors (per-repo `reputation.scoreCount`)
5. Logs a structured `sync_summary` with timing metrics

## Dashboard

### Server

The HTTP server (`pkg/cli/server.go`) serves:
- **Static assets** — CSS, JS, images via `go:embed` filesystem
- **HTML templates** — Go `html/template` with header/home/footer structure
- **Data API** — 20+ JSON endpoints under `/data/` for chart data

### Frontend

- **jQuery 3.6** — DOM manipulation, AJAX calls
- **Chart.js 4.4** — all chart rendering (bar, line, pie, polar area, mixed)
- **No build step** — vanilla JS + CSS, no bundler or framework

### Layout

The dashboard is organized into:
1. **Top bar** — search input (`org:` / `repo:` prefix), period selector, theme toggle
2. **Summary banner** — global counts (orgs, repos, events, contributors, last import timestamp in GMT). Shows datetime when a repo is selected, date-only otherwise.
3. **Six tabs** — Health, Activity, Velocity, Quality, Community, Events

Charts load lazily per tab — only the active tab's API calls are made. Tab state persists in the URL hash (`#health`, `#activity`, etc.) for browser navigation.

The Health tab includes a **Repository Overview** table showing all repos with stars, forks, events, contributors, scored count, language, license, and last import date.

### Theme

Dark/light mode toggle with CSS custom properties. Theme preference saved to `localStorage`. Chart colors adapt via `Chart.defaults` overrides.

## Authentication

GitHub OAuth device flow (`pkg/auth/`):
1. Request device code from GitHub
2. User authorizes in browser
3. Poll for access token
4. Store token in OS keychain (macOS Keychain, Linux secret service, Windows Credential Manager)

Alternatively, set `GITHUB_TOKEN` environment variable to skip the auth flow. Multiple comma-separated tokens are supported for round-robin rotation in the `sync` command.

## Rate Limit Handling

All GitHub API calls go through `pkg/net/` which:
- Checks `X-RateLimit-Remaining` headers after each response
- Waits with jitter backoff when approaching the limit
- Logs rate limit state at debug level

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `test-on-push.yaml` | push to main, PRs | Calls reusable test workflow |
| `test-on-call.yaml` | reusable (workflow_call) | tidy, lint, test with race detector |
| `release-on-tag.yaml` | version tags (`v*.*.*`) | goreleaser build, cosign signing, SBOM, attestations, Homebrew tap |
| `codeql-analysis.yaml` | schedule, push | CodeQL security analysis (Go + JavaScript) |
| `scan-on-schedule.yaml` | schedule | Vulnerability scanning |
| `score-on-schedule.yaml` | schedule | Scheduled reputation scoring |
| `reputation-on-pr.yaml` | PR events | Reputation scoring on PR contributors |

## Supply Chain Security

Releases are built and signed in CI (GitHub Actions):
- **Binary signing** — keyless Sigstore/cosign via GitHub OIDC
- **Build provenance** — GitHub build attestations
- **SBOM** — SPDX JSON generated by syft for each binary
- **Vulnerability scanning** — govulncheck in CI, Trivy on schedule
- **Dependency pinning** — all GitHub Actions pinned by commit hash

Tool versions and quality thresholds are centralized in `.settings.yaml`.
