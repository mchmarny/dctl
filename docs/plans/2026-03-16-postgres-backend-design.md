# PostgreSQL Backend Design

## Goal

Implement `data.Store` for PostgreSQL, mirroring the SQLite implementation with Postgres-dialect SQL.

## SQL Dialect Translation

| SQLite | PostgreSQL |
|--------|-----------|
| `?` placeholders | `$1, $2, $3` numbered placeholders |
| `IFNULL(x, y)` | `COALESCE(x, y)` |
| `datetime('now')` | `NOW()` |
| `GROUP_CONCAT(x, ',')` | `STRING_AGG(x, ',')` |
| `TEXT NOT NULL DEFAULT (datetime('now'))` | `TEXT NOT NULL DEFAULT NOW()` |
| `PRAGMA` statements | Not needed |
| `LIKE` | `ILIKE` for case-insensitive |

## Package Layout

```
pkg/data/postgres/
  postgres.go        — Store struct, New(), Close(), migrations
  helpers.go         — sinceDate, bot-exclude SQL (Postgres dialect)
  state.go           — StateStore methods
  delete.go          — DeleteStore methods
  sub.go             — SubstitutionStore methods
  entity.go          — EntityStore methods
  repo.go            — RepoStore methods
  org.go             — OrgStore methods
  developer.go       — DeveloperStore methods
  cncf.go            — CNCF affiliation (standalone, takes DeveloperStore)
  query.go           — QueryStore methods
  event.go           — EventStore methods
  insights.go        — InsightsStore methods
  release.go         — ReleaseStore methods
  container.go       — ContainerStore methods
  repo_meta.go       — RepoMetaStore methods
  metric_history.go  — MetricHistoryStore methods
  reputation.go      — ReputationStore methods
  gh.go              — symlink/copy from shared github helpers
  ratelimit.go       — symlink/copy from shared rate limit
  sql/migrations/    — Postgres-dialect DDL

pkg/data/github/     — Extracted shared GitHub API helpers (gh.go, ratelimit.go)
```

## Connection

```go
func New(dsn string) (*Store, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("opening database: %w", err)
    }
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, fmt.Errorf("pinging database: %w", err)
    }
    if err := runMigrations(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("running migrations: %w", err)
    }
    return &Store{db: db}, nil
}
```

Connection pooling via `database/sql`. Pool config via `db.SetMaxOpenConns()`.

## Migrations

Translate 12 SQLite migration files to Postgres dialect:
- Keep TEXT for date columns (minimize data layer changes)
- `schema_version` uses `NOW()` instead of `datetime('now')`
- `ON CONFLICT` upserts work in both (same syntax)
- No AUTOINCREMENT — use INTEGER PRIMARY KEY or SERIAL where needed

## Shared Code

Extract GitHub API helpers and rate limiting to `pkg/data/github/` package:
- `gh.go` — mapUserToDeveloper, GetGitHubDeveloper, GetUserOrgs, GetOrgRepos, etc.
- `ratelimit.go` — checkRateLimit, abuseRetryAfter

Both sqlite and postgres packages import from this shared package.

## CLI Wiring

```go
func openStore(dsn string) (data.Store, error) {
    if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
        return postgres.New(dsn)
    }
    return sqlite.New(dsn)
}
```

## Driver

`github.com/lib/pq` — pure Go, battle-tested, `database/sql` compatible.

## Testing

- Skip when no Postgres: `if os.Getenv("DEVPULSE_TEST_PG") == "" { t.Skip() }`
- `setupTestDB(t)` connects to test Postgres, creates temp schema, runs migrations
- Tests mirror sqlite test structure

## Implementation Order

1. Add `github.com/lib/pq` dependency
2. Extract `pkg/data/github/` from sqlite gh.go + ratelimit.go, update sqlite imports
3. Create `pkg/data/postgres/postgres.go` scaffold (Store, New, Close, migrations)
4. Translate migration SQL files to Postgres dialect
5. Implement Store methods file-by-file (state, delete, sub, entity, repo, org, developer, cncf, query, event, insights, release, container, repo_meta, metric_history, reputation)
6. Wire openStore() in CLI
7. Test against real Postgres
8. make qualify
