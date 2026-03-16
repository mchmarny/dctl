# PostgreSQL Backend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `data.Store` for PostgreSQL using `github.com/lib/pq`, enabling devpulse to store data in PostgreSQL instead of (or alongside) SQLite.

**Architecture:** Mirror the SQLite Store implementation file-by-file in `pkg/data/postgres/`, translating SQL dialect (? → $N placeholders, IFNULL → COALESCE, etc.). Extract shared GitHub API helpers into `pkg/data/github/` to avoid duplication. Wire `openStore()` to route `postgres://` URIs to the new backend.

**Tech Stack:** Go 1.26, `database/sql`, `github.com/lib/pq`, PostgreSQL (AlloyDB)

---

## Phase 1: Dependencies and Shared Code Extraction

### Task 1: Add lib/pq Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add dependency**

```bash
go get github.com/lib/pq
go mod vendor
```

**Step 2: Verify build**

Run: `go build ./...`

**Step 3: Commit**

```bash
git add go.mod go.sum vendor/
git commit -S -m "feat: add github.com/lib/pq postgres driver dependency"
```

---

### Task 2: Extract Shared GitHub Helpers to pkg/data/github/

**Files:**
- Create: `pkg/data/github/gh.go`
- Create: `pkg/data/github/ratelimit.go`
- Modify: `pkg/data/sqlite/event.go` — update imports
- Modify: `pkg/data/sqlite/developer.go` — update imports
- Modify: `pkg/data/sqlite/cncf.go` — update imports
- Modify: `pkg/data/sqlite/gh.go` — remove functions that moved, keep only sqlite-specific code or delete entirely
- Modify: `pkg/data/sqlite/ratelimit.go` — remove, replaced by shared package
- Modify: `pkg/data/sqlite/gh_test.go` — move to github package or update imports
- Modify: `pkg/data/sqlite/ratelimit_test.go` — move to github package or update imports
- Modify: `pkg/cli/query.go` — update imports from `sqlite.GetUserOrgs` to `github.GetUserOrgs`

**Step 1: Create pkg/data/github/gh.go**

Move these functions from `pkg/data/sqlite/gh.go` (they have ZERO database dependencies):
- `mapUserToDeveloper`
- `mapGitHubUserToDeveloperListItem`
- `deref`, `trim`
- `GetGitHubDeveloper`
- `SearchGitHubUsers`
- `GetUserOrgs`
- `GetOrgRepos`
- `GetOrgRepoNames`
- `parseUsers` (used by event importers)
- `printDevDeltas` (used by MergeDeveloper)
- Any other helper that doesn't touch `*sql.DB` or `*Store`

Package declaration: `package github` (import path `github.com/mchmarny/devpulse/pkg/data/github`)

Note: This package name will shadow `github.com/google/go-github/v83/github`. Use an import alias in the shared package: `gh "github.com/google/go-github/v83/github"`. OR name the package `ghutil` or `ghapi` to avoid the shadow. **Recommended: name it `pkg/data/ghutil/`** to avoid the conflict entirely.

**Step 2: Create pkg/data/ghutil/ratelimit.go**

Move `checkRateLimit` and `abuseRetryAfter` from `pkg/data/sqlite/ratelimit.go`.

**Step 3: Update sqlite package imports**

Replace all calls to the moved functions with `ghutil.FunctionName(...)`. Update imports in:
- `sqlite/event.go` — uses mapUserToDeveloper, parseUsers, checkRateLimit
- `sqlite/developer.go` — uses GetGitHubDeveloper, printDevDeltas
- `sqlite/cncf.go` — uses GetCNCFEntityAffiliations (but this calls DeveloperStore methods, check if it needs ghutil)
- `sqlite/gh.go` — delete if empty after extraction, or keep sqlite-specific wrappers
- `sqlite/ratelimit.go` — delete after extraction

**Step 4: Update pkg/cli/query.go**

Change `sqlite.GetUserOrgs(...)` and `sqlite.GetOrgRepos(...)` to `ghutil.GetUserOrgs(...)` and `ghutil.GetOrgRepos(...)`.

**Step 5: Move test files**

Move `sqlite/gh_test.go` → `ghutil/gh_test.go` and `sqlite/ratelimit_test.go` → `ghutil/ratelimit_test.go`. Update package declarations and imports.

**Step 6: Verify**

Run: `go build ./... && go test -race -count=1 ./...`

**Step 7: Commit**

```bash
git add pkg/data/ghutil/ pkg/data/sqlite/ pkg/cli/query.go go.mod go.sum vendor/
git commit -S -m "refactor: extract shared GitHub helpers to pkg/data/ghutil"
```

---

## Phase 2: Postgres Package Scaffold and Migrations

### Task 3: Create Postgres Store Scaffold

**Files:**
- Create: `pkg/data/postgres/postgres.go`

**Step 1: Create the scaffold**

```go
package postgres

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
	_ "github.com/lib/pq"
)

//go:embed sql/migrations/*.sql
var migrationsFS embed.FS

var _ data.Store = (*Store)(nil)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn not specified")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	var currentVersion int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("sql/migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}

		var ver int
		if _, err := fmt.Sscanf(parts[0], "%d", &ver); err != nil {
			continue
		}

		if ver <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile("sql/migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		slog.Debug("applying migration", "version", ver, "file", name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration tx %d: %w", ver, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES ($1)", ver); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", ver, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", ver, err)
		}

		slog.Info("applied migration", "version", ver, "file", name)
	}

	return nil
}
```

Note: This won't compile yet (Store doesn't implement all interface methods). That's expected.

**Step 2: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: create postgres package scaffold"
```

---

### Task 4: Create Postgres Migration Files

**Files:**
- Create: `pkg/data/postgres/sql/migrations/001_initial_schema.sql`
- Create: `pkg/data/postgres/sql/migrations/002_event_state_columns.sql`
- Create: `pkg/data/postgres/sql/migrations/003_repo_meta_and_release.sql`
- Create: `pkg/data/postgres/sql/migrations/004_reputation.sql`
- Create: `pkg/data/postgres/sql/migrations/005_reputation_deep.sql`
- Create: `pkg/data/postgres/sql/migrations/006_reputation_signals.sql`
- Create: `pkg/data/postgres/sql/migrations/007_event_indexes.sql`
- Create: `pkg/data/postgres/sql/migrations/008_release_asset.sql`
- Create: `pkg/data/postgres/sql/migrations/009_repo_metric_history.sql`
- Create: `pkg/data/postgres/sql/migrations/010_event_title.sql`
- Create: `pkg/data/postgres/sql/migrations/011_container_package.sql`
- Create: `pkg/data/postgres/sql/migrations/012_event_pr_detail.sql`

**Step 1: Translate each migration**

Key translations from SQLite → Postgres:

001: Same (TEXT types, PRIMARY KEY — all valid Postgres)
003: `DEFAULT (datetime('now'))` → `DEFAULT NOW()`, `INTEGER NOT NULL DEFAULT 0` for boolean → keep as INTEGER (or use BOOLEAN, but keep INTEGER for compatibility)
010: `ALTER TABLE event ADD COLUMN title TEXT NOT NULL DEFAULT ''` — same syntax

The migrations are mostly portable. Main change: `datetime('now')` → `NOW()`.

Write each file with the Postgres-compatible DDL. Most are identical to SQLite versions.

**Step 2: Commit**

```bash
git add pkg/data/postgres/sql/
git commit -S -m "feat: add postgres migration files"
```

---

## Phase 3: Implement Store Methods

For each file, the pattern is: copy the sqlite version, then translate SQL.

**SQL translation checklist per file:**
- `?` → `$1, $2, $3...` (numbered sequentially per query)
- `IFNULL(x, y)` → `COALESCE(x, y)` (COALESCE already works in both, but IFNULL is SQLite-only)
- `substr(x, y, z)` → `SUBSTRING(x FROM y FOR z)` or just use standard `SUBSTRING(x, y, z)` which Postgres supports
- `julianday(x) - julianday(y)` → `EXTRACT(EPOCH FROM (x::timestamp - y::timestamp)) / 86400.0`
- `GROUP_CONCAT(x, ',')` → `STRING_AGG(x, ',')`
- `LIKE` for case-insensitive → `ILIKE`

### Task 5: Implement Helpers + State + Delete + Sub

**Files:**
- Create: `pkg/data/postgres/helpers.go`
- Create: `pkg/data/postgres/state.go`
- Create: `pkg/data/postgres/delete.go`
- Create: `pkg/data/postgres/sub.go`

**Step 1: Create helpers.go**

Copy from `sqlite/helpers.go`. Same content — `sinceDate()`, bot-exclude SQL constants, entity maps. The bot-exclude SQL uses `LIKE` and `NOT IN` which work in Postgres too. Keep `LIKE` for bot patterns (exact match with `%[bot]`).

**Step 2: Create state.go**

Copy from `sqlite/state.go`. Translate:
- `insertState`: 7 `?` → `$1..$7`
- `selectState`: 3 `?` → `$1..$3`
- `ClearState` inline SQL: 2 `?` → `$1..$2`
- State queries (SELECT COUNT): no placeholders, no changes needed

**Step 3: Create delete.go**

Copy from `sqlite/delete.go`. Translate:
- Each delete SQL: 2 `?` → `$1, $2`

**Step 4: Create sub.go**

Copy from `sqlite/sub.go`. Translate:
- `insertSubSQL`: 4 `?` → `$1..$4`
- `selectSubSQL`: no placeholders
- `updateDeveloperPropertySQL`: 2 `?` → `$1, $2` (note: this uses `fmt.Sprintf` for column names, then `?` for values)

**Step 5: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: implement postgres state, delete, sub stores"
```

---

### Task 6: Implement Entity + Repo + Org

**Files:**
- Create: `pkg/data/postgres/entity.go`
- Create: `pkg/data/postgres/repo.go`
- Create: `pkg/data/postgres/org.go`

**Step 1: Create entity.go**

Copy from sqlite, translate placeholders. `LIKE` in entity queries → keep `LIKE` (it's for pattern matching with `%`).

**Step 2: Create repo.go**

Copy from sqlite, translate `?` → `$N`.

**Step 3: Create org.go**

Copy from sqlite. This has `IFNULL` usage and `GROUP_CONCAT` — translate to `COALESCE` and `STRING_AGG`. Also has `interface{}` args for dynamic query building — review carefully.

**Step 4: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: implement postgres entity, repo, org stores"
```

---

### Task 7: Implement Developer + CNCF

**Files:**
- Create: `pkg/data/postgres/developer.go`
- Create: `pkg/data/postgres/cncf.go`

**Step 1: Create developer.go**

Copy from sqlite. The `insertDeveloperSQL` has complex COALESCE/CASE logic — translate all `?` to `$N`. The `MergeDeveloper` method calls `ghutil.GetGitHubDeveloper` — import from shared package.

**Step 2: Create cncf.go**

Copy from sqlite. `UpdateDevelopersWithCNCFEntityAffiliations` is a standalone function that takes `data.DeveloperStore` and `data.EntityStore` — same signature, no SQL translation needed (it calls store methods). `GetCNCFEntityAffiliations` and parsing helpers have no SQL at all.

**Step 3: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: implement postgres developer and cncf stores"
```

---

### Task 8: Implement Query + Event

**Files:**
- Create: `pkg/data/postgres/query.go`
- Create: `pkg/data/postgres/event.go`

**Step 1: Create query.go**

Copy from sqlite. Translate placeholders. The `GetEventTypeSeries` query uses `COALESCE` (already Postgres-compatible) and `?` placeholders.

**Step 2: Create event.go**

Copy from sqlite. This is the largest and most complex file (869 lines). Key translations:
- `insertEventSQL`: ~18 `?` → `$1..$18` in INSERT, plus `$19..$N` in ON CONFLICT SET clause. **Count carefully.**
- `selectPRsMissingSizeSQL`: 2 `?` → `$1, $2`
- `updatePRSizeSQL`: 7 `?` → `$1..$7`
- The `eventImporter` struct holds `*Store` — change to postgres `*Store`
- GitHub API calls use `ghutil.*` — import from shared package
- `flush()` method builds prepared statements — translate all `?` in the batch insert

**Step 3: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: implement postgres query and event stores"
```

---

### Task 9: Implement Insights

**Files:**
- Create: `pkg/data/postgres/insights.go`

**Step 1: Create insights.go**

Copy from sqlite (912 lines, 17 SQL constants). This is the heaviest translation:
- Many queries use `IFNULL` → `COALESCE`
- `substr(e.date, 1, 7)` → `SUBSTRING(e.date, 1, 7)` (Postgres supports this form)
- `julianday()` date arithmetic → `EXTRACT(EPOCH FROM ...)` or cast and subtract: `(closed_at::timestamp - created_at::timestamp)` to get an interval, then `EXTRACT(EPOCH FROM interval) / 86400.0` for days
- `GROUP_CONCAT` → `STRING_AGG`
- All `?` → `$N`

**Critical:** The julianday conversions in velocity/latency queries are the trickiest. Example:

SQLite: `julianday(e.merged_at) - julianday(e.created_at)`
Postgres: `EXTRACT(EPOCH FROM (e.merged_at::timestamp - e.created_at::timestamp)) / 86400.0`

**Step 2: Commit**

```bash
git add pkg/data/postgres/insights.go
git commit -S -m "feat: implement postgres insights store"
```

---

### Task 10: Implement Release + Container + RepoMeta + MetricHistory

**Files:**
- Create: `pkg/data/postgres/release.go`
- Create: `pkg/data/postgres/container.go`
- Create: `pkg/data/postgres/repo_meta.go`
- Create: `pkg/data/postgres/metric_history.go`

**Step 1: Create all four files**

Copy from sqlite, translate placeholders. These are mostly straightforward INSERT/SELECT with `?` → `$N`. Import functions use `ghutil.*` for GitHub API calls.

`release.go` has `julianday()` in one query for cadence calculation — translate like insights.

**Step 2: Commit**

```bash
git add pkg/data/postgres/
git commit -S -m "feat: implement postgres release, container, repo_meta, metric_history stores"
```

---

### Task 11: Implement Reputation

**Files:**
- Create: `pkg/data/postgres/reputation.go`

**Step 1: Create reputation.go**

Copy from sqlite (613 lines, 10 SQL constants). Translate:
- All `?` → `$N`
- `IFNULL` → `COALESCE`
- `julianday('now') - julianday(...)` → `EXTRACT(EPOCH FROM (NOW() - ...::timestamp)) / 86400.0`
- `GROUP_CONCAT` → `STRING_AGG`
- Uses `ghutil.*` for deep reputation (GitHub API calls)

**Step 2: Commit**

```bash
git add pkg/data/postgres/reputation.go
git commit -S -m "feat: implement postgres reputation store"
```

---

## Phase 4: Wire CLI and Verify

### Task 12: Wire openStore() for Postgres

**Files:**
- Modify: `pkg/cli/app.go`

**Step 1: Update openStore**

```go
import (
    "github.com/mchmarny/devpulse/pkg/data/postgres"
    "github.com/mchmarny/devpulse/pkg/data/sqlite"
)

func openStore(dsn string) (data.Store, error) {
    if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
        return postgres.New(dsn)
    }
    return sqlite.New(dsn)
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Compiles (postgres.Store implements data.Store)

**Step 3: Commit**

```bash
git add pkg/cli/app.go
git commit -S -m "feat: wire postgres backend in openStore"
```

---

### Task 13: Run make qualify

**Step 1: Run full qualification**

Run: `make qualify`
Expected: All pass. Fix any lint issues.

**Step 2: Test with SQLite (regression)**

Run: `dist/devpulse_darwin_arm64_v8.0/devpulse query events --limit 1`
Expected: Still works with default SQLite DB.

**Step 3: Test with Postgres**

Requires AlloyDB proxy running:
```bash
make build
dist/devpulse_darwin_arm64_v8.0/devpulse --db "postgres://devpulse:PASSWORD@127.0.0.1:5432/devpulse" imp --org NVIDIA --repo aicr
```

**Step 4: Commit any fixes**

```bash
git add -A
git commit -S -m "fix: resolve lint and test issues for postgres backend"
```

---

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 (Tasks 1-2) | Dependencies + extract shared code | Add lib/pq, extract ghutil package |
| 2 (Tasks 3-4) | Postgres scaffold + migrations | Store struct, New(), Postgres DDL |
| 3 (Tasks 5-11) | Implement all Store methods | Translate SQL dialect per file |
| 4 (Tasks 12-13) | Wire CLI + verify | openStore routing, make qualify, live test |
