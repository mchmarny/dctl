# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`dctl` is a Go CLI that imports GitHub contribution data into a local SQLite database and serves a browser-based analytics dashboard. No external services, no Kubernetes, no cloud dependencies at runtime.

## Build & Test

```shell
make test          # unit tests with race detector
make lint          # go vet + golangci-lint
make qualify       # test + lint + grype vulnerability scan
make build         # goreleaser single-target build
make server        # run dev server with --debug
```

Go version is pinned in `go.mod` (currently 1.26). The `.golangci.yaml` at the repo root configures linting.

## Non-Negotiable Rules

1. **Read before writing** — Never modify code you haven't read
2. **Tests must pass** — `make test` with race detector; never skip or disable tests
3. **Run `make qualify` often** — Run at every stopping point (after completing a phase, before commits). Fix ALL lint/test failures before proceeding
4. **Use project patterns** — Learn existing code before inventing new approaches
5. **3-strike rule** — After 3 failed fix attempts, stop and reassess

## Git Configuration

- Commit to `main` branch (not `master`)
- Do use `-S` to cryptographically sign the commit
- Do NOT add `Co-Authored-By` lines (organization policy)
- Do not sign-off commits (no `-s` flag) unless the commit can't be cryptographically signed

## Code Conventions

**Error handling:**
- Use `fmt.Errorf("context: %w", err)` for wrapping — this is the project-wide pattern
- Sentinel errors defined as package-level vars (e.g. `errDBNotInitialized`)

**Logging:**
- Use `log/slog` (Info, Debug, Warn, Error) — never `fmt.Println`

**Database:**
- All SQL constants defined at the top of the file they're used in
- COALESCE pattern for optional filters on non-nullable columns: `WHERE col = COALESCE(?, col)`
- IFNULL/COALESCE pattern for nullable columns (e.g. entity): `IFNULL(d.entity, '') = COALESCE(?, IFNULL(d.entity, ''))`
- Developer upsert preserves existing entity when new value is empty: `CASE WHEN ? = '' THEN COALESCE(developer.entity, '') ELSE ? END`
- Transactions with explicit rollback on error
- Upserts via `INSERT ... ON CONFLICT(...) DO UPDATE SET`

**HTTP handlers:**
- Return `http.HandlerFunc` closures: `func handler(db *sql.DB) http.HandlerFunc`
- Use `writeJSON(w, status, v)` and `writeError(w, status, msg)` helpers
- Query params: `r.URL.Query().Get("key")`, convert with `queryParamInt()`

**Imports:**
- GitHub API via `github.com/google/go-github/v83/github`
- CLI via `github.com/urfave/cli/v2`
- Testing via `github.com/stretchr/testify` (assert + require)
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)

**Testing:**
- `setupTestDB(t)` helper creates temp DB with all migrations
- Table-driven tests where applicable
- Test both nil DB and empty DB cases for query functions

## Anti-Patterns (Do Not Do)

| Anti-Pattern | Correct Approach |
|---|---|
| Modify code without reading it first | Always `Read` files before `Edit` |
| Skip or disable tests to make CI pass | Fix the actual issue |
| Invent new patterns | Study existing code in same package first |
| Use `fmt.Println` for logging | Use `slog.Info/Debug/Warn/Error` |
| Add features not requested | Implement exactly what was asked |
| Create new files when editing suffices | Prefer `Edit` over `Write` |
| Guess at missing parameters | Ask for clarification |
| Continue after 3 failed fix attempts | Stop, reassess approach, explain blockers |

## Design Principles

- Partial failure is the steady state — design for timeouts, bounded retries
- Boring first — default to proven, simple technologies
- Observability is mandatory — structured logging
- Correctness must be reproducible — same inputs, same outputs
- Trust requires verifiable provenance — SBOM, Sigstore, GitHub attestations

## Decision Framework

When choosing between approaches, prioritize in this order:
1. **Testability** — Can it be unit tested without external dependencies?
2. **Readability** — Can another engineer understand it quickly?
3. **Consistency** — Does it match existing patterns in the codebase?
4. **Simplicity** — Is it the simplest solution that works?
5. **Reversibility** — Can it be easily changed later?

## Architecture

```
cmd/dctl/         Main entrypoint (thin wrapper)
pkg/cli/          CLI commands, HTTP handlers, templates, static assets
pkg/data/         Data layer: SQLite queries, GitHub importers, migrations
pkg/auth/         GitHub OAuth token management (OS keychain)
pkg/net/          HTTP client utilities
tools/            Dev scripts (version bump, shared helpers)
```

CLI commands: `auth`, `import`, `substitute`, `query`, `server`, `reset`

Key data flow: GitHub API → EventImporter (concurrent, batched) → SQLite → HTTP API → Chart.js dashboard

Dashboard is full-width (no sidebar). Theme toggle is in the top bar next to search. All insight panels support entity filtering via `?e=` query param.

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `test-on-push.yaml` | push to main, PRs | Calls reusable test workflow |
| `test-on-call.yaml` | reusable (workflow_call) | tidy, lint, test with race detector |
| `image-on-tag.yaml` | version tags (`v*.*.*`) | goreleaser build, cosign signing, SBOM, attestations, Homebrew tap |
| `codeql-analysis.yml` | schedule, push | CodeQL security analysis (Go + JavaScript) |

## Release Process

Releases are triggered by version tags. Use `make bump-patch`, `make bump-minor`, or `make bump-major` to tag and push.

- **Build**: goreleaser v2 cross-compiles for darwin/linux/windows × amd64/arm64
- **Signing**: cosign v3 keyless signing via Sigstore OIDC; produces `.sigstore.json` bundles (not separate .sig/.pem)
- **SBOM**: syft generates SPDX JSON for each binary
- **Attestations**: GitHub build provenance attestations via `actions/attest-build-provenance`
- **Homebrew**: goreleaser pushes formula to `mchmarny/homebrew-dctl` tap
