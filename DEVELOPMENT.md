# Development Guide

## Quick Start

```bash
git clone https://github.com/mchmarny/devpulse.git && cd devpulse
make tidy           # format code and vendor dependencies
make test           # unit tests with race detector
make lint           # go vet + golangci-lint + yamllint
make build          # build binary for current platform
make qualify        # full check: test + lint + vulncheck + e2e
```

## Prerequisites

### Required

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
| goreleaser | Release builds | [goreleaser.com](https://goreleaser.com/install/) |
| jq | JSON processing (e2e tests) | `brew install jq` or `apt install jq` |
| yq | YAML processing (.settings.yaml) | `brew install yq` or [github.com/mikefarah/yq](https://github.com/mikefarah/yq) |

Tool versions and quality thresholds are centralized in `.settings.yaml`.

## Development Workflow

### 1. Create a branch

```bash
git checkout -b feat/my-feature
```

### 2. Make changes

- Read existing code in the package before modifying it
- Write tests alongside your code
- Small, focused commits — each addresses one logical change

### 3. Test and lint

```bash
make test           # unit tests with race detector
make lint           # go vet + golangci-lint + yamllint
```

### 4. Run locally

```bash
make server         # start dashboard with debug logging
```

### 5. Qualify before submitting

```bash
make qualify        # test + lint + vulncheck + e2e
```

This must pass before any PR is submitted.

## Make Targets

### Quality

| Target | Description |
|--------|-------------|
| `make qualify` | Full qualification (test + lint + vulncheck + e2e) |
| `make test` | Unit tests with race detector and coverage |
| `make test-coverage` | Tests with coverage threshold enforcement |
| `make lint` | Go + YAML linting |
| `make vulncheck` | Vulnerability scanning with govulncheck |
| `make e2e` | End-to-end CLI tests |
| `make bench` | Run benchmarks |

### Build

| Target | Description |
|--------|-------------|
| `make build` | Build binary for current OS/arch (output in `./dist`) |
| `make release` | Full release with goreleaser (snapshot) |
| `make local` | Build and install binary to `/usr/local/bin` |

### Release

| Target | Description |
|--------|-------------|
| `make bump-patch` | Bump patch version (0.10.1 → 0.10.2) and push tag |
| `make bump-minor` | Bump minor version (0.10.1 → 0.11.0) and push tag |
| `make bump-major` | Bump major version (0.10.1 → 1.0.0) and push tag |

Pushing a version tag triggers the CI release workflow (goreleaser build, cosign signing, SBOM, attestations, Homebrew tap update).

### Maintenance

| Target | Description |
|--------|-------------|
| `make tidy` | Format code, tidy modules, vendor dependencies |
| `make upgrade` | Upgrade all dependencies to latest |
| `make clean` | Clean build artifacts |
| `make clean-all` | Deep clean including Go module cache |
| `make info` | Print version, commit, branch, Go version, linter version |
| `make server` | Start dev server with debug logging |
| `make help` | Show all available targets |

## Debugging

### Common Issues

| Issue | Solution |
|-------|----------|
| Tests fail with race conditions | Check for shared state in goroutines |
| Linter errors | Run `make lint` and fix reported issues |
| Build failures | Run `make tidy` to update dependencies |
| Import hits rate limit | Re-run; the importer uses jitter backoff automatically |
| `make server` fails | Ensure you have imported data first (`devpulse import --org <org>`) |

### Running Specific Tests

```bash
# Single test with verbose output
go test -v ./pkg/data/... -run TestSpecificFunction

# Tests with race detector
go test -race ./...

# Coverage report
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

### Debug Logging

```bash
# Via make
make server

# Directly
go run ./cmd/devpulse --debug serve

# JSON format (useful for cloud environments or log aggregators)
go run ./cmd/devpulse --debug --log-json serve
```

## Related Documentation

- [README.md](README.md) — project overview and quick start
- [CONTRIBUTING.md](CONTRIBUTING.md) — contribution guidelines
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system architecture and design
- [docs/IMPORT.md](docs/IMPORT.md) — import command details
- [docs/SCORE.md](docs/SCORE.md) — reputation scoring
- [docs/SERVER.md](docs/SERVER.md) — dashboard and server
- [docs/QUERY.md](docs/QUERY.md) — CLI query interface
