VERSION            ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
COMMIT             := $(shell git rev-parse --short HEAD)
BRANCH             := $(shell git rev-parse --abbrev-ref HEAD)
DATE               := $(shell date +%Y-%m-%dT%H:%M:%S%Z)
GO_VERSION         := $(shell go env GOVERSION 2>/dev/null | sed 's/go//')
GOLINT_VERSION      = $(shell golangci-lint --version 2>/dev/null | awk '{print $$4}' || echo "not installed")
LINT_TIMEOUT       ?= 5m
TEST_TIMEOUT       ?= 10m
YAML_FILES         := $(shell find . ! -path "./vendor/*" -type f -regex ".*\.yaml")
COVERAGE_THRESHOLD ?= $(shell awk '/^target:/{print $$2}' .codecov.yaml 2>/dev/null || echo 30)

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
	@UNFORMATTED=$$(gofmt -l $$(go list -f '{{.Dir}}' ./...)); \
	test -z "$$UNFORMATTED" || (echo "Code not formatted. Run 'make tidy':" && echo "$$UNFORMATTED" && exit 1)
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
lint: lint-go lint-yaml ## Lints Go code and YAML files

.PHONY: lint-go
lint-go: ## Lints Go code with go vet and golangci-lint
	@set -e; \
	echo "Running go vet..."; \
	GOFLAGS="-mod=vendor" go vet ./...; \
	echo "Running golangci-lint..."; \
	golangci-lint -c .golangci.yaml run --timeout=$(LINT_TIMEOUT)

.PHONY: lint-yaml
lint-yaml: ## Lints YAML files with yamllint
	yamllint -c .yamllint.yaml $(YAML_FILES)

.PHONY: test
test: tidy ## Runs unit tests with race detector and coverage
	GOFLAGS="-mod=vendor" go test -short -count=1 -race -timeout=$(TEST_TIMEOUT) -covermode=atomic -coverprofile=cover.out ./...

.PHONY: test-coverage
test-coverage: test ## Runs tests and enforces coverage threshold
	@coverage=$$(go tool cover -func=cover.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$coverage% (threshold: $(COVERAGE_THRESHOLD)%)"; \
	if [ $$(echo "$$coverage < $(COVERAGE_THRESHOLD)" | bc) -eq 1 ]; then \
		echo "ERROR: Coverage $$coverage% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi; \
	echo "Coverage check passed"

.PHONY: bench
bench: ## Runs benchmarks
	go test -bench=. -benchmem ./...

.PHONY: vulncheck
vulncheck: ## Scans for known vulnerabilities with govulncheck
	GOFLAGS="-mod=vendor" govulncheck -test ./...

.PHONY: e2e
e2e: ## Runs end-to-end CLI tests
	tools/e2e

.PHONY: qualify
qualify: test-coverage lint vulncheck e2e ## Qualifies the codebase (test, lint, vulncheck, e2e)
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
	go run ./cmd/devpulse --debug s

.PHONY: local
local: build ## Copies latest binary to local bin directory
	sudo cp $$(find dist -name devpulse -type f | head -1) /usr/local/bin/devpulse
	sudo chmod 755 /usr/local/bin/devpulse

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
	rm -rf ./dist ./bin ./cover.out
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
