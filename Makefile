VERSION            ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
COMMIT             := $(shell git rev-parse --short HEAD)
BRANCH             := $(shell git rev-parse --abbrev-ref HEAD)
DATE               := $(shell date +%Y-%m-%dT%H:%M:%S%Z)
GO_VERSION         := $(shell go env GOVERSION 2>/dev/null | sed 's/go//')
GOLINT_VERSION      = $(shell golangci-lint --version 2>/dev/null | awk '{print $$4}' || echo "not installed")
LINT_TIMEOUT       ?= 5m
TEST_TIMEOUT       ?= 10m
COVERAGE_THRESHOLD ?= 30

all: help

# =============================================================================
# Info
# =============================================================================

.PHONY: info
info: ## Prints current project info
	@echo "version:   $(VERSION)"
	@echo "commit:    $(COMMIT)"
	@echo "branch:    $(BRANCH)"
	@echo "date:      $(DATE)"
	@echo "go:        $(GO_VERSION)"
	@echo "linter:    $(GOLINT_VERSION)"

# =============================================================================
# Code Formatting & Dependencies
# =============================================================================

.PHONY: tidy
tidy: ## Formats code and updates Go module dependencies
	go fmt ./...
	go mod tidy
	go mod vendor

.PHONY: fmt-check
fmt-check: ## Checks if code is formatted (CI-friendly, no modifications)
	@test -z "$$(gofmt -l .)" || (echo "Code not formatted. Run 'make tidy':" && gofmt -l . && exit 1)
	@echo "Code formatting check passed"

.PHONY: upgrade
upgrade: ## Upgrades all dependencies to latest versions
	go get -u ./...
	go mod tidy
	go mod vendor

# =============================================================================
# Quality
# =============================================================================

.PHONY: lint
lint: ## Lints the entire project with go vet and golangci-lint
	@set -e; \
	echo "Running go vet..."; \
	go vet ./...; \
	echo "Running golangci-lint..."; \
	golangci-lint -c .golangci.yaml run --timeout=$(LINT_TIMEOUT)

.PHONY: test
test: tidy ## Runs unit tests with race detector and coverage
	go test -short -count=1 -race -timeout=$(TEST_TIMEOUT) -covermode=atomic -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test ## Runs tests and enforces coverage threshold (COVERAGE_THRESHOLD=70)
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$coverage% (threshold: $(COVERAGE_THRESHOLD)%)"; \
	if [ $$(echo "$$coverage < $(COVERAGE_THRESHOLD)" | bc) -eq 1 ]; then \
		echo "ERROR: Coverage $$coverage% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi; \
	echo "Coverage check passed"

.PHONY: bench
bench: ## Runs benchmarks
	go test -bench=. -benchmem ./...

.PHONY: scan
scan: ## Scans for vulnerabilities with grype
	grype dir:. --fail-on high --quiet

.PHONY: qualify
qualify: test-coverage lint scan ## Qualifies the codebase (test-coverage, lint, scan)
	@echo "Codebase qualification completed"

# =============================================================================
# Build & Release
# =============================================================================

.PHONY: build
build: tidy ## Builds binaries for current OS and architecture
	@set -e; \
	GITHUB_TOKEN= GITLAB_TOKEN= goreleaser build --clean --single-target --snapshot --timeout 10m0s || exit 1; \
	echo "Build completed, binaries in ./dist"

.PHONY: release
release: ## Runs the full release process with goreleaser
	goreleaser release --snapshot --clean --timeout 10m0s

.PHONY: server
server: ## Starts local development server with debug logging
	go run ./cmd/dctl --debug s

.PHONY: local
local: build ## Copies latest binary to local bin directory
	sudo cp $$(find dist -name dctl -type f | head -1) /usr/local/bin/dctl
	sudo chmod 755 /usr/local/bin/dctl

.PHONY: bump-major
bump-major: ## Bumps major version (1.2.3 → 2.0.0)
	tools/bump major

.PHONY: bump-minor
bump-minor: ## Bumps minor version (1.2.3 → 1.3.0)
	tools/bump minor

.PHONY: bump-patch
bump-patch: ## Bumps patch version (1.2.3 → 1.2.4)
	tools/bump patch

# =============================================================================
# Cleanup
# =============================================================================

.PHONY: clean
clean: ## Cleans build artifacts
	rm -rf ./dist ./bin ./coverage.out
	go clean ./...

.PHONY: clean-all
clean-all: clean ## Deep cleans including Go module cache
	go clean -modcache

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help: ## Displays available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk \
		'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
