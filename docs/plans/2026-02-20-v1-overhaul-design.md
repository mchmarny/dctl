# dctl v1.0.0 Overhaul Design

## Goal

Modernize dctl from v0.4.0 to v1.0.0: replace deprecated dependencies with stdlib equivalents, upgrade GitHub API client, improve architecture, and increase test coverage to 60%+.

## Phasing Strategy

Four phases, each independently buildable and testable:

| Phase | Focus | Dependencies |
|-------|-------|-------------|
| P1: Foundation | Error handling, logging, deprecated APIs, build system | None |
| P2: Dependencies | go-github v45->v83, Gin->stdlib net/http | P1 |
| P3: Architecture | DB migrations, DI, connection mgmt, keychain, CNCF update | P1 |
| P4: Quality | Test coverage 60%+, rate limiting, error recovery | P2, P3 |

## P1: Foundation Modernization

### Error Handling
- Replace `github.com/pkg/errors` with `fmt.Errorf("...: %w", err)` and `errors.New`
- Remove `pkg/errors` from go.mod

### Logging
- Replace `github.com/sirupsen/logrus` with `log/slog`
- Create setup function in `internal/logger` configuring text/JSON handler based on debug flag
- Replace `log.WithFields(...)` with `slog.With(...)`
- Replace `log.Debugf/Infof/Errorf` with `slog.Debug/Info/Error` + structured attrs
- Remove global mutable `debug` var; pass logger through config struct

### Deprecated APIs
- `io/ioutil.ReadAll` -> `io.ReadAll` in `pkg/auth/token.go`
- Fix `.golangci.yaml` Go version from 1.18 to match go.mod
- Update deprecated linter names: deadcode->unused, varcheck->unused

### Build System
- Add `.versions.yaml` as single source of truth for tool versions
- Update Makefile `goreleaser` flags: `--rm-dist` -> `--clean`
- Version bump to v1.0.0

## P2: Dependency Upgrades

### go-github v45 -> v83
- Update import paths `go-github/v45` -> `go-github/v83`
- Adapt to API changes in `event.go` (all 5 importer methods):
  - `github.Timestamp` changes
  - Pagination API updates
  - `Rate` struct location
  - `ListOptions` changes
- Update `pkg/net/client.go` OAuth client creation

### Gin -> stdlib net/http (Go 1.22+)
- Replace `gin.Engine` with `http.ServeMux` pattern routing
- Replace `gin.Context` with `http.Request`/`http.ResponseWriter`
- Move template rendering to direct `html/template` usage
- Replace `r.StaticFS` with `http.FileServerFS`
- Replace route groups with `mux.Handle("GET /data/...")` patterns
- Remove `gin-gonic/gin` and ~15 transitive deps

## P3: Architecture Improvements

### Database Migration System
- Embed numbered SQL files: `sql/migrations/001_initial.sql`, `002_add_indexes.sql`, etc.
- Add `schema_version` table tracking applied migrations
- On startup: diff embedded vs applied, run missing in order
- Current `ddl.sql` becomes `001_initial.sql`

### Token Storage: OS Keychain Integration
- Replace plaintext `~/.dctl/github_token` with `zalando/go-keyring`
- Service name: `dctl`, key: `github_token`
- Fallback: file-based storage with warning if keychain unavailable
- Migration: on first run with keychain, read existing file token, store in keychain, delete file

### Dependency Injection / Connection Management
- Replace global `dbFilePath` var with config struct passed through CLI context
- Single `*sql.DB` opened at startup, passed through (not per-operation GetDB)
- Enable SQLite WAL mode for concurrent read performance
- `defer db.Close()` at top level only

### Context Propagation
- Thread `context.Context` from CLI command through all operations
- Ctrl+C cancels in-flight GitHub API calls
- Replace `context.Background()` in `ImportEvents` with caller-provided context

### GitHub API Rate Limiting
- Check `resp.Rate.Remaining` after each API call
- Sleep until `resp.Rate.Reset` when remaining < threshold (10)
- Add jitter (0-2s random) to avoid thundering herd

### CNCF Affiliation Fetcher Update
- Replace hardcoded `numOfAffilFilesToDownload = 100` with dynamic discovery
- Fetch files sequentially until 404, then stop
- Also check `cncf/dev-affiliations` repo as data source
- Add context support for cancellation

## P4: Test Coverage & Quality

### Testing Strategy
- **Unit tests**: pure functions (unique, parseUsers, parseDate, getLabels, entity name cleaning)
- **Integration tests**: in-memory SQLite (`:memory:`) for CRUD, migrations, queries
- **Interface mocking**: define interface over used `*github.Client` methods, test double
- **HTTP handler tests**: `httptest.NewRecorder()` for `/data/*` endpoints
- **Target**: 60%+ coverage on non-vendor code

### Test Files to Add

| File | Coverage |
|------|----------|
| `pkg/data/db_test.go` | Migration system, GetDB, schema creation |
| `pkg/data/event_test.go` | EventImporter.add, flush, isEventBatchValidAge, unique |
| `pkg/data/query_test.go` | SQL query builders, time series, search |
| `pkg/data/entity_test.go` | Entity CRUD, query operations |
| `pkg/data/state_test.go` | State load/save, pagination tracking |
| `pkg/auth/token_test.go` | Token request/response (httptest server) |
| `cmd/cli/data_test.go` | HTTP handler responses |
| `pkg/data/cncf_test.go` | Affiliation parsing, extraction |

### Error Recovery
- Partial import failure: log + continue with next repo
- Transaction failure: rollback with error context (improve existing)
- Network timeout: bounded retries with exponential backoff + jitter (max 3 retries)

## Decisions

| Decision | Rationale |
|----------|-----------|
| stdlib over frameworks | Fewer deps, Go 1.22+ routing sufficient |
| `log/slog` over zerolog/zap | Stdlib, zero deps, adequate for CLI |
| Embedded migrations over goose/migrate | No external dep for simple schema |
| `zalando/go-keyring` for tokens | Cross-platform (macOS/Linux/Windows), pure Go |
| go-github v83 | Latest, actively maintained |
| Interface mocking over gomock | Simpler, less codegen, fits project scale |
| Keep urfave/cli/v2 | Works, low-churn area |
| Version v1.0.0 | Signals production-ready after overhaul |

## Dependency Changes

### Added
- `github.com/google/go-github/v83`
- `github.com/zalando/go-keyring`

### Removed
- `github.com/pkg/errors`
- `github.com/sirupsen/logrus`
- `github.com/gin-gonic/gin` (+ ~15 transitive: gin-contrib/sse, go-playground/validator, goccy/go-json, ugorji/go, bytedance/sonic, etc.)
- `github.com/google/go-github/v45`

### Kept
- `github.com/urfave/cli/v2`
- `github.com/stretchr/testify`
- `golang.org/x/oauth2`
- `modernc.org/sqlite`
