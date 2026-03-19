# Optimization Review Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the top 5 optimization issues from the architecture review: HTTP server hardening, resource leaks, PostgreSQL pool config, errgroup concurrency, and HTTP client improvements.

**Architecture:** Five independent workstreams, each touching different files. Tasks 1-2 are low-risk config changes. Task 3 fixes resource leaks. Task 4 replaces manual WaitGroup+semaphore with errgroup. Task 5 improves the HTTP client transport. All changes must be mirrored in both sqlite/ and postgres/ implementations where applicable.

**Tech Stack:** Go 1.26, database/sql, net/http, golang.org/x/sync/errgroup

---

### Task 1: Harden HTTP Server Configuration

**Files:**
- Modify: `pkg/cli/server.go:23-28` (constants), `pkg/cli/server.go:113-119` (server config)
- Modify: `pkg/cli/data.go:219-222` (POST handler)
- Test: `pkg/cli/server_test.go` (if exists), manual verification

**Step 1: Update server timeout constants**

In `pkg/cli/server.go`, replace the constants block (lines 23-28):

```go
const (
	serverShutdownWaitSeconds = 5
	serverReadTimeout         = 30 * time.Second
	serverReadHeaderTimeout   = 5 * time.Second
	serverWriteTimeout        = 60 * time.Second
	serverIdleTimeout         = 120 * time.Second
	serverMaxHeaderBytes      = 20
	serverPortDefault         = 8080
	maxRequestBodyBytes       = 1 << 20 // 1 MB
)
```

**Step 2: Update http.Server struct**

In `pkg/cli/server.go`, replace lines 113-119:

```go
s := &http.Server{
	Addr:              address,
	Handler:           handler,
	ReadTimeout:       serverReadTimeout,
	ReadHeaderTimeout: serverReadHeaderTimeout,
	WriteTimeout:      serverWriteTimeout,
	IdleTimeout:       serverIdleTimeout,
	MaxHeaderBytes:    1 << serverMaxHeaderBytes,
}
```

**Step 3: Add MaxBytesReader to POST handler**

In `pkg/cli/data.go`, update `eventSearchAPIHandler` (line 220-222):

```go
func eventSearchAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		var q data.EventSearchCriteria
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
```

Note: `maxRequestBodyBytes` is in `server.go`. Either move it to `data.go` or export it. Simplest: define it locally in `data.go`.

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 5: Run qualify**

Run: `make qualify`
Expected: All lint/test/vuln checks pass.

**Step 6: Commit**

```bash
git add pkg/cli/server.go pkg/cli/data.go
git commit -S -m "fix: harden HTTP server timeouts and add request body limit"
```

---

### Task 2: Configure PostgreSQL Connection Pool

**Files:**
- Modify: `pkg/data/postgres/postgres.go:28-48` (New function)

**Step 1: Add pool configuration constants**

Add constants at the top of `pkg/data/postgres/postgres.go`:

```go
const (
	maxOpenConns    = 25
	maxIdleConns    = 10
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 1 * time.Minute
)
```

Add `"time"` to the import block.

**Step 2: Configure pool after sql.Open**

In `pkg/data/postgres/postgres.go`, after `sql.Open` (line 33) and before `db.Ping()` (line 38):

```go
db, err := sql.Open("postgres", dsn)
if err != nil {
	return nil, fmt.Errorf("opening database: %w", err)
}

db.SetMaxOpenConns(maxOpenConns)
db.SetMaxIdleConns(maxIdleConns)
db.SetConnMaxLifetime(connMaxLifetime)
db.SetConnMaxIdleTime(connMaxIdleTime)

if err := db.Ping(); err != nil {
```

**Step 3: Run tests**

Run: `make test`
Expected: All tests pass (postgres tests use testcontainers, pool config is transparent).

**Step 4: Run qualify**

Run: `make qualify`
Expected: Pass.

**Step 5: Commit**

```bash
git add pkg/data/postgres/postgres.go
git commit -S -m "fix: configure PostgreSQL connection pool limits"
```

---

### Task 3: Fix Resource Leaks

**Files:**
- Modify: `pkg/data/sqlite/release.go:106-151` (ImportReleases)
- Modify: `pkg/data/postgres/release.go:113-158` (ImportReleases)
- Modify: `pkg/data/sqlite/release.go:153-198` (upsertReleasePage)
- Modify: `pkg/data/postgres/release.go:160-212` (upsertReleasePage)
- Modify: `pkg/data/sqlite/container.go:105-140` (upsertContainerVersions)
- Modify: `pkg/data/postgres/container.go:108-143` (upsertContainerVersions)

**Step 1: Fix prepared statement leak in sqlite/release.go**

In `pkg/data/sqlite/release.go`, add `defer stmt.Close()` and `defer assetStmt.Close()` after each Prepare call (after lines 117 and 122):

```go
stmt, err := s.db.Prepare(insertReleaseSQL)
if err != nil {
	return fmt.Errorf("error preparing release insert: %w", err)
}
defer stmt.Close()

assetStmt, err := s.db.Prepare(insertReleaseAssetSQL)
if err != nil {
	return fmt.Errorf("error preparing release asset insert: %w", err)
}
defer assetStmt.Close()
```

**Step 2: Fix prepared statement leak in postgres/release.go**

Same change in `pkg/data/postgres/release.go` after lines 124 and 129.

**Step 3: Fix tx.Stmt() in loop — sqlite/release.go upsertReleasePage**

In `pkg/data/sqlite/release.go` function `upsertReleasePage` (line 153), create tx-bound statements once before the loop:

```go
func upsertReleasePage(db *sql.DB, stmt, assetStmt *sql.Stmt, owner, repo string, releases []*github.RepositoryRelease, latestPublishedAt string) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("error starting release tx: %w", err)
	}

	txStmt := tx.Stmt(stmt)
	txAssetStmt := tx.Stmt(assetStmt)

	seenOld := false
	for _, r := range releases {
		// ... unchanged ...

		if _, execErr := txStmt.Exec(
			owner, repo, tag, name, publishedAt, pre,
			name, publishedAt, pre,
		); execErr != nil {
			rollbackTransaction(tx)
			return false, fmt.Errorf("error inserting release %s: %w", tag, execErr)
		}

		for _, a := range r.Assets {
			aName := a.GetName()
			if aName == "" {
				continue
			}
			if _, execErr := txAssetStmt.Exec(
				// ... same params ...
			); execErr != nil {
				rollbackTransaction(tx)
				return false, fmt.Errorf("error inserting release asset %s/%s: %w", tag, aName, execErr)
			}
		}
	}
```

**Step 4: Same tx.Stmt fix in postgres/release.go upsertReleasePage**

Mirror the same change in `pkg/data/postgres/release.go` function `upsertReleasePage` (line 160).

**Step 5: Fix db.QueryRow outside tx — sqlite/container.go**

In `pkg/data/sqlite/container.go` function `upsertContainerVersions` (line 122), change `db.QueryRow` to `tx.QueryRow`:

```go
var latestCreatedAt string
if err := tx.QueryRow(selectLatestContainerVersionSQL, org, repo, pkgName).Scan(&latestCreatedAt); err != nil {
```

**Step 6: Fix db.QueryRow outside tx — postgres/container.go**

Same change in `pkg/data/postgres/container.go` (line 125): `db.QueryRow` -> `tx.QueryRow`.

**Step 7: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 8: Run qualify**

Run: `make qualify`
Expected: Pass.

**Step 9: Commit**

```bash
git add pkg/data/sqlite/release.go pkg/data/postgres/release.go pkg/data/sqlite/container.go pkg/data/postgres/container.go
git commit -S -m "fix: close prepared statements and use tx-scoped queries"
```

---

### Task 4: Replace WaitGroup+Semaphore With errgroup

**Files:**
- Modify: `pkg/data/sqlite/event.go:77-119` (UpdateEvents)
- Modify: `pkg/data/postgres/event.go:81-123` (UpdateEvents)
- Modify: `go.mod` (add golang.org/x/sync dependency)

**Step 1: Add errgroup dependency**

Run: `go get golang.org/x/sync`

This adds `golang.org/x/sync` to `go.mod` as a direct dependency.

**Step 2: Rewrite UpdateEvents in sqlite/event.go**

Replace the WaitGroup+semaphore pattern in `pkg/data/sqlite/event.go` lines 77-119:

```go
func (s *Store) UpdateEvents(ctx context.Context, token string, concurrency int) (map[string]int, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	if concurrency < 1 {
		concurrency = 1
	}

	list, err := s.GetAllOrgRepos()
	if err != nil {
		return nil, fmt.Errorf("error getting org/repo list: %w", err)
	}

	results := make(map[string]int)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for _, r := range list {
		org, repo := r.Org, r.Repo
		g.Go(func() error {
			m, _, importErr := s.ImportEvents(ctx, token, org, repo, data.EventAgeMonthsDefault)
			if importErr != nil {
				slog.Error("error importing events", "org", org, "repo", repo, "error", importErr)
				return nil // log and continue, don't abort other repos
			}

			mu.Lock()
			for k, v := range m {
				results[k] += v
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, fmt.Errorf("error during event update: %w", err)
	}

	return results, nil
}
```

Add `"golang.org/x/sync/errgroup"` to the import block. Remove `"sync"` only if no other usage in the file (it IS used elsewhere — keep it).

**Step 3: Rewrite UpdateEvents in postgres/event.go**

Mirror the exact same change in `pkg/data/postgres/event.go` lines 81-123. Same import addition.

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 5: Run qualify**

Run: `make qualify`
Expected: Pass.

**Step 6: Commit**

```bash
git add go.mod go.sum pkg/data/sqlite/event.go pkg/data/postgres/event.go
git commit -S -m "fix: replace WaitGroup+semaphore with errgroup in UpdateEvents"
```

---

### Task 5: Improve HTTP Client Transport

**Files:**
- Modify: `pkg/net/download.go:13-27` (transport config)

**Step 1: Enable compression and tune connection pool**

In `pkg/net/download.go`, update the constants and transport config (lines 13-27):

```go
const (
	maxIdleConns        = 50
	maxIdleConnsPerHost = 20
	timeoutInSeconds    = 60
	clientAgent         = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.88 Safari/537.36"
)

var (
	reqTransport = &http.Transport{
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       timeoutInSeconds * time.Second,
		DisableCompression:    false,
		DisableKeepAlives:     false,
		ResponseHeaderTimeout: time.Duration(timeoutInSeconds) * time.Second,
	}
)
```

Key changes:
- `MaxIdleConns`: 10 -> 50 (support concurrent imports)
- `MaxIdleConnsPerHost`: added at 20 (was defaulting to 2)
- `DisableCompression`: true -> false (enable gzip for GitHub API JSON)

**Step 2: Run tests**

Run: `make test`
Expected: All tests pass.

**Step 3: Run qualify**

Run: `make qualify`
Expected: Pass.

**Step 4: Commit**

```bash
git add pkg/net/download.go
git commit -S -m "fix: enable HTTP compression and tune connection pool"
```

---

## Verification Checklist

After all tasks are complete:

1. `make test` — all unit tests pass with race detector
2. `make lint` — no lint issues
3. `make qualify` — full quality gate passes
4. `make build` — binary builds successfully
5. `make server` — dashboard starts and serves correctly

## Unresolved Questions

1. **Context propagation to Store read methods (C1):** The biggest architectural change (adding `context.Context` to ~40 Store interface methods) is deliberately excluded from this plan. It would touch every handler, every interface method, and both implementations — a separate plan/branch is warranted.
2. **errgroup for ImportEvents inner fan-out (M11):** The 5-importer fan-out inside `ImportEvents` (lines 171-191) also uses WaitGroup. Converting it to errgroup with context cancellation would short-circuit on fatal errors (e.g., revoked token). Deferred to avoid coupling with Task 4.
3. **CheckRateLimit context awareness (M6):** `ghutil.CheckRateLimit` ignores context and can sleep for up to an hour. Adding a `ctx` parameter requires changing every call site across both implementations. Deferred.
