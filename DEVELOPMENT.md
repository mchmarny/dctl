# Development Guide

This guide covers project setup, architecture, development workflows, and tooling for contributors working on devpulse.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Project Architecture](#project-architecture)
- [Development Workflow](#development-workflow)
- [Make Targets Reference](#make-targets-reference)
- [Debugging](#debugging)

## Quick Start

```bash
# 1. Clone and setup
git clone https://github.com/mchmarny/devpulse.git && cd devpulse
make tidy           # Download dependencies

# 2. Develop
make test           # Run tests with race detector
make lint           # Run linters
make build          # Build binary

# 3. Before submitting PR
make qualify        # Full check: test + lint + vulncheck + e2e
```

## Prerequisites

### Required Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **Go 1.26+** | Language runtime | [golang.org/dl](https://golang.org/dl/) |
| **make** | Build automation | Pre-installed on macOS; `apt install make` on Ubuntu/Debian |
| **git** | Version control | Pre-installed on most systems |

### Development Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| golangci-lint | Go linting | [golangci-lint.run](https://golangci-lint.run/welcome/install/) |
| yamllint | YAML linting | `pip install yamllint` or `brew install yamllint` |
| govulncheck | Vulnerability scanning | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| goreleaser | Release automation / local builds | [goreleaser.com](https://goreleaser.com/install/) |
| jq | JSON processing (for e2e tests) | `brew install jq` or `apt install jq` |

## Development Setup

### Clone and Build

```bash
git clone https://github.com/mchmarny/devpulse.git
cd devpulse
```

Build for your current platform:

```bash
make build
```

The binary is in `./dist`. To install it to `/usr/local/bin`:

```bash
make local
```

### Finalize Setup

After cloning:

```bash
# Download Go module dependencies
make tidy

# Run full qualification to ensure setup is correct
make qualify
```

## Project Architecture

### Directory Structure

```
devpulse/
├── cmd/devpulse/     Main entrypoint (thin wrapper)
├── pkg/
│   ├── cli/          CLI commands, HTTP handlers, templates, static assets
│   ├── data/         Data layer: SQLite queries, GitHub importers, migrations
│   ├── auth/         GitHub OAuth token management (OS keychain)
│   └── net/          HTTP client utilities
├── tools/            Development scripts (bump, common)
└── docs/             Documentation and images
```

### Key Components

#### CLI
- **Location**: `pkg/cli/`
- **Framework**: [urfave/cli v2](https://github.com/urfave/cli)
- **Commands**: `auth`, `import`, `query`, `server`, `reset`, `substitute`

#### Data Layer
- **Location**: `pkg/data/`
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Patterns**: Upserts via `INSERT ... ON CONFLICT`, COALESCE for optional filters, transactions with explicit rollback

#### GitHub Importer
- **Location**: `pkg/data/`
- **Pattern**: Concurrent, batched event import with rate limit detection and jitter backoff
- **Library**: `github.com/google/go-github/v83/github`

#### HTTP Server
- **Location**: `pkg/cli/`
- **Pattern**: `http.HandlerFunc` closures with `writeJSON`/`writeError` helpers
- **Dashboard**: Chart.js-powered analytics served on localhost

### Data Flow

```
GitHub API --> devpulse import --> SQLite --> devpulse server --> localhost:8080
                                           \--> devpulse query --> JSON (stdout)
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feat/add-retention-chart
```

### 2. Make Changes

- Small, focused commits: each commit should address one logical change
- Test as you go: write tests alongside your code
- Read existing code in the package before modifying it

### 3. Run Tests

```bash
# Unit tests with race detector
make test
```

### 4. Lint Your Code

```bash
make lint
```

### 5. Vulnerability Check

```bash
make vulncheck
```

### 6. End-to-End Tests

```bash
make e2e
```

### 7. Full Qualification

Before submitting a PR, run everything:

```bash
make qualify
```

This runs: `test` -> `lint` -> `vulncheck` -> `e2e`

### 7. Run Locally

```bash
# Start dev server with debug logging
make server
```

## Make Targets Reference

### Quality and Testing

| Target | Description |
|--------|-------------|
| `make qualify` | Full qualification (test + lint + vulncheck + e2e) |
| `make test` | Unit tests with race detector and coverage |
| `make test-coverage` | Tests with coverage threshold enforcement |
| `make lint` | Go lint + YAML lint (`lint-go` + `lint-yaml`) |
| `make lint-go` | Go vet + golangci-lint |
| `make lint-yaml` | YAML linting with yamllint |
| `make vulncheck` | Vulnerability scanning with govulncheck |
| `make e2e` | End-to-end CLI tests |
| `make bench` | Run benchmarks |

### Build and Release

| Target | Description |
|--------|-------------|
| `make build` | Build binary for current OS/arch |
| `make release` | Full release with goreleaser (snapshot) |
| `make local` | Build and install binary to /usr/local/bin |
| `make bump-major` | Bump major version (1.2.3 -> 2.0.0) |
| `make bump-minor` | Bump minor version (1.2.3 -> 1.3.0) |
| `make bump-patch` | Bump patch version (1.2.3 -> 1.2.4) |

### Code Maintenance

| Target | Description |
|--------|-------------|
| `make tidy` | Format code and update dependencies |
| `make fmt-check` | Check code formatting (CI-friendly) |
| `make upgrade` | Upgrade all dependencies |

### Utilities

| Target | Description |
|--------|-------------|
| `make info` | Print project info (version, commit, tools) |
| `make server` | Start local dev server with debug logging |
| `make clean` | Clean build artifacts |
| `make clean-all` | Deep clean including module cache |
| `make help` | Show all available targets |

## Debugging

### Common Issues

| Issue | Solution |
|-------|----------|
| Tests fail with race conditions | Check for shared state in goroutines |
| Linter errors | Run `make lint` and fix reported issues |
| Build failures | Run `make tidy` to update dependencies |
| Import hits rate limit | Re-run; the importer uses jitter backoff automatically |

### Debugging Tests

```bash
# Run specific test with verbose output
go test -v ./pkg/data/... -run TestSpecificFunction

# Run tests with race detector
go test -race ./...

# Generate coverage report
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

### Debugging the Server

```bash
# Start with debug logging
make server

# Or directly
go run ./cmd/devpulse --debug s
```

## Additional Resources

- [CONTRIBUTING.md](CONTRIBUTING.md) - How to contribute
- [README.md](README.md) - Project overview and quick start
