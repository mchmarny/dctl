RELEASE_VERSION    ?=$(shell cat ./version)
RELEASE_COMMIT     ?=$(shell git rev-parse --short HEAD)

all: help

version: ## Prints the current version
	@echo $(RELEASE_VERSION)
.PHONY: version

tidy: ## Updates the go modules and vendors all dependancies 
	go mod tidy
	go mod vendor
.PHONY: tidy

upgrade: ## Upgrades all dependancies 
	go get -d -u ./...
	go mod tidy
	go mod vendor
.PHONY: upgrade

test: tidy ## Runs unit tests
		go test -count=1 -race -covermode=atomic -coverprofile=cover.out ./...
.PHONY: test

run: tidy ## Runs uncompiled version of the app
	go run cmd/cli/*.go
.PHONY: run

cover: test ## Runs unit tests and putputs coverage
	go tool cover -func=cover.out
.PHONY: cover

lint: ## Lints the entire project 
	golangci-lint -c .golangci.yaml run
.PHONY: lint

cli: tidy ## Builds CLI binary
	CGO_ENABLED=0 go build -ldflags=" \
		-X 'main.version=$(RELEASE_VERSION)' \
		-X 'main.commit=$(RELEASE_COMMIT)' " \
		-o bin/dctl \
		cmd/cli/*.go
.PHONY: cli

dist: test lint ## Runs test, lint before building distributables
	goreleaser release --snapshot --rm-dist --timeout 10m0s
.PHONY: dist

server: cli ## Builds CLI and runs the server
	bin/dctl --debug s
.PHONY: server

local: ## Copies latest binary to local bin directory
	sudo cp bin/dctl /usr/local/bin/dctl
	sudo chmod 755 /usr/local/bin/dctl
.PHONY: local

tag: ## Creates release tag 
	git tag $(RELEASE_VERSION)
	git push origin $(RELEASE_VERSION)
.PHONY: tag

tagless: ## Delete the current release tag 
	git tag -d $(RELEASE_VERSION)
	git push --delete origin $(RELEASE_VERSION)
.PHONY: tagless

clean: ## Cleans bin and temp directories
	go clean
	rm -fr ./vendor
	rm -fr ./bin
.PHONY: clean

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk \
		'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help