.PHONY: build test test-race lint vet tidy clean install help release-snapshot release-check

BIN := plumbline
PKG := github.com/sroberts/plumbline
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -X $(PKG)/internal/buildinfo.Version=$(VERSION) -X $(PKG)/internal/buildinfo.Commit=$(COMMIT)

build:
	go build -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/plumbline

install:
	go install -ldflags='$(LDFLAGS)' ./cmd/plumbline

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

lint: vet
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "(golangci-lint not installed; ran go vet only)"

clean:
	rm -f $(BIN)
	rm -rf dist build

release-snapshot:
	@command -v goreleaser >/dev/null || { echo "install goreleaser: https://goreleaser.com/install/"; exit 1; }
	goreleaser release --snapshot --clean --skip=publish

release-check:
	@command -v goreleaser >/dev/null || { echo "install goreleaser: https://goreleaser.com/install/"; exit 1; }
	goreleaser check

help:
	@echo "Targets: build test test-race vet tidy lint clean install release-snapshot release-check"
