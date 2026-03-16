# Multi-Database Support Design

## Goal

Add PostgreSQL support alongside SQLite by extracting a `Store` interface and moving each backend into its own package.

## Backend Detection

Single `--db` flag (already exists). Value determines backend:
- File path (default) -> SQLite
- `postgres://` or `postgresql://` URI -> PostgreSQL
- Env var `DEVPULSE_DB` works the same way

## Package Layout

```
pkg/data/
  store.go          — Store interface (composed from grouped sub-interfaces)
  types.go          — Shared types (Developer, Event, ImportSummary, etc.)
  helpers.go        — Shared utilities (sinceDate, Contains, entityRegEx, etc.)

pkg/data/sqlite/
  sqlite.go         — Constructor, Init, connection pool, migration runner
  event.go          — EventStore methods
  developer.go      — DeveloperStore methods
  insights.go       — InsightsStore methods
  query.go          — QueryStore methods
  state.go          — StateStore methods
  entity.go         — EntityStore methods
  repo.go           — RepoStore methods
  repo_meta.go      — RepoMetaStore methods
  release.go        — ReleaseStore methods
  container.go      — ContainerStore methods
  reputation.go     — ReputationStore methods
  metric_history.go — MetricHistoryStore methods
  org.go            — OrgStore methods
  delete.go         — DeleteStore methods
  sub.go            — SubstitutionStore methods
  cncf.go           — CNCF affiliation logic (uses DeveloperStore)
  sql/migrations/   — Current SQLite migration files (moved as-is)
  *_test.go         — SQLite-specific tests

pkg/data/postgres/  — (future PR) PostgreSQL implementation
  postgres.go
  sql/migrations/
  ...
```

## Interface Design

Grouped sub-interfaces composed into a single `Store`:

```go
type Store interface {
    EventStore
    DeveloperStore
    InsightsStore
    QueryStore
    StateStore
    EntityStore
    RepoStore
    RepoMetaStore
    ReleaseStore
    ContainerStore
    ReputationStore
    MetricHistoryStore
    OrgStore
    DeleteStore
    SubstitutionStore
    io.Closer
}
```

Each sub-interface maps to the public functions in its current file. Example:

```go
type StateStore interface {
    GetState(query, org, repo string, min time.Time) (*State, error)
    SaveState(query, org, repo string, state *State) error
    ClearState(org, repo string) error
    GetDataState() (map[string]int64, error)
}
```

Methods drop the `db *sql.DB` first parameter — the store holds the connection pool internally.

## CLI Wiring

`appConfig` changes from `DB *sql.DB` to `Store data.Store`.

```go
func openStore(dsn string) (data.Store, error) {
    if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
        return nil, fmt.Errorf("postgres support not yet implemented")
    }
    return sqlite.New(dsn)
}
```

All HTTP handlers and CLI commands receive `data.Store` instead of `*sql.DB`.

## Connection Management

- Each backend uses `database/sql` connection pooling (no manual connection juggling)
- Import functions that currently take `dbPath` and call `GetDB()` internally will instead receive the `Store` — the pool handles concurrent access
- SQLite keeps `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000` for concurrent readers

## Migrations

- Duplicate per backend — each owns its schema and dialect
- Migration runner logic (version tracking, file ordering) is a method on each store implementation
- SQLite: `datetime('now')`, `?` placeholders, `IFNULL`
- PostgreSQL (future): `NOW()`, `$1` placeholders, `COALESCE`

## CNCF Affiliations

`UpdateDevelopersWithCNCFEntityAffiliations` accepts `DeveloperStore` (not full `Store`) since it only needs developer read/write methods.

## Testing

- Tests are backend-specific, living in each backend package
- `setupTestDB(t)` returns `*sqlite.Store` (not `data.Store`) — tests verify backend behavior
- Future: postgres tests use testcontainers or similar

## Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Backend detection | URI prefix | Zero ambiguity, single flag |
| Migration files | Duplicated per backend | Clarity, dialect independence |
| Interface granularity | Grouped + composed | Incremental implementation, better testability |
| Connection management | Connection pool | `database/sql` handles this natively |
| Shared types | `pkg/data/` package | No backend dependency |
| SQL constants | Per backend package | Dialect-specific |
| Test helpers | Backend-specific | Tests verify actual backend behavior |

## Implementation Order

1. Define interfaces in `pkg/data/store.go`, move shared types to `types.go` and helpers to `helpers.go`
2. Create `pkg/data/sqlite/` package, move all current implementations as methods on `sqlite.Store`
3. Move migrations into `pkg/data/sqlite/sql/migrations/`
4. Update CLI (`appConfig`, `Before` hook) to use `data.Store`
5. Update all HTTP handlers to accept `data.Store`
6. Update all import functions to accept `data.Store`
7. Move and update all tests
8. Verify `make qualify` passes
9. (Future PR) Add `pkg/data/postgres/` implementation
