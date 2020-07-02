BINARY_NAME=contain
BIN = bin/$(BINARY_NAME)

DATE    ?= $(shell date +%FT%T%z)
BRANCH  ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT  ?= $(shell git rev-parse HEAD)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)
GOLDFLAGS="-X main.branch=$(BRANCH) -X main.commit=$(COMMIT) -X main.version=$(VERSION)"
PACKAGENAME="github.com/byxorna/simple-container-executor"
GO15VENDOREXPERIMENT=1
PKGS     = $(or $(PKG),$(shell go list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell env go list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

all: lint fmt test build

.PHONY: lint
lint: vendor ; $(info $(M) running golint…)
	golint $(PKGS)

fmt: ; $(info running gofmt…) @
	go fmt $(PKGS)

# Dependency management

vendor: go.mod go.sum | ; $(info retrieving dependencies…)
	go mod download 2>&1
	go mod vendor

install: test
	@go install -ldflags=$(GOLDFLAGS)

build:
	@go build -ldflags=$(GOLDFLAGS) -o $(BIN) $(PACKAGE)/cmd

test: fmt vet errcheck
	@go test ./... -cover

linux: test
	GOARCH=amd64 GOOS=linux go build -ldflags=$(GOLDFLAGS) -o $(BIN)

clean: ; $(info cleaning…)	@ ## Cleanup everything
	@rm -rf vendor/
	@rm -rf bin
	@rm -rf test/tests.* test/coverage.*

help:
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

version:
	@echo $(VERSION)

.PHONY: errcheck vet fmt install build test clean help version

