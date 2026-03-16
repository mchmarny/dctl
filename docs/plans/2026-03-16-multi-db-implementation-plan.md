# Multi-Database Store Interface Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract a `Store` interface from the current `pkg/data/` package and move the SQLite implementation into `pkg/data/sqlite/`, enabling future PostgreSQL support.

**Architecture:** Define grouped sub-interfaces in `pkg/data/store.go`, extract shared types to `pkg/data/types.go`, move all SQLite-specific implementation code into `pkg/data/sqlite/` as methods on a `Store` struct that holds a `*sql.DB` connection pool. Update CLI to depend on `data.Store` interface rather than `*sql.DB` directly.

**Tech Stack:** Go 1.26, `database/sql`, `modernc.org/sqlite`, `github.com/stretchr/testify`

**Design doc:** `docs/plans/2026-03-16-multi-db-support-design.md`

---

## Phase 1: Define Interfaces and Extract Shared Types

This phase is purely additive — new files in `pkg/data/`, no existing code changes. Everything compiles after each task.

### Task 1: Create Store Interface

**Files:**
- Create: `pkg/data/store.go`

**Step 1: Create the interface file**

Create `pkg/data/store.go` with all grouped sub-interfaces composed into `Store`. Derive each method signature from the existing public functions by dropping the `db *sql.DB` first parameter. For functions that take `dbPath string` (import functions), change to take the store receiver instead.

```go
package data

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Store is the top-level interface for all data operations.
type Store interface {
	StateStore
	DeleteStore
	SubstitutionStore
	EntityStore
	RepoStore
	OrgStore
	DeveloperStore
	QueryStore
	EventStore
	InsightsStore
	ReleaseStore
	ContainerStore
	RepoMetaStore
	MetricHistoryStore
	ReputationStore
	io.Closer
}

type StateStore interface {
	GetState(query, org, repo string, min time.Time) (*State, error)
	SaveState(query, org, repo string, state *State) error
	ClearState(org, repo string) error
	GetDataState() (map[string]int64, error)
}

type DeleteStore interface {
	DeleteRepoData(org, repo string) (*DeleteResult, error)
}

type SubstitutionStore interface {
	SaveAndApplyDeveloperSub(prop, old, new string) (*Substitution, error)
	ApplySubstitutions() ([]*Substitution, error)
}

type EntityStore interface {
	GetEntityLike(query string, limit int) ([]*ListItem, error)
	GetEntity(val string) (*EntityResult, error)
	QueryEntities(val string, limit int) ([]*CountedItem, error)
	CleanEntities() error
}

type RepoStore interface {
	GetRepoLike(query string, limit int) ([]*ListItem, error)
}

type OrgStore interface {
	GetAllOrgRepos() ([]*OrgRepoItem, error)
	GetDeveloperPercentages(entity, org, repo *string, ex []string, months int) ([]*CountedItem, error)
	GetEntityPercentages(entity, org, repo *string, ex []string, months int) ([]*CountedItem, error)
	SearchDeveloperUsernames(query string, org, repo *string, months, limit int) ([]string, error)
	GetOrgLike(query string, limit int) ([]*ListItem, error)
}

type DeveloperStore interface {
	GetDeveloperUsernames() ([]string, error)
	GetNoFullnameDeveloperUsernames() ([]string, error)
	SaveDevelopers(devs []*Developer) error
	MergeDeveloper(ctx context.Context, client *http.Client, username string, cDev *CNCFDeveloper) (*Developer, error)
	GetDeveloper(username string) (*Developer, error)
	SearchDevelopers(val string, limit int) ([]*DeveloperListItem, error)
	UpdateDeveloperNames(devs map[string]string) error
}

type QueryStore interface {
	SearchEvents(q *EventSearchCriteria) ([]*EventDetails, error)
	GetMinEventDate(org, repo *string) (string, error)
	GetEventTypeSeries(org, repo, entity *string, months int) (*EventTypeSeries, error)
}

type EventStore interface {
	ImportEvents(ctx context.Context, token, owner, repo string, months int) (map[string]int, *ImportSummary, error)
	UpdateEvents(ctx context.Context, token string, concurrency int) (map[string]int, error)
}

type InsightsStore interface {
	GetInsightsSummary(org, repo, entity *string, months int) (*InsightsSummary, error)
	GetDailyActivity(org, repo, entity *string, months int) (*DailyActivitySeries, error)
	GetContributorRetention(org, repo, entity *string, months int) (*RetentionSeries, error)
	GetPRReviewRatio(org, repo, entity *string, months int) (*PRReviewRatioSeries, error)
	GetChangeFailureRate(org, repo, entity *string, months int) (*ChangeFailureRateSeries, error)
	GetReviewLatency(org, repo, entity *string, months int) (*ReviewLatencySeries, error)
	GetTimeToMerge(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetTimeToClose(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetTimeToRestoreBugs(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetPRSizeDistribution(org, repo, entity *string, months int) (*PRSizeSeries, error)
	GetForksAndActivity(org, repo, entity *string, months int) (*ForksAndActivitySeries, error)
	GetContributorFunnel(org, repo, entity *string, months int) (*ContributorFunnelSeries, error)
	GetContributorMomentum(org, repo, entity *string, months int) (*MomentumSeries, error)
	GetContributorProfile(username string, org, repo, entity *string, months int) (*ContributorProfileSeries, error)
}

type ReleaseStore interface {
	ImportReleases(ctx context.Context, token, owner, repo string) error
	ImportAllReleases(ctx context.Context, token string) error
	GetReleaseCadence(org, repo, entity *string, months int) (*ReleaseCadenceSeries, error)
	GetReleaseDownloads(org, repo *string, months int) (*ReleaseDownloadsSeries, error)
	GetReleaseDownloadsByTag(org, repo *string, months int) (*ReleaseDownloadsByTagSeries, error)
}

type ContainerStore interface {
	ImportContainerVersions(ctx context.Context, token, org, repo string) error
	ImportAllContainerVersions(ctx context.Context, token string) error
	GetContainerActivity(org, repo *string, months int) (*ContainerActivitySeries, error)
}

type RepoMetaStore interface {
	ImportRepoMeta(ctx context.Context, token, owner, repo string) error
	ImportAllRepoMeta(ctx context.Context, token string) error
	GetRepoMetas(org, repo *string) ([]*RepoMeta, error)
}

type MetricHistoryStore interface {
	ImportRepoMetricHistory(ctx context.Context, token, owner, repo string) error
	ImportAllRepoMetricHistory(ctx context.Context, token string) error
	GetRepoMetricHistory(org, repo *string) ([]*RepoMetricHistory, error)
}

type ReputationStore interface {
	ImportReputation(org, repo *string) (*ReputationResult, error)
	ImportDeepReputation(ctx context.Context, token string, limit int, org, repo *string) (*DeepReputationResult, error)
	GetOrComputeDeepReputation(ctx context.Context, token, username string) (*UserReputation, error)
	ComputeDeepReputation(ctx context.Context, token, username string) (*UserReputation, error)
	GetReputationDistribution(org, repo, entity *string, months int) (*ReputationDistribution, error)
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/mchmarny/dev/devpulse && go build ./...`
Expected: Success (interfaces reference types in same package, all types exist)

**Step 3: Commit**

```bash
git add pkg/data/store.go
git commit -S -m "feat: define Store interface with grouped sub-interfaces"
```

---

### Task 2: Extract Shared Types to types.go

**Files:**
- Create: `pkg/data/types.go`
- Modify: `pkg/data/db.go` — remove `Query`, `CountedResult`
- Modify: `pkg/data/state.go` — remove `State`
- Modify: `pkg/data/delete.go` — remove `DeleteResult`
- Modify: `pkg/data/sub.go` — remove `Substitution`, `UpdatableProperties`, `entityNoise`, `entitySubstitutions`
- Modify: `pkg/data/entity.go` — remove `EntityResult`
- Modify: `pkg/data/repo.go` — remove `CountedItem`, `Repo`, `ListItem`
- Modify: `pkg/data/org.go` — remove `Org`, `OrgRepoItem`
- Modify: `pkg/data/developer.go` — remove `Developer`, `DeveloperListItem`
- Modify: `pkg/data/query.go` — remove `EventTypeSeries`, `EventDetails`, `EventSearchCriteria`
- Modify: `pkg/data/event.go` — remove `Event`, `ImportSummary`
- Modify: `pkg/data/insights.go` — remove all series types (`InsightsSummary`, `DailyActivitySeries`, `VelocitySeries`, `RetentionSeries`, `PRReviewRatioSeries`, `ChangeFailureRateSeries`, `ReviewLatencySeries`, `PRSizeSeries`, `MomentumSeries`, `ForksAndActivitySeries`, `ContributorFunnelSeries`, `ContributorProfileSeries`)
- Modify: `pkg/data/release.go` — remove `ReleaseCadenceSeries`, `ReleaseDownloadsSeries`, `ReleaseDownloadsByTagSeries`
- Modify: `pkg/data/container.go` — remove `ContainerActivitySeries`
- Modify: `pkg/data/reputation.go` — remove `ReputationResult`, `DeepReputationResult`, `ReputationDistribution`, `UserReputation`, `SignalSummary`
- Modify: `pkg/data/cncf.go` — remove `CNCFDeveloper`, `CNCFAffiliation`, `AffiliationImportResult`
- Modify: `pkg/data/metric_history.go` — remove `RepoMetricHistory`
- Modify: `pkg/data/repo_meta.go` — remove `RepoMeta`

**Step 1: Create types.go**

Move ALL public types (structs) from their current files into `pkg/data/types.go`. This is a same-package reorganization — all existing code continues to compile because types are still in `package data`.

The file should contain every public struct listed in the "Modify" list above, grouped logically (state types, event types, developer types, insights types, etc.). Keep the original struct tags and comments. Also move `UpdatableProperties`, `entityNoise`, `entitySubstitutions` vars since they're type-related configuration.

Also move the `EventAgeMonthsDefault` and other constants referenced from outside the package (like `data.EventAgeMonthsDefault` in `pkg/cli/view.go:32`).

**Step 2: Remove moved types from original files**

Delete the type definitions and moved vars/constants from each original file listed above. Keep the function implementations and their SQL constants in the original files.

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Success (same package, just file reorganization)

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass

**Step 5: Commit**

```bash
git add pkg/data/
git commit -S -m "refactor: extract shared types to pkg/data/types.go"
```

---

### Task 3: Extract Shared Helpers to helpers.go

**Files:**
- Create: `pkg/data/helpers.go`
- Modify: `pkg/data/db.go` — remove `sinceDate`, `Contains`, `nonAlphaNumRegex`, `entityRegEx`, `errDBNotInitialized`, `botExcludeSQL`, `botExcludeDSQL`, `botExcludePrSQL`

**Step 1: Create helpers.go**

Move these from `db.go` to `pkg/data/helpers.go`:
- `sinceDate()` function
- `Contains()` function
- `nonAlphaNumRegex` constant
- `entityRegEx` var
- `errDBNotInitialized` var
- `botExcludeSQL`, `botExcludeDSQL`, `botExcludePrSQL` constants
- `DataFileName` constant

**Step 2: Remove from db.go**

Remove all moved items from `db.go`. After this, `db.go` should only contain: `Init()`, `runMigrations()`, `GetDB()`, the `migrationsFS` embed, and the sqlite driver import.

**Step 3: Verify and test**

Run: `go build ./... && make test`
Expected: All pass

**Step 4: Commit**

```bash
git add pkg/data/
git commit -S -m "refactor: extract shared helpers to pkg/data/helpers.go"
```

---

## Phase 2: Create SQLite Package

This phase creates `pkg/data/sqlite/` and moves all implementations there. **Important:** During this phase, the code will NOT compile until the CLI is updated in Phase 3. Work through all tasks in Phase 2 and Phase 3 before attempting `go build`.

### Task 4: Create SQLite Store Scaffold

**Files:**
- Create: `pkg/data/sqlite/sqlite.go`
- Move: `pkg/data/sql/migrations/*.sql` → `pkg/data/sqlite/sql/migrations/*.sql`

**Step 1: Create the sqlite package**

Create `pkg/data/sqlite/sqlite.go`:

```go
package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed sql/migrations/*.sql
var migrationsFS embed.FS

// Store implements data.Store for SQLite.
type Store struct {
	db *sql.DB
}

// New creates a new SQLite Store, initializing the database and running migrations.
func New(dbFilePath string) (*Store, error) {
	if dbFilePath == "" {
		return nil, fmt.Errorf("dbFilePath not specified")
	}

	db, err := openDB(dbFilePath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbFilePath, err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB for cases that need direct access.
func (s *Store) DB() *sql.DB {
	return s.db
}

func openDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %s: %w", path, err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	return conn, nil
}

func runMigrations(db *sql.DB) error {
	// Copy the exact runMigrations logic from pkg/data/db.go
	// (bootstrap schema_version, read embedded SQL files, apply in order)
	// ... same implementation, same SQL ...
}
```

**Step 2: Move migration files**

```bash
mkdir -p pkg/data/sqlite/sql/migrations
cp pkg/data/sql/migrations/*.sql pkg/data/sqlite/sql/migrations/
```

Do NOT delete the originals yet — they're still referenced by `pkg/data/db.go` until Phase 3.

**Step 3: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: create sqlite package scaffold with migrations"
```

---

### Task 5: Move State, Delete, Sub Implementations

**Files:**
- Create: `pkg/data/sqlite/state.go`
- Create: `pkg/data/sqlite/delete.go`
- Create: `pkg/data/sqlite/sub.go`

**Step 1: Create sqlite/state.go**

Move all functions from `pkg/data/state.go` as methods on `*Store`. Pattern for every function:

Before (in `pkg/data/state.go`):
```go
func GetState(db *sql.DB, query, org, repo string, min time.Time) (*State, error) {
    if db == nil {
        return nil, errDBNotInitialized
    }
    // ... uses db.Prepare, db.Exec, etc.
}
```

After (in `pkg/data/sqlite/state.go`):
```go
package sqlite

import (
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/mchmarny/devpulse/pkg/data"
)

// SQL constants (moved from pkg/data/state.go)
const (
    insertState = `INSERT INTO state ...`  // exact same SQL
    selectState = `SELECT since, page ...` // exact same SQL
)

var stateQueries = map[string]string{ ... } // exact same map

func (s *Store) GetState(query, org, repo string, min time.Time) (*data.State, error) {
    if s.db == nil {
        return nil, data.ErrDBNotInitialized  // exported from helpers.go
    }
    // exact same body, but use s.db instead of db parameter
}
```

Apply this pattern for all 4 state functions: `GetState`, `SaveState`, `ClearState`, `GetDataState`, plus private helper `getCount`.

**Step 2: Create sqlite/delete.go**

Same pattern. Move `DeleteRepoData` and its SQL constants from `pkg/data/delete.go`.

**Step 3: Create sqlite/sub.go**

Move `SaveAndApplyDeveloperSub`, `ApplySubstitutions`, and private `applyDeveloperSub` from `pkg/data/sub.go`. Move SQL constants `insertSubSQL`, `selectSubSQL`. Reference `updateDeveloperPropertySQL` (defined in developer.go — will move in Task 6).

**Note:** `errDBNotInitialized` needs to be exported from `pkg/data/helpers.go` as `ErrDBNotInitialized` so the sqlite package can reference it. Update `helpers.go` accordingly and fix all references in `pkg/data/*.go`.

**Step 4: Commit**

```bash
git add pkg/data/sqlite/ pkg/data/helpers.go
git commit -S -m "feat: move state, delete, sub implementations to sqlite package"
```

---

### Task 6: Move Entity, Repo, Org Implementations

**Files:**
- Create: `pkg/data/sqlite/entity.go`
- Create: `pkg/data/sqlite/repo.go`
- Create: `pkg/data/sqlite/org.go`

**Step 1: Create sqlite/entity.go**

Move from `pkg/data/entity.go`: `GetEntityLike`, `GetEntity`, `QueryEntities`, `CleanEntities` and all SQL constants (`queryEntitySQL`, `selectEntityDevelopersSQL`, `selectEntityLike`, `selectEntityNamesSQL`, `updateEntityNamesSQL`).

Same method-on-Store pattern. Return types become `*data.EntityResult`, `[]*data.ListItem`, etc.

**Step 2: Create sqlite/repo.go**

Move from `pkg/data/repo.go`: `GetRepoLike` and its SQL constants.

**Step 3: Create sqlite/org.go**

Move from `pkg/data/org.go`: `GetAllOrgRepos`, `GetDeveloperPercentages`, `GetEntityPercentages`, `SearchDeveloperUsernames`, `GetOrgLike`, and private helper `getPercentages`. Move all SQL constants.

**Step 4: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: move entity, repo, org implementations to sqlite package"
```

---

### Task 7: Move Developer and CNCF Implementations

**Files:**
- Create: `pkg/data/sqlite/developer.go`
- Create: `pkg/data/sqlite/cncf.go`

**Step 1: Create sqlite/developer.go**

Move from `pkg/data/developer.go`: all public functions (`GetDeveloperUsernames`, `GetNoFullnameDeveloperUsernames`, `SaveDevelopers`, `MergeDeveloper`, `GetDeveloper`, `SearchDevelopers`, `UpdateDeveloperNames`) and private helpers (`getDBSlice`). Move all SQL constants (`insertDeveloperSQL`, `selectDeveloperSQL`, `searchDeveloperSQL`, `updateDeveloperPropertySQL`, etc.).

**Step 2: Create sqlite/cncf.go**

Move from `pkg/data/cncf.go`: `UpdateDevelopersWithCNCFEntityAffiliations` and all related functions.

**Important:** Per the design, `UpdateDevelopersWithCNCFEntityAffiliations` should accept `data.DeveloperStore` rather than full `data.Store`. However, since it currently calls `GetDeveloperUsernames`, `GetNoFullnameDeveloperUsernames`, `SaveDevelopers`, `MergeDeveloper`, and `UpdateDeveloperNames` — all of which are on `DeveloperStore` — this works. Make it a standalone function (not a method on Store):

```go
func UpdateDevelopersWithCNCFEntityAffiliations(
    ctx context.Context, ds data.DeveloperStore, client *http.Client,
) (*data.AffiliationImportResult, error) {
    usernames, err := ds.GetDeveloperUsernames()
    // ...
}
```

The CLI import.go call site changes from `data.UpdateDevelopersWithCNCFEntityAffiliations(ctx, db, client)` to `sqlite.UpdateDevelopersWithCNCFEntityAffiliations(ctx, store, client)`.

**Step 3: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: move developer and cncf implementations to sqlite package"
```

---

### Task 8: Move Query and Event Implementations

**Files:**
- Create: `pkg/data/sqlite/query.go`
- Create: `pkg/data/sqlite/event.go`

**Step 1: Create sqlite/query.go**

Move from `pkg/data/query.go`: `SearchEvents`, `GetMinEventDate`, `GetEventTypeSeries`, and all SQL constants + private helpers.

**Step 2: Create sqlite/event.go**

Move from `pkg/data/event.go`: `ImportEvents`, `UpdateEvents`, and ALL private functions (`saveEventBatch`, `parsePREvents`, `parseIssueEvents`, `parseIssueCommentEvents`, `parseForkEvents`, `importEventsByType`, `importPRSizes`, etc.) and SQL constants.

**Critical:** These functions currently call `GetDB(dbPath)` to open their own connections. Change them to use `s.db` (the pool) instead. Remove all `GetDB(dbPath)` calls within import functions. The concurrency in `UpdateEvents` uses goroutines — with connection pooling, `s.db` handles concurrent access safely.

Before:
```go
func ImportEvents(ctx context.Context, dbPath, token, owner, repo string, months int) (...) {
    db, err := GetDB(dbPath)
    defer db.Close()
    // ...
}
```

After:
```go
func (s *Store) ImportEvents(ctx context.Context, token, owner, repo string, months int) (...) {
    // use s.db directly — pool handles concurrency
    // ...
}
```

**Step 3: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: move query and event implementations to sqlite package"
```

---

### Task 9: Move Insights Implementation

**Files:**
- Create: `pkg/data/sqlite/insights.go`

**Step 1: Create sqlite/insights.go**

Move from `pkg/data/insights.go`: all 14 public functions and private helpers (`getVelocitySeries`, all SQL constants, all private query-builder helpers). This is the largest file (~1000 lines). Same mechanical pattern — method on `*Store`, use `s.db`, return `*data.XxxSeries` types.

**Step 2: Commit**

```bash
git add pkg/data/sqlite/insights.go
git commit -S -m "feat: move insights implementation to sqlite package"
```

---

### Task 10: Move Release, Container, RepoMeta, MetricHistory Implementations

**Files:**
- Create: `pkg/data/sqlite/release.go`
- Create: `pkg/data/sqlite/container.go`
- Create: `pkg/data/sqlite/repo_meta.go`
- Create: `pkg/data/sqlite/metric_history.go`

**Step 1: Create all four files**

Apply the same pattern as Task 8 for import functions: change `dbPath string` parameter to `s.db` pool access.

Each file gets its public functions, private helpers, and SQL constants moved from the corresponding `pkg/data/` file.

For functions like `ImportReleases(ctx, dbPath, token, owner, repo)` that call `GetDB(dbPath)` internally, change to `(s *Store) ImportReleases(ctx, token, owner, repo)` using `s.db`.

For `ImportAllReleases(ctx, dbPath, token)` which calls `GetAllOrgRepos(db)` and then loops calling `ImportReleases` per repo — these become `(s *Store) ImportAllReleases(ctx, token)` calling `s.GetAllOrgRepos()` and `s.ImportReleases(...)`.

**Step 2: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: move release, container, repo_meta, metric_history to sqlite package"
```

---

### Task 11: Move Reputation Implementation

**Files:**
- Create: `pkg/data/sqlite/reputation.go`

**Step 1: Create sqlite/reputation.go**

Move from `pkg/data/reputation.go`: `ImportReputation`, `ImportDeepReputation`, `GetOrComputeDeepReputation`, `ComputeDeepReputation`, `GetReputationDistribution`, and all private functions (`gatherLocalSignals`, `computeGlobalStats`, `getStaleReputationUsernames`, `getLowestReputationUsernames`, `updateReputation`, `getDistinctOrgs`).

Move all SQL constants. Note: reputation.go references the `score` sub-package if one exists — check imports and preserve them.

**Step 2: Commit**

```bash
git add pkg/data/sqlite/reputation.go
git commit -S -m "feat: move reputation implementation to sqlite package"
```

---

### Task 12: Move GitHub Helpers

**Files:**
- Create: `pkg/data/sqlite/gh.go`
- Create: `pkg/data/sqlite/ratelimit.go`

**Step 1: Create sqlite/gh.go**

Move from `pkg/data/gh.go`: all GitHub API mapping helpers (`mapUserToDeveloper`, `mapGitHubUserToDeveloperListItem`, `deref`, `trim`, `GetUserOrgs`, `GetOrgRepos`, and any other GitHub client helper functions). These are used by import and CNCF functions now in the sqlite package.

**Step 2: Create sqlite/ratelimit.go**

Move from `pkg/data/ratelimit.go`: rate limit handling logic used by import functions.

**Step 3: Commit**

```bash
git add pkg/data/sqlite/
git commit -S -m "feat: move github helpers and rate limiting to sqlite package"
```

---

## Phase 3: Update CLI to Use Store Interface

This phase switches all CLI code from `*sql.DB` to `data.Store`. After this phase, `go build ./...` should compile.

### Task 13: Update appConfig and App Initialization

**Files:**
- Modify: `pkg/cli/app.go`

**Step 1: Update appConfig**

```go
import (
    "github.com/mchmarny/devpulse/pkg/data"
    "github.com/mchmarny/devpulse/pkg/data/sqlite"
)

type appConfig struct {
    Debug bool
    Store data.Store
}
```

Remove `DBPath` field — the store holds the connection internally.

**Step 2: Update the Before hook**

```go
Before: func(ctx context.Context, cmd *urfave.Command) (context.Context, error) {
    applyFlags(cmd)
    dsn := cmd.String(dbFilePathFlag.Name)
    store, err := openStore(dsn)
    if err != nil {
        return ctx, fmt.Errorf("initializing store: %w", err)
    }
    cmd.Root().Metadata[appConfigKey] = &appConfig{
        Debug: cmd.Bool(debugFlag.Name),
        Store: store,
    }
    return ctx, nil
},
```

**Step 3: Add openStore function**

```go
func openStore(dsn string) (data.Store, error) {
    if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
        return nil, fmt.Errorf("postgres support not yet implemented")
    }
    return sqlite.New(dsn)
}
```

**Step 4: Update the After hook**

```go
After: func(ctx context.Context, cmd *urfave.Command) error {
    if cfg, ok := cmd.Root().Metadata[appConfigKey].(*appConfig); ok && cfg.Store != nil {
        if err := cfg.Store.Close(); err != nil {
            slog.Error("error closing store", "error", err)
        }
    }
    return nil
},
```

**Step 5: Update dbFilePathFlag description**

```go
dbFilePathFlag = &urfave.StringFlag{
    Name:    "db",
    Usage:   "SQLite file path or postgres:// connection URI",
    Value:   filepath.Join(getHomeDir(), data.DataFileName),
    Sources: urfave.EnvVars("DEVPULSE_DB"),
}
```

**Step 6: Commit**

```bash
git add pkg/cli/app.go
git commit -S -m "feat: update appConfig to use data.Store interface"
```

---

### Task 14: Update All HTTP Handlers

**Files:**
- Modify: `pkg/cli/data.go`
- Modify: `pkg/cli/server.go`

**Step 1: Update server.go makeRouter**

Change signature from `func makeRouter(db *sql.DB, basePath string)` to `func makeRouter(store data.Store, basePath string)`. Pass `store` to all handler factories. Update the call site in `cmdStartServer` from `makeRouter(cfg.DB, basePath)` to `makeRouter(cfg.Store, basePath)`.

**Step 2: Update all handlers in data.go**

Change every handler from `func xxxHandler(db *sql.DB) http.HandlerFunc` to `func xxxHandler(store data.Store) http.HandlerFunc`.

Inside each handler, change calls from `data.GetFoo(db, ...)` to `store.GetFoo(...)`.

Example:
```go
// Before:
func minDateAPIHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        d, err := data.GetMinEventDate(db, p.org, p.repo)

// After:
func minDateAPIHandler(store data.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        d, err := store.GetMinEventDate(p.org, p.repo)
```

Apply this to ALL ~30 handlers. The `percentageProvider` function pointer type changes from `func(db *sql.DB, ...) ([]*data.CountedItem, error)` to accepting store methods directly.

**Step 3: Commit**

```bash
git add pkg/cli/data.go pkg/cli/server.go
git commit -S -m "feat: update HTTP handlers to use data.Store interface"
```

---

### Task 15: Update CLI Commands (import, delete, query, score, sub, reset)

**Files:**
- Modify: `pkg/cli/import.go`
- Modify: `pkg/cli/delete.go`
- Modify: `pkg/cli/query.go`
- Modify: `pkg/cli/score.go`
- Modify: `pkg/cli/sub.go`
- Modify: `pkg/cli/reset.go`

**Step 1: Update import.go**

Change all `cfg.DB` to `cfg.Store` and all `cfg.DBPath` to `cfg.Store`:

```go
// Before:
data.ClearState(cfg.DB, org, r)
data.ImportEvents(ctx, cfg.DBPath, token, org, r, months)
data.ImportRepoMeta(ctx, cfg.DBPath, token, org, r)

// After:
cfg.Store.ClearState(org, r)
cfg.Store.ImportEvents(ctx, token, org, r, months)
cfg.Store.ImportRepoMeta(ctx, token, org, r)
```

Update `importAffiliations` to accept `data.DeveloperStore`:
```go
func importAffiliations(ctx context.Context, ds data.DeveloperStore) (*data.AffiliationImportResult, error) {
    // ...
    return sqlite.UpdateDevelopersWithCNCFEntityAffiliations(ctx, ds, client)
}
```

Call site: `importAffiliations(ctx, cfg.Store)` (Store embeds DeveloperStore, so it satisfies the interface).

**Step 2: Update delete.go**

Change `data.GetAllOrgRepos(cfg.DB)` → `cfg.Store.GetAllOrgRepos()` and `data.DeleteRepoData(cfg.DB, org, r)` → `cfg.Store.DeleteRepoData(org, r)`.

**Step 3: Update query.go**

Change all `data.Foo(cfg.DB, ...)` calls to `cfg.Store.Foo(...)`. For `data.GetUserOrgs` and `data.GetOrgRepos` which are GitHub API helpers (not store methods), import them from sqlite package: `sqlite.GetUserOrgs(...)`.

**Step 4: Update score.go**

Change `data.ImportDeepReputation(ctx, cfg.DB, ...)` → `cfg.Store.ImportDeepReputation(ctx, ...)`.

**Step 5: Update sub.go**

Change `data.SaveAndApplyDeveloperSub(cfg.DB, ...)` → `cfg.Store.SaveAndApplyDeveloperSub(...)`.

**Step 6: Update reset.go**

`data.Init(cfg.DBPath)` is no longer needed — the store initializes on creation. If reset needs to re-initialize, call `sqlite.New(dsn)` again or add a `Reset()` method to the Store interface.

**Step 7: Commit**

```bash
git add pkg/cli/
git commit -S -m "feat: update all CLI commands to use data.Store interface"
```

---

### Task 16: First Compilation Check

**Step 1: Build**

Run: `go build ./...`
Expected: Success. If not, fix remaining references to old `data.Foo(db, ...)` patterns.

**Step 2: Run tests**

Run: `make test`
Expected: Tests in `pkg/data/` may fail (implementations moved). Tests in `pkg/cli/` should pass if they don't depend on moved functions. This is expected — tests move in Phase 4.

**Step 3: Commit any fixes**

```bash
git add -A
git commit -S -m "fix: resolve compilation issues after store interface migration"
```

---

## Phase 4: Clean Up and Move Tests

### Task 17: Remove Old Implementation Code from pkg/data/

**Files:**
- Delete or gut: `pkg/data/db.go` — remove `Init()`, `GetDB()`, `runMigrations()`, sqlite driver import, embed directive
- Delete or gut: `pkg/data/state.go` — remove all function bodies (types already moved)
- Delete or gut: `pkg/data/delete.go`, `pkg/data/sub.go`, `pkg/data/entity.go`, `pkg/data/repo.go`, `pkg/data/org.go`, `pkg/data/developer.go`, `pkg/data/query.go`, `pkg/data/event.go`, `pkg/data/insights.go`, `pkg/data/release.go`, `pkg/data/container.go`, `pkg/data/repo_meta.go`, `pkg/data/metric_history.go`, `pkg/data/reputation.go`, `pkg/data/cncf.go`, `pkg/data/gh.go`, `pkg/data/ratelimit.go`
- Delete: `pkg/data/sql/migrations/` directory (now lives in sqlite package)

After cleanup, `pkg/data/` should contain only:
- `store.go` — interfaces
- `types.go` — shared types
- `helpers.go` — shared utilities (`sinceDate`, `Contains`, `ErrDBNotInitialized`, constants)

**Step 1: Remove files**

Delete all files listed above that are now empty of meaningful code. For `db.go`, if `DataFileName` was moved to `helpers.go`, delete `db.go` entirely.

**Step 2: Build and fix**

Run: `go build ./...`
Fix any remaining import issues.

**Step 3: Commit**

```bash
git add -A
git commit -S -m "refactor: remove old implementation files from pkg/data"
```

---

### Task 18: Move Tests to SQLite Package

**Files:**
- Move: `pkg/data/*_test.go` → `pkg/data/sqlite/*_test.go`
- Keep: Tests that don't use DB (pure function tests) can stay in `pkg/data/` if they test helpers/types

**Step 1: Create sqlite/db_test.go with setupTestDB**

```go
package sqlite

import (
    "testing"

    "github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *Store {
    t.Helper()
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")
    store, err := New(dbPath)
    require.NoError(t, err)
    t.Cleanup(func() { store.Close() })
    return store
}
```

**Step 2: Move and adapt DB-dependent test files**

For each test file that uses `setupTestDB`:
1. Change package to `sqlite`
2. Change `setupTestDB(t)` to return `*Store` (already done in Step 1)
3. Change `data.Foo(db, ...)` calls to `store.Foo(...)`
4. Change `*sql.DB` type references to `*Store`
5. Update imports: `data.` type references become `data.` (unchanged, types still in `pkg/data`)

Test files to move: `db_test.go`, `state_test.go`, `delete_test.go`, `sub_test.go`, `entity_test.go`, `repo_test.go`, `org_test.go`, `developer_test.go`, `query_test.go`, `insights_test.go`, `release_test.go`, `container_test.go`, `repo_meta_test.go`, `metric_history_test.go`, `reputation_test.go`

Also move `seedTestData` helper (currently in `entity_test.go`):
```go
func seedTestData(t *testing.T, store *Store) {
    // same logic but use store.db instead of db parameter
}
```

**Step 3: Keep non-DB tests in pkg/data/**

These test pure functions and don't need the store: `event_test.go` (if it only tests mapping), `gh_test.go`, `ratelimit_test.go`, `cncf_test.go` (if it only tests parsing).

If any of these reference functions that moved to `sqlite/`, move them too.

**Step 4: Update CLI tests**

- `pkg/cli/app_test.go` — update `TestMain` to use `sqlite.New()` instead of `data.Init()` + `data.GetDB()`
- `pkg/cli/server_test.go` — update if it references `*sql.DB` anywhere

**Step 5: Commit**

```bash
git add -A
git commit -S -m "refactor: move tests to sqlite package"
```

---

## Phase 5: Verify

### Task 19: Full Qualification

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No lint errors. Fix any unused imports, missing error checks.

**Step 3: Run full qualification**

Run: `make qualify`
Expected: All pass (test + lint + govulncheck)

**Step 4: Commit any fixes**

```bash
git add -A
git commit -S -m "fix: resolve lint and test issues after refactoring"
```

---

### Task 20: Verify Interface Compliance

**Step 1: Add compile-time interface check**

Add to `pkg/data/sqlite/sqlite.go`:

```go
// Compile-time check that Store implements data.Store.
var _ data.Store = (*Store)(nil)
```

**Step 2: Build**

Run: `go build ./...`
Expected: If this compiles, the SQLite Store implements every method in the interface.

**Step 3: Commit**

```bash
git add pkg/data/sqlite/sqlite.go
git commit -S -m "feat: add compile-time Store interface compliance check"
```

---

### Task 21: Final make qualify

**Step 1: Run**

Run: `make qualify`
Expected: All pass

**Step 2: Review**

Verify final package structure:
```
pkg/data/
  store.go       — interfaces
  types.go       — shared types
  helpers.go     — shared utilities

pkg/data/sqlite/
  sqlite.go      — Store struct, New(), Close(), migrations
  state.go       — StateStore methods
  delete.go      — DeleteStore methods
  sub.go         — SubstitutionStore methods
  entity.go      — EntityStore methods
  repo.go        — RepoStore methods
  org.go         — OrgStore methods
  developer.go   — DeveloperStore methods
  cncf.go        — CNCF affiliation (standalone function taking DeveloperStore)
  query.go       — QueryStore methods
  event.go       — EventStore methods
  insights.go    — InsightsStore methods
  release.go     — ReleaseStore methods
  container.go   — ContainerStore methods
  repo_meta.go   — RepoMetaStore methods
  metric_history.go — MetricHistoryStore methods
  reputation.go  — ReputationStore methods
  gh.go          — GitHub API helpers
  ratelimit.go   — Rate limit handling
  sql/migrations/*.sql
  *_test.go
```

**Step 3: Final commit**

```bash
git add -A
git commit -S -m "feat: complete multi-database store interface refactoring"
```
