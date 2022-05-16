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
	CGO_ENABLED=1 go build -ldflags=" \
		-X 'main.version=$(RELEASE_VERSION)' \
		-X 'main.commit=$(RELEASE_COMMIT)' " \
		-o bin/dctl \
		cmd/cli/*.go
.PHONY: cli

dist: tidy ## Builds CLI distributables
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags=" \
		-X 'main.version=$(RELEASE_VERSION)' \
		-X 'main.commit=$(RELEASE_COMMIT)' " \
		-o dist/dctl-darwin-arm64 \
		cmd/cli/*.go
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags=" \
		-X 'main.version=$(RELEASE_VERSION)' \
		-X 'main.commit=$(RELEASE_COMMIT)' " \
		-o dist/dctl-darwin-amd64 \
		cmd/cli/*.go
	# GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -ldflags=" \
	# 	-X 'main.version=$(RELEASE_VERSION)' \
	# 	-X 'main.commit=$(RELEASE_COMMIT)' " \
	# 	-o dist/dctl-linux-amd64 \
	# 	cmd/cli/*.go
	# GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -ldflags=" \
	# 	-X 'main.version=$(RELEASE_VERSION)' \
	# 	-X 'main.commit=$(RELEASE_COMMIT)' " \
	# 	-o dist/dctl-linux-arm64 \
	# 	cmd/cli/*.go
	# GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags=" \
	# 	-X 'main.version=$(RELEASE_VERSION)' \
	# 	-X 'main.commit=$(RELEASE_COMMIT)' " \
	# 	-o dist/dctl-windows-amd64 \
	# 	cmd/cli/*.go
.PHONY: dist

server: cli ## Builds CLI and runs the server
	bin/dctl --debug s
.PHONY: server

local: ## Copies latest binary to local bin directory
	sudo mv bin/dctl /usr/local/bin/
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