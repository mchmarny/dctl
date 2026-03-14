# Architecture

`devpulse` is a self-contained Go CLI that imports GitHub contribution data into a local SQLite database and serves a browser-based analytics dashboard. No external services, no cloud dependencies at runtime. All data stays on your machine.

## High-Level Data Flow

```
GitHub API ──→ devpulse import ──→ SQLite (~/.devpulse/data.db)
                                       │
CNCF gitdm ──→ affiliations ──────────┘
                                       │
                                       ├──→ devpulse server ──→ localhost:8080 (Chart.js dashboard)
                                       ├──→ devpulse query  ──→ JSON (stdout)
                                       └──→ devpulse score  ──→ GitHub API (deep reputation)
```

## Directory Structure

```
devpulse/
├── cmd/devpulse/       Main entrypoint (thin wrapper, delegates to pkg/cli)
├── pkg/
│   ├── cli/            CLI commands, HTTP handlers, templates, static assets
│   │   ├── assets/     Frontend: CSS, JS, images (embedded via go:embed)
│   │   └── templates/  HTML templates: header, home (tabbed dashboard), footer
│   ├── data/           Data layer: SQLite queries, GitHub importers, migrations
│   │   └── sql/        SQL migration files applied on startup
│   ├── auth/           GitHub OAuth device flow + OS keychain token storage
│   ├── logging/        Structured logging setup (slog)
│   └── net/            HTTP client utilities with rate limit handling
├── tools/              Dev scripts (version bump, e2e tests)
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
| `delete` | Remove imported data for an org or repo |
| `substitute` | Normalize entity names (e.g., rename company aliases) |
| `query` | Export data as JSON for scripting |
| `server` | Start local dashboard HTTP server |
| `reset` | Delete all data and start fresh |

## Data Layer

### Database

SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGO required. Database file lives at `~/.devpulse/data.db`. Schema migrations are applied automatically on startup.

### Key Tables

| Table | Purpose |
|-------|---------|
| `event` | Contribution events (PRs, reviews, issues, comments, forks) with timing metadata |
| `developer` | Developer profiles, entity affiliations, reputation scores |
| `repo_meta` | Repository metadata (stars, forks, language, license) |
| `repo_metric_history` | Daily star/fork counts for trend charts |
| `release` | Release tags, dates, and download counts |
| `release_asset` | Per-asset download counts |
| `state` | Import pagination state for incremental fetches |
| `sub` | Entity name substitution rules |

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
4. **Metadata** — fetch repo stars, forks, open issues, language, license
5. **Releases** — fetch release tags, dates, asset downloads
6. **Metric history** — backfill daily star/fork counts (30-day window)
7. **Reputation** — compute shallow reputation scores from local data (no API calls)

Running `import` with no flags re-runs all steps for every previously imported org/repo. Pagination state enables incremental imports — only new data since the last run is fetched.

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
2. **Summary banner** — global counts (orgs, repos, events, contributors, last import)
3. **Six tabs** — Health, Activity, Velocity, Quality, Community, Events

Charts load lazily per tab — only the active tab's API calls are made. Tab state persists in the URL hash (`#health`, `#activity`, etc.) for browser navigation.

### Theme

Dark/light mode toggle with CSS custom properties. Theme preference saved to `localStorage`. Chart colors adapt via `Chart.defaults` overrides.

## Authentication

GitHub OAuth device flow (`pkg/auth/`):
1. Request device code from GitHub
2. User authorizes in browser
3. Poll for access token
4. Store token in OS keychain (macOS Keychain, Linux secret service, Windows Credential Manager)

Alternatively, set `GITHUB_TOKEN` environment variable to skip the auth flow.

## Rate Limit Handling

All GitHub API calls go through `pkg/net/` which:
- Checks `X-RateLimit-Remaining` headers after each response
- Waits with jitter backoff when approaching the limit
- Logs rate limit state at debug level

## Supply Chain Security

Releases are built and signed in CI (GitHub Actions):
- **Binary signing** — keyless Sigstore/cosign via GitHub OIDC
- **Build provenance** — GitHub build attestations
- **SBOM** — SPDX JSON generated by syft for each binary
- **Vulnerability scanning** — govulncheck in CI, Trivy on schedule
- **Dependency pinning** — all GitHub Actions pinned by commit hash

Tool versions and quality thresholds are centralized in `.settings.yaml`.
