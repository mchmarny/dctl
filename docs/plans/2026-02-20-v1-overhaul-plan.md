# dctl v1.0.0 Overhaul Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Modernize dctl from v0.4.0 to v1.0.0 — replace deprecated deps with stdlib, upgrade go-github, improve architecture, achieve 60%+ test coverage.

**Architecture:** Four sequential phases: P1 (foundation: errors, logging, build), P2 (deps: go-github v83, remove Gin), P3 (architecture: migrations, keychain, DI, rate limiting, CNCF), P4 (tests: unit, integration, handler). Each phase ends with a passing `make test` and commit.

**Tech Stack:** Go 1.25, log/slog, net/http (Go 1.22+ routing), modernc.org/sqlite, zalando/go-keyring, google/go-github/v83, urfave/cli/v2

---

## Phase 1: Foundation Modernization

### Task 1: Replace pkg/errors with stdlib errors

**Files:**
- Modify: `pkg/data/db.go`
- Modify: `pkg/data/developer.go`
- Modify: `pkg/data/entity.go`
- Modify: `pkg/data/event.go`
- Modify: `pkg/data/gh.go`
- Modify: `pkg/data/org.go`
- Modify: `pkg/data/repo.go`
- Modify: `pkg/data/query.go`
- Modify: `pkg/data/state.go`
- Modify: `pkg/data/sub.go`
- Modify: `pkg/data/cncf.go`
- Modify: `pkg/auth/token.go`
- Modify: `pkg/net/client.go`
- Modify: `pkg/net/download.go`
- Modify: `pkg/net/http.go`
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/auth.go`
- Modify: `cmd/cli/import.go`
- Modify: `cmd/cli/query.go`

**Step 1: Replace all pkg/errors imports and calls**

In every file listed above:
- Remove `"github.com/pkg/errors"` from imports
- Add `"errors"` and/or `"fmt"` to imports as needed
- Replace patterns:
  - `errors.New("msg")` → `errors.New("msg")` (just change import source)
  - `errors.Wrap(err, "msg")` → `fmt.Errorf("msg: %w", err)`
  - `errors.Wrapf(err, "msg: %s", val)` → `fmt.Errorf("msg: %s: %w", val, err)`
  - `errors.Errorf("msg: %s", val)` → `fmt.Errorf("msg: %s", val)`
  - `errors.Is(err, target)` → `errors.Is(err, target)` (already stdlib compatible)

Special case in `pkg/net/download.go`:
- `var ErrorURLNotFound = errors.New("URL not found")` → keep but ensure it uses stdlib `errors`
- Change `var ErrorURLNotFound = errors.New(...)` to use stdlib `errors` package

Special case in `pkg/auth/token.go:76`:
- Replace `ioutil.ReadAll(res.Body)` → `io.ReadAll(res.Body)`
- Replace `"io/ioutil"` import → `"io"`

Special case in `cmd/cli/auth.go:75`:
- Replace `ioutil.ReadFile(tokenPath)` → `os.ReadFile(tokenPath)`
- Remove `"io/ioutil"` import

**Step 2: Run tests to verify**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass, no compilation errors

**Step 3: Remove pkg/errors from go.mod**

Run: `go mod tidy && go mod vendor`
Verify `github.com/pkg/errors` is gone from `go.mod`

**Step 4: Commit**

```
git add -A && git commit -S -m "replace pkg/errors with stdlib errors and fmt.Errorf"
```

---

### Task 2: Replace logrus with log/slog

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/query.go`
- Modify: `cmd/cli/data.go`
- Modify: `cmd/cli/server.go`
- Modify: `pkg/data/db.go`
- Modify: `pkg/data/developer.go`
- Modify: `pkg/data/entity.go`
- Modify: `pkg/data/event.go`
- Modify: `pkg/data/gh.go`
- Modify: `pkg/data/org.go`
- Modify: `pkg/data/cncf.go`
- Modify: `pkg/net/debug.go`

**Step 1: Update cmd/cli/main.go logging setup**

Replace `initLogging` function and remove logrus import. Replace with:

```go
import "log/slog"

func initLogging(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
```

Update `main()`:
- Move `initLogging` call after flag parsing (inside `Before` hook)
- Replace `log.Fatalf(...)` → `slog.Error(...); os.Exit(1)`
- Replace `log.Debugf("msg: %s", val)` → `slog.Debug("msg", "key", val)`

**Step 2: Replace logrus calls in all pkg/ files**

Pattern replacements across all files:
- `log "github.com/sirupsen/logrus"` → `"log/slog"`
- `log.Debug("msg")` → `slog.Debug("msg")`
- `log.Debugf("msg: %s", val)` → `slog.Debug("msg", "key", val)`
- `log.Infof("msg: %s", val)` → `slog.Info("msg", "key", val)`
- `log.Errorf("msg: %s", err)` → `slog.Error("msg", "error", err)`
- `log.WithFields(log.Fields{...}).Debug("msg")` → `slog.Debug("msg", "key1", val1, "key2", val2)`
- `log.Fatalf(...)` → `slog.Error(...); os.Exit(1)` (only in cmd/cli)

Key files with structured logging to convert:
- `cmd/cli/query.go:229-239` — `log.WithFields` → `slog.Debug` with key-value pairs
- `pkg/data/developer.go:140-147` — `log.WithFields` → `slog.Error` with key-value pairs
- `pkg/data/gh.go:93-100` — `log.WithFields` → `slog.Debug` with key-value pairs
- `pkg/data/org.go:206-209` — `log.WithFields` → `slog.Debug` with key-value pairs

**Step 3: Update pkg/net/debug.go**

```go
package net

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
)

func PrintHTTPResponse(resp *http.Response) {
	if resp == nil {
		return
	}
	if respDump, err := httputil.DumpResponse(resp, true); err == nil {
		slog.Debug("http response", "body", string(respDump))
	}
}
```

**Step 4: Run tests**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 5: Remove logrus from go.mod**

Run: `go mod tidy && go mod vendor`
Verify `github.com/sirupsen/logrus` is gone from `go.mod`

**Step 6: Commit**

```
git add -A && git commit -S -m "replace logrus with log/slog"
```

---

### Task 3: Update linter config and build system

**Files:**
- Modify: `.golangci.yaml`
- Modify: `Makefile`
- Modify: `.goreleaser.yml`
- Modify: `.github/workflows/test-on-call.yaml`
- Modify: `version`
- Create: `.versions.yaml`

**Step 1: Update .golangci.yaml**

Replace the entire file with a modernized config:
- Remove deprecated linters: `deadcode`, `varcheck`, `exportloopref`, `golint`, `maligned`, `gomnd`
- Add modern equivalents: `unused`, `mnd` (replaces gomnd), `copyloopvar` (replaces exportloopref)
- Update `run.go` from `"1.18"` to `"1.25"`
- Remove `skip-dirs` (deprecated) → use `issues.exclude-dirs`
- Remove duplicate entries (`errcheck`, `gomnd`, `deadcode` listed twice)

```yaml
linters-settings:
  dupl:
    threshold: 175
  funlen:
    lines: 150
    statements: 100
  goconst:
    min-len: 2
    min-occurrences: 2
  gocyclo:
    min-complexity: 25
  lll:
    line-length: 250
  misspell:
    locale: US
  nolintlint:
    allow-unused: false
    require-explanation: true
    require-specific: true

linters:
  disable-all: true
  enable:
    - unused
    - dogsled
    - dupl
    - errcheck
    - funlen
    - gochecknoinits
    - goconst
    - gocyclo
    - gofmt
    - goimports
    - mnd
    - goprintffuncname
    - gosec
    - govet
    - lll
    - misspell
    - nakedret
    - nolintlint
    - copyloopvar
    - typecheck
    - unconvert
    - whitespace

issues:
  exclude-dirs:
    - tests
    - tools
    - vendor

run:
  concurrency: 4
  timeout: 5m
  issues-exit-code: 5
  tests: true
  go: "1.25"
```

**Step 2: Update .github/workflows/test-on-call.yaml**

- Change `go-version: ^1.18` → `go-version: "1.25"`
- Update `actions/setup-go@v3` → `actions/setup-go@v5`
- Update `actions/cache@v3` → `actions/cache@v4`
- Update `actions/checkout@v3` → `actions/checkout@v4`
- Update `golangci/golangci-lint-action@v3` → `golangci/golangci-lint-action@v6`
- Update `andstor/file-existence-action@v1` → `andstor/file-existence-action@v3`

**Step 3: Update Makefile**

- Change `goreleaser release --snapshot --rm-dist` → `goreleaser release --snapshot --clean`

**Step 4: Create .versions.yaml**

```yaml
go: "1.25.0"
golangci-lint: "1.63.0"
goreleaser: "2.6.0"
```

**Step 5: Update version file**

Change `v0.4.0` → `v1.0.0`

**Step 6: Run tests**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 7: Commit**

```
git add -A && git commit -S -m "update linter config, CI, and build system for v1.0.0"
```

---

## Phase 2: Dependency Upgrades

### Task 4: Upgrade go-github v45 to v83

**Files:**
- Modify: `go.mod`
- Modify: `pkg/data/event.go`
- Modify: `pkg/data/gh.go`
- Modify: `pkg/data/org.go`
- Modify: `pkg/data/repo.go`

**Step 1: Update go.mod**

Run:
```bash
go get github.com/google/go-github/v83@latest
```

**Step 2: Update all import paths**

In every file that imports go-github, change:
```go
"github.com/google/go-github/v45/github"
```
to:
```go
"github.com/google/go-github/v83/github"
```

Files to update: `pkg/data/event.go`, `pkg/data/gh.go`, `pkg/data/org.go`, `pkg/data/repo.go`

**Step 3: Fix API breaking changes in pkg/data/event.go**

Key changes between v45 and v83:
- `github.Timestamp` is now `github.Timestamp` (check if `.Time` accessor changed)
- `items[i].UpdatedAt` on `*github.Repository` for forks: `items[i].UpdatedAt.Time` may need adjustment
- `*github.PullRequestListCommentsOptions` fields may have changed — verify `Sort`, `Direction`, `Since` fields
- `*github.IssueListCommentsOptions` fields: `Sort` and `Direction` changed from `*string` to `string` in newer versions
- Check if `resp.Rate` is still at `resp.Rate` or moved to a different field
- `github.ListOptions` should still be the same

For `importIssueCommentEvents`: The `Sort` and `Direction` fields on `IssueListCommentsOptions` likely changed from `*string` to `string`:
```go
// Old (v45):
opt := &github.IssueListCommentsOptions{
    Sort:      getStrPtr(sortField),
    Direction: getStrPtr(sortCommentField),
    Since:     &e.state[EventTypeIssueComment].Since,
    ...
}

// New (v83) — check actual API and adjust:
opt := &github.IssueListCommentsOptions{
    Sort:      sortField,
    Direction: sortCommentField,
    Since:     e.state[EventTypeIssueComment].Since,
    ...
}
```

For `importForkEvents`: Check `items[i].UpdatedAt` — if it's now `*github.Timestamp` instead of `github.Timestamp`, adjust dereference.

**Step 4: Fix API changes in pkg/data/gh.go, org.go, repo.go**

- `github.NewClient(client)` may need `github.NewClient(client).WithAuthToken(token)` pattern in newer versions, or continue using oauth2 transport
- Check `Search.Users`, `Users.Get`, `Organizations.List`, `Repositories.List` signatures
- `list.Users` in search results: check if it's `[]*github.User` or changed

**Step 5: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 6: Tidy and vendor**

Run: `go mod tidy && go mod vendor`

**Step 7: Commit**

```
git add -A && git commit -S -m "upgrade go-github from v45 to v83"
```

---

### Task 5: Replace Gin with stdlib net/http

**Files:**
- Modify: `cmd/cli/server.go`
- Modify: `cmd/cli/data.go`
- Modify: `cmd/cli/view.go`
- Modify: `go.mod`

**Step 1: Rewrite cmd/cli/server.go**

Replace Gin router with `http.ServeMux` (Go 1.22+ pattern routing):

```go
package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

const (
	serverShutdownWaitSeconds = 5
	serverTimeoutSeconds      = 300
	serverMaxHeaderBytes      = 20
	serverPortDefault         = 8080
)

var (
	//go:embed assets/* templates/*
	embedFS embed.FS

	portFlag = &cli.IntFlag{
		Name:     "port",
		Usage:    "Port on which the server will listen",
		Value:    serverPortDefault,
		Required: false,
	}

	serverCmd = &cli.Command{
		Name:    "server",
		Aliases: []string{"s"},
		Usage:   "Start local HTTP server",
		Action:  cmdStartServer,
		Flags: []cli.Flag{
			portFlag,
		},
	}
)

func cmdStartServer(c *cli.Context) error {
	port := c.Int(portFlag.Name)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	mux := makeRouter()
	s := &http.Server{
		Addr:           address,
		Handler:        mux,
		ReadTimeout:    serverTimeoutSeconds * time.Second,
		WriteTimeout:   serverTimeoutSeconds * time.Second,
		MaxHeaderBytes: 1 << serverMaxHeaderBytes,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server start failed", "error", err)
		}
	}()

	slog.Info("server started", "address", address)

	<-done

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownWaitSeconds*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		slog.Error("server shutdown failed", "error", err)
	}
	return nil
}

func makeRouter() *http.ServeMux {
	tmpl := template.Must(template.New("").ParseFS(embedFS, "templates/*.html"))
	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.FileServerFS(embedFS))
	mux.HandleFunc("GET /favicon.ico", faveIconHandler)

	// Views
	mux.HandleFunc("GET /", homeViewHandlerFunc(tmpl))

	// Data API
	mux.HandleFunc("GET /data/query", queryHandlerFunc)
	mux.HandleFunc("GET /data/type", eventDataHandlerFunc)
	mux.HandleFunc("GET /data/entity", entityDataHandlerFunc)
	mux.HandleFunc("GET /data/developer", developerDataHandlerFunc)
	mux.HandleFunc("POST /data/search", eventSearchHandlerFunc)

	return mux
}
```

**Step 2: Rewrite cmd/cli/view.go**

```go
package main

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mchmarny/dctl/pkg/data"
)

func faveIconHandler(w http.ResponseWriter, r *http.Request) {
	file, err := embedFS.ReadFile("assets/img/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(file)
}

func homeViewHandlerFunc(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := map[string]any{
			"version":       version,
			"commit":        commit,
			"build_date":    date,
			"err":           r.URL.Query().Get("err"),
			"period_months": data.EventAgeMonthsDefault,
		}
		if err := tmpl.ExecuteTemplate(w, "home", d); err != nil {
			slog.Error("template render failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
```

**Step 3: Rewrite cmd/cli/data.go**

Replace all `gin.Context` handlers with `http.HandlerFunc`. Key changes:
- `c.Query("key")` → `r.URL.Query().Get("key")`
- `c.JSON(status, data)` → `w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); json.NewEncoder(w).Encode(data)`
- `c.ShouldBindJSON(&q)` → `json.NewDecoder(r.Body).Decode(&q)`
- `gin.H{"error": "msg"}` → `map[string]string{"error": "msg"}`

Create a helper:

```go
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

Convert each handler (queryHandler, eventDataHandler, entityDataHandler, developerDataHandler, eventSearchHandler) to use `http.HandlerFunc` signature.

Replace `queryAsInt` to use `r.URL.Query().Get(key)` instead of `c.Query(key)`.

**Step 4: Remove embedded FS conflict**

The `cmd/cli/server.go` currently declares `var f embed.FS` which conflicts with the same name in `pkg/data/db.go`. Rename the server one to `embedFS` (already done in Step 1).

**Step 5: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 6: Remove Gin from go.mod**

Run: `go mod tidy && go mod vendor`
Verify `gin-gonic/gin` and its transitive deps are removed.

**Step 7: Commit**

```
git add -A && git commit -S -m "replace Gin with stdlib net/http routing"
```

---

## Phase 3: Architecture Improvements

### Task 6: Add database migration system

**Files:**
- Rename: `pkg/data/sql/ddl.sql` → `pkg/data/sql/migrations/001_initial_schema.sql`
- Create: `pkg/data/sql/migrations/002_schema_version.sql`
- Modify: `pkg/data/db.go`

**Step 1: Create migration directory and files**

Move current `pkg/data/sql/ddl.sql` to `pkg/data/sql/migrations/001_initial_schema.sql` (same content).

Create `pkg/data/sql/migrations/002_schema_version.sql`:
```sql
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Step 2: Rewrite db.go Init with migration system**

```go
package data

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	DataFileName     string = "data.db"
	nonAlphaNumRegex string = "[^a-zA-Z0-9 ]+"
)

var (
	//go:embed sql/migrations/*.sql
	migrationsFS embed.FS

	errDBNotInitialized = errors.New("database not initialized")
	entityRegEx         *regexp.Regexp
)

func Init(dbFilePath string) error {
	if dbFilePath == "" {
		return errors.New("dbFilePath not specified")
	}

	db, err := GetDB(dbFilePath)
	if err != nil {
		return fmt.Errorf("opening database %s: %w", dbFilePath, err)
	}
	defer db.Close()

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enabling WAL mode: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	entityRegEx, err = regexp.Compile(nonAlphaNumRegex)
	if err != nil {
		return fmt.Errorf("compiling entity regex: %w", err)
	}

	return nil
}

func runMigrations(db *sql.DB) error {
	// Ensure schema_version table exists (bootstrap)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	// Read migration files
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

		// Extract version number from filename (e.g., "001_initial_schema.sql" → 1)
		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			continue
		}

		if version <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile("sql/migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		slog.Debug("applying migration", "version", version, "file", name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration tx %d: %w", version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", version, err)
		}

		slog.Info("applied migration", "version", version, "file", name)
	}

	return nil
}

func GetDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", path, err)
	}
	return conn, nil
}
```

**Step 3: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add embedded SQL migration system with WAL mode"
```

---

### Task 7: Integrate OS keychain for token storage

**Files:**
- Modify: `go.mod`
- Modify: `cmd/cli/auth.go`

**Step 1: Add go-keyring dependency**

Run: `go get github.com/zalando/go-keyring@latest`

**Step 2: Rewrite cmd/cli/auth.go**

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/mchmarny/dctl/pkg/auth"
	"github.com/urfave/cli/v2"
	"github.com/zalando/go-keyring"
)

const (
	clientID       = "f1b500ebdf533aa8a3e2"
	tokenFileName  = "github_token"
	keyringService = "dctl"
	keyringUser    = "github_token"
)

var (
	authCmd = &cli.Command{
		Name:    "auth",
		Aliases: []string{"a"},
		Usage:   "Authenticate to GitHub to obtain an access token",
		Action:  cmdInitAuthFlow,
	}
)

func cmdInitAuthFlow(c *cli.Context) error {
	code, err := auth.GetDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("getting device code: %w", err)
	}

	fmt.Printf("1). Copy this code: %s\n", code.UserCode)
	fmt.Printf("2). Navigate to this URL in your browser to authenticate: %s\n", code.VerificationURL)
	fmt.Print("3). Hit enter to complete the process:\n")
	fmt.Print(">")

	if _, err = fmt.Scanln(); err != nil {
		return fmt.Errorf("reading user input: %w", err)
	}

	token, err := auth.GetToken(clientID, code)
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}

	if err = saveGitHubToken(token.AccessToken); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Println("Token saved to OS keychain")
	return nil
}

func saveGitHubToken(token string) error {
	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		slog.Warn("keychain unavailable, falling back to file", "error", err)
		return saveGitHubTokenFile(token)
	}

	// Clean up legacy file if it exists
	legacyPath := path.Join(getHomeDir(), tokenFileName)
	os.Remove(legacyPath)

	return nil
}

func getGitHubToken() (string, error) {
	// Try keychain first
	token, err := keyring.Get(keyringService, keyringUser)
	if err == nil && token != "" {
		return token, nil
	}

	// Fall back to file
	token, err = getGitHubTokenFile()
	if err != nil {
		return "", err
	}

	// Migrate to keychain
	if migrateErr := keyring.Set(keyringService, keyringUser, token); migrateErr == nil {
		slog.Info("migrated token from file to OS keychain")
		legacyPath := path.Join(getHomeDir(), tokenFileName)
		os.Remove(legacyPath)
	}

	return token, nil
}

func saveGitHubTokenFile(token string) error {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

func getGitHubTokenFile() (string, error) {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	b, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("reading token file %s: %w", tokenPath, err)
	}
	return string(b), nil
}
```

**Step 3: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 4: Tidy and vendor**

Run: `go mod tidy && go mod vendor`

**Step 5: Commit**

```
git add -A && git commit -S -m "integrate OS keychain for token storage with file fallback"
```

---

### Task 8: Dependency injection and connection management

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/import.go`
- Modify: `cmd/cli/query.go`
- Modify: `cmd/cli/data.go`
- Modify: `cmd/cli/server.go`
- Modify: `cmd/cli/view.go`

**Step 1: Create app config struct in main.go**

Replace global vars `dbFilePath`, `debug` with a config struct stored in CLI context:

```go
type appConfig struct {
	DBPath string
	Debug  bool
	DB     *sql.DB
}

const appConfigKey = "app-config"

func getConfig(c *cli.Context) *appConfig {
	return c.App.Metadata[appConfigKey].(*appConfig)
}
```

Update `main()` to:
- Create config in `Before` hook
- Open single DB connection and store in config
- Close DB in `After` hook
- Pass config through CLI metadata

**Step 2: Replace getDBOrFail() calls**

In every command handler and data handler, replace:
```go
db := getDBOrFail()
defer db.Close()
```
with:
```go
cfg := getConfig(c)
db := cfg.DB
```

For HTTP handlers that don't have CLI context, pass the `*sql.DB` via closure:
```go
mux.HandleFunc("GET /data/query", queryHandlerFunc(db))
```

**Step 3: Update import.go to use context from CLI**

Replace `context.Background()` with `c.Context` in all command handlers.

**Step 4: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 5: Commit**

```
git add -A && git commit -S -m "replace globals with config struct and single DB connection"
```

---

### Task 9: Add GitHub API rate limiting

**Files:**
- Create: `pkg/data/ratelimit.go`
- Modify: `pkg/data/event.go`

**Step 1: Create pkg/data/ratelimit.go**

```go
package data

import (
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/go-github/v83/github"
)

const rateLimitThreshold = 10

func checkRateLimit(resp *github.Response) {
	if resp == nil {
		return
	}

	if resp.Rate.Remaining > rateLimitThreshold {
		return
	}

	resetAt := resp.Rate.Reset.Time
	wait := time.Until(resetAt)
	if wait <= 0 {
		return
	}

	// Add jitter: 0-2s random
	jitter := time.Duration(rand.IntN(2000)) * time.Millisecond
	total := wait + jitter

	slog.Info("rate limit approaching, waiting",
		"remaining", resp.Rate.Remaining,
		"reset_at", resetAt.Format(time.RFC3339),
		"wait", total.String(),
	)

	time.Sleep(total)
}
```

**Step 2: Add checkRateLimit calls in event.go**

In each import method (`importPREvents`, `importIssueEvents`, `importIssueCommentEvents`, `importPRReviewEvents`, `importForkEvents`), add after each API call:

```go
items, resp, err := e.client.PullRequests.List(ctx, e.owner, e.repo, opt)
if err != nil {
    ...
}
checkRateLimit(resp)
```

**Step 3: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add GitHub API rate limit detection with jitter backoff"
```

---

### Task 10: Update CNCF affiliation fetcher

**Files:**
- Modify: `pkg/data/cncf.go`

**Step 1: Replace hardcoded file count with dynamic discovery**

Replace `numOfAffilFilesToDownload = 100` loop with sequential fetch until 404:

```go
func GetCNCFEntityAffiliations(ctx context.Context) (map[string]*CNCFDeveloper, error) {
	start := time.Now()
	devs := make(map[string]*CNCFDeveloper)
	completed := 0

	for i := 1; ; i++ {
		select {
		case <-ctx.Done():
			return devs, ctx.Err()
		default:
		}

		url := fmt.Sprintf(affilFileURL, i)
		ok, err := loadAffiliations(url, devs)
		if err != nil {
			return devs, fmt.Errorf("loading affiliation file %d (%s): %w", i, url, err)
		}
		if !ok {
			break
		}
		completed++
	}

	slog.Debug("CNCF affiliations loaded",
		"files", completed,
		"developers", len(devs),
		"duration", time.Since(start).String(),
	)

	return devs, nil
}
```

Remove `numOfAffilFilesToDownload` constant.

**Step 2: Add context parameter to UpdateDevelopersWithCNCFEntityAffiliations**

Update callers in `cmd/cli/import.go` to pass context through.

**Step 3: Build and test**

Run: `go build ./... && go test -count=1 -race ./...`
Expected: All tests pass

**Step 4: Commit**

```
git add -A && git commit -S -m "update CNCF fetcher with dynamic discovery and context support"
```

---

## Phase 4: Test Coverage

### Task 11: Add database integration tests

**Files:**
- Create: `pkg/data/db_test.go`

**Step 1: Write db_test.go**

```go
package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := Init(dbPath)
	require.NoError(t, err)

	db, err := GetDB(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })
	return db
}

func TestInit_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := Init(dbPath)
	require.NoError(t, err)

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestInit_EmptyPath(t *testing.T) {
	err := Init("")
	assert.Error(t, err)
}

func TestInit_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := Init(dbPath)
	require.NoError(t, err)

	db, err := GetDB(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify schema_version table exists and has entries
	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	assert.NoError(t, err)
	assert.Greater(t, version, 0)

	// Verify developer table exists
	_, err = db.Exec("SELECT count(*) FROM developer")
	assert.NoError(t, err)

	// Verify event table exists
	_, err = db.Exec("SELECT count(*) FROM event")
	assert.NoError(t, err)
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	err := Init(dbPath)
	require.NoError(t, err)

	// Running again should not fail
	err = Init(dbPath)
	assert.NoError(t, err)
}

func TestContains(t *testing.T) {
	assert.True(t, Contains([]string{"a", "b", "c"}, "b"))
	assert.False(t, Contains([]string{"a", "b", "c"}, "d"))
	assert.False(t, Contains[string](nil, "a"))
}
```

**Step 2: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 3: Commit**

```
git add -A && git commit -S -m "add database integration tests"
```

---

### Task 12: Add state and developer data tests

**Files:**
- Create: `pkg/data/state_test.go`
- Modify: `pkg/data/developer_test.go` (add more tests)

**Step 1: Write state_test.go**

```go
package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetState_NoExistingState(t *testing.T) {
	db := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()

	state, err := GetState(db, "pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Page)
	assert.Equal(t, min.Unix(), state.Since.Unix())
}

func TestSaveAndGetState(t *testing.T) {
	db := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()
	since := time.Now().AddDate(0, -3, 0).UTC()

	s := &State{Page: 5, Since: since}
	err := SaveState(db, "pr", "testorg", "testrepo", s)
	require.NoError(t, err)

	got, err := GetState(db, "pr", "testorg", "testrepo", min)
	require.NoError(t, err)
	assert.Equal(t, 5, got.Page)
	assert.Equal(t, since.Unix(), got.Since.Unix())
}

func TestSaveState_Upsert(t *testing.T) {
	db := setupTestDB(t)
	min := time.Now().AddDate(0, -6, 0).UTC()

	s1 := &State{Page: 3, Since: min}
	err := SaveState(db, "pr", "org", "repo", s1)
	require.NoError(t, err)

	s2 := &State{Page: 7, Since: min}
	err = SaveState(db, "pr", "org", "repo", s2)
	require.NoError(t, err)

	got, err := GetState(db, "pr", "org", "repo", min)
	require.NoError(t, err)
	assert.Equal(t, 7, got.Page)
}

func TestSaveState_NilState(t *testing.T) {
	db := setupTestDB(t)
	err := SaveState(db, "pr", "org", "repo", nil)
	assert.Error(t, err)
}

func TestSaveState_EmptyParams(t *testing.T) {
	db := setupTestDB(t)
	s := &State{Page: 1, Since: time.Now()}
	err := SaveState(db, "", "org", "repo", s)
	assert.Error(t, err)
}
```

**Step 2: Expand developer_test.go**

Add tests for SaveDevelopers, GetDeveloper, SearchDevelopers:

```go
func TestSaveAndGetDeveloper(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "testuser", FullName: "Test User", Email: "test@example.com", Entity: "TESTCORP"},
	}

	err := SaveDevelopers(db, devs)
	require.NoError(t, err)

	got, err := GetDeveloper(db, "testuser")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "testuser", got.Username)
	assert.Equal(t, "Test User", got.FullName)
	assert.Equal(t, "TESTCORP", got.Entity)
}

func TestGetDeveloper_NotFound(t *testing.T) {
	db := setupTestDB(t)
	got, err := GetDeveloper(db, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestSearchDevelopers(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "alice", FullName: "Alice Smith", Email: "alice@corp.com", Entity: "CORP"},
		{Username: "bob", FullName: "Bob Jones", Email: "bob@other.com", Entity: "OTHER"},
	}
	err := SaveDevelopers(db, devs)
	require.NoError(t, err)

	results, err := SearchDevelopers(db, "alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "alice", results[0].Username)
}

func TestSaveDevelopers_Upsert(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "user1", FullName: "Original", Entity: "CORP"},
	}
	err := SaveDevelopers(db, devs)
	require.NoError(t, err)

	devs[0].FullName = "Updated"
	err = SaveDevelopers(db, devs)
	require.NoError(t, err)

	got, err := GetDeveloper(db, "user1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.FullName)
}

func TestSaveDevelopers_EmptySlice(t *testing.T) {
	db := setupTestDB(t)
	err := SaveDevelopers(db, []*Developer{})
	assert.NoError(t, err)
}
```

**Step 3: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add state and developer data tests"
```

---

### Task 13: Add entity and event query tests

**Files:**
- Create: `pkg/data/entity_test.go`
- Create: `pkg/data/query_test.go`

**Step 1: Write entity_test.go**

```go
package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	devs := []*Developer{
		{Username: "dev1", FullName: "Dev One", Email: "dev1@google.com", Entity: "GOOGLE"},
		{Username: "dev2", FullName: "Dev Two", Email: "dev2@google.com", Entity: "GOOGLE"},
		{Username: "dev3", FullName: "Dev Three", Email: "dev3@msft.com", Entity: "MICROSOFT"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	// Insert events directly
	tx, err := db.Begin()
	require.NoError(t, err)
	stmt, err := db.Prepare(insertEventSQL)
	require.NoError(t, err)

	events := []Event{
		{Org: "testorg", Repo: "testrepo", Username: "dev1", Type: EventTypePR, Date: "2026-01-15", URL: "https://github.com/pr/1", Mentions: "", Labels: ""},
		{Org: "testorg", Repo: "testrepo", Username: "dev2", Type: EventTypeIssue, Date: "2026-01-16", URL: "https://github.com/issue/1", Mentions: "dev1", Labels: "bug"},
		{Org: "testorg", Repo: "testrepo", Username: "dev3", Type: EventTypePR, Date: "2026-01-17", URL: "https://github.com/pr/2", Mentions: "", Labels: ""},
	}

	for _, e := range events {
		_, err = tx.Stmt(stmt).Exec(e.Org, e.Repo, e.Username, e.Type, e.Date,
			e.URL, e.Mentions, e.Labels, e.URL, e.Mentions, e.Labels)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func TestQueryEntities(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	results, err := QueryEntities(db, "GOOGLE", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "GOOGLE", results[0].Name)
}

func TestGetEntity(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	result, err := GetEntity(db, "GOOGLE")
	require.NoError(t, err)
	assert.Equal(t, "GOOGLE", result.Entity)
	assert.Equal(t, 2, result.DeveloperCount)
}

func TestGetEntityLike(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	results, err := GetEntityLike(db, "GOOG", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestGetEntityLike_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetEntityLike(db, "", 10)
	assert.Error(t, err)
}
```

**Step 2: Write query_test.go**

```go
package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchEvents(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	q := &EventSearchCriteria{PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_ByType(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	prType := EventTypePR
	q := &EventSearchCriteria{Type: &prType, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchEvents_ByOrg(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	org := "testorg"
	q := &EventSearchCriteria{Org: &org, PageSize: 10, Page: 1}
	results, err := SearchEvents(db, q)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestSearchEvents_NilDB(t *testing.T) {
	q := &EventSearchCriteria{PageSize: 10, Page: 1}
	_, err := SearchEvents(nil, q)
	assert.Error(t, err)
}

func TestGetEventTypeSeries(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	series, err := GetEventTypeSeries(db, nil, nil, nil, 12)
	require.NoError(t, err)
	assert.NotNil(t, series)
}

func TestEventSearchCriteria_String(t *testing.T) {
	q := EventSearchCriteria{PageSize: 10, Page: 1}
	s := q.String()
	assert.Contains(t, s, "page_size")
}
```

**Step 3: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add entity and event query tests"
```

---

### Task 14: Add CNCF affiliation parser tests

**Files:**
- Create: `pkg/data/cncf_test.go`

**Step 1: Write cncf_test.go**

```go
package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAffiliations(t *testing.T) {
	content := `jdoe: jdoe!gmail.com, jdoe!users.noreply.github.com
Google from 2020-01-01 until 2022-12-31
Microsoft from 2023-01-01
asmith: asmith!corp.com
Independent`

	dir := t.TempDir()
	path := filepath.Join(dir, "test_affiliations.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations(path, devs)
	require.NoError(t, err)

	// jdoe should be present (asmith is last and not flushed by next user)
	assert.Contains(t, devs, "jdoe")

	jdoe := devs["jdoe"]
	assert.Len(t, jdoe.Affiliations, 2)
	assert.Equal(t, "Google", jdoe.Affiliations[0].Entity)
	assert.Equal(t, "2020-01-01", jdoe.Affiliations[0].From)
	assert.Equal(t, "Microsoft", jdoe.Affiliations[1].Entity)

	// Verify noreply emails are filtered
	assert.Len(t, jdoe.Identities, 1)
	assert.Equal(t, "jdoe@gmail.com", jdoe.Identities[0])
}

func TestExtractAffiliations_EmptyPath(t *testing.T) {
	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations("", devs)
	assert.Error(t, err)
}

func TestExtractAffiliations_SkipsComments(t *testing.T) {
	content := `# This is a comment
jdoe: jdoe!test.com
Independent`

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	devs := make(map[string]*CNCFDeveloper)
	err := extractAffiliations(path, devs)
	require.NoError(t, err)
}

func TestCNCFDeveloper_GetBestIdentity(t *testing.T) {
	dev := &CNCFDeveloper{
		Identities: []string{"first@test.com", "second@test.com"},
	}
	assert.Equal(t, "first@test.com", dev.GetBestIdentity())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetBestIdentity())
}

func TestCNCFDeveloper_GetLatestAffiliation(t *testing.T) {
	dev := &CNCFDeveloper{
		Affiliations: []*CNCFAffiliation{
			{Entity: "OldCorp", From: "2020-01-01"},
			{Entity: "NewCorp", From: "2023-06-01"},
		},
	}
	assert.Equal(t, "NewCorp", dev.GetLatestAffiliation())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetLatestAffiliation())
}
```

**Step 2: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 3: Commit**

```
git add -A && git commit -S -m "add CNCF affiliation parser tests"
```

---

### Task 15: Add pure function unit tests

**Files:**
- Create: `pkg/data/event_test.go`
- Expand: `pkg/data/gh_test.go`

**Step 1: Write event_test.go**

```go
package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"@a", "@b", "@a"}, []string{"a", "b"}},
		{[]string{" x ", "@y"}, []string{"x", "y"}},
		{[]string{}, []string{}},
		{nil, []string{}},
	}

	for _, tc := range tests {
		result := unique(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}

func TestIsEventBatchValidAge(t *testing.T) {
	minTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	imp := &EventImporter{minEventTime: minTime}

	recent := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.True(t, imp.isEventBatchValidAge(&recent, &recent))
	assert.True(t, imp.isEventBatchValidAge(&recent, &old))
	assert.False(t, imp.isEventBatchValidAge(&old, &old))
	assert.False(t, imp.isEventBatchValidAge(nil, nil))
	assert.False(t, imp.isEventBatchValidAge(nil, &recent))
}

func TestQualifyTypeKey(t *testing.T) {
	imp := &EventImporter{owner: "org", repo: "repo"}
	assert.Equal(t, "org/repo/pr", imp.qualifyTypeKey("pr"))
}
```

**Step 2: Expand gh_test.go**

Add tests for `parseDate`, `getLabels`, `getUsernames`, `trim`:

```go
func TestParseDate(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2025-06-15", parseDate(&now))
	assert.NotEmpty(t, parseDate(nil)) // returns current date
}

func TestTrim(t *testing.T) {
	s := " @hello "
	assert.Equal(t, "hello", trim(&s))
	assert.Equal(t, "", trim(nil))
}

func TestGetLabels(t *testing.T) {
	// Note: getLabels uses *github.Label which requires go-github types
	// Test nil input
	assert.Empty(t, getLabels(nil))
}

func TestGetUsernames(t *testing.T) {
	assert.Empty(t, getUsernames(nil))
}
```

**Step 3: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add unit tests for pure functions"
```

---

### Task 16: Add substitution and org tests

**Files:**
- Create: `pkg/data/sub_test.go`
- Create: `pkg/data/org_test.go`

**Step 1: Write sub_test.go**

```go
package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndApplyDeveloperSub(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "user1", FullName: "User One", Entity: "OLDNAME"},
		{Username: "user2", FullName: "User Two", Entity: "OLDNAME"},
		{Username: "user3", FullName: "User Three", Entity: "OTHER"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	sub, err := SaveAndApplyDeveloperSub(db, "entity", "OLDNAME", "NEWNAME")
	require.NoError(t, err)
	assert.Equal(t, int64(2), sub.Records)

	// Verify
	dev, err := GetDeveloper(db, "user1")
	require.NoError(t, err)
	assert.Equal(t, "NEWNAME", dev.Entity)
}

func TestApplySubstitutions(t *testing.T) {
	db := setupTestDB(t)

	devs := []*Developer{
		{Username: "user1", Entity: "OLD"},
	}
	require.NoError(t, SaveDevelopers(db, devs))

	_, err := SaveAndApplyDeveloperSub(db, "entity", "OLD", "NEW")
	require.NoError(t, err)

	// Reset entity back to OLD for re-apply test
	devs[0].Entity = "OLD"
	require.NoError(t, SaveDevelopers(db, devs))

	subs, err := ApplySubstitutions(db)
	require.NoError(t, err)
	assert.NotEmpty(t, subs)
}

func TestSaveAndApplyDeveloperSub_InvalidProperty(t *testing.T) {
	db := setupTestDB(t)
	_, err := SaveAndApplyDeveloperSub(db, "invalid_prop", "old", "new")
	assert.Error(t, err)
}
```

**Step 2: Write org_test.go**

```go
package data

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllOrgRepos(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	repos, err := GetAllOrgRepos(db)
	require.NoError(t, err)
	assert.NotEmpty(t, repos)
	assert.Equal(t, "testorg", repos[0].Org)
}

func TestGetAllOrgRepos_NilDB(t *testing.T) {
	_, err := GetAllOrgRepos(nil)
	assert.Error(t, err)
}

func TestGetOrgLike(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	items, err := GetOrgLike(db, "test", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestGetOrgLike_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	_, err := GetOrgLike(db, "", 10)
	assert.Error(t, err)
}

func TestGetEntityPercentages(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	items, err := GetEntityPercentages(db, nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestGetDeveloperPercentages(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	items, err := GetDeveloperPercentages(db, nil, nil, nil, []string{}, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}
```

**Step 3: Run tests**

Run: `go test -count=1 -race -v ./pkg/data/...`
Expected: All pass

**Step 4: Commit**

```
git add -A && git commit -S -m "add substitution and org tests"
```

---

### Task 17: Add auth token tests

**Files:**
- Create: `pkg/auth/token_test.go`

**Step 1: Write token_test.go**

```go
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDeviceCode_EmptyClientID(t *testing.T) {
	_, err := GetDeviceCode("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "clientID")
}

func TestGetToken_EmptyClientID(t *testing.T) {
	_, err := GetToken("", &DeviceCode{})
	assert.Error(t, err)
}

func TestGetToken_NilCode(t *testing.T) {
	_, err := GetToken("test-client", nil)
	assert.Error(t, err)
}

func TestDeviceCode_Struct(t *testing.T) {
	dc := DeviceCode{
		DeviceCode:      "abc123",
		UserCode:        "ABCD-EFGH",
		VerificationURL: "https://github.com/login/device",
		ExpiresInSec:    900,
		Interval:        5,
	}
	assert.Equal(t, "abc123", dc.DeviceCode)
	assert.Equal(t, "ABCD-EFGH", dc.UserCode)
}

func TestAccessTokenResponse_Struct(t *testing.T) {
	raw := `{"access_token":"gho_test123","token_type":"bearer","scope":""}`
	var atr AccessTokenResponse
	err := json.Unmarshal([]byte(raw), &atr)
	require.NoError(t, err)
	assert.Equal(t, "gho_test123", atr.AccessToken)
	assert.Equal(t, "bearer", atr.TokenType)
}
```

**Step 2: Run tests**

Run: `go test -count=1 -race -v ./pkg/auth/...`
Expected: All pass

**Step 3: Commit**

```
git add -A && git commit -S -m "add auth token tests"
```

---

### Task 18: Run full test suite, check coverage, final commit

**Step 1: Run full test suite**

Run: `go test -count=1 -race -covermode=atomic -coverprofile=cover.out ./...`
Expected: All pass

**Step 2: Check coverage**

Run: `go tool cover -func=cover.out`
Expected: Coverage should be significantly improved. Document the actual percentage.

**Step 3: Build final binary**

Run: `CGO_ENABLED=0 go build -o bin/dctl cmd/cli/*.go`
Expected: Binary builds successfully

**Step 4: Final commit**

```
git add -A && git commit -S -m "v1.0.0: complete modernization overhaul

- Replace pkg/errors with stdlib fmt.Errorf
- Replace logrus with log/slog
- Upgrade go-github v45 to v83
- Replace Gin with stdlib net/http
- Add embedded SQL migration system with WAL mode
- Integrate OS keychain for token storage
- Replace globals with config struct and single DB connection
- Add GitHub API rate limit detection
- Update CNCF fetcher with dynamic discovery
- Add comprehensive test suite
- Update linter config and CI for Go 1.25
- Update goreleaser and build system"
```
