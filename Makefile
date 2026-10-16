BINARY  := repolens
BIN_DIR := bin
CMD     := ./cmd/repolens

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test test-race vet fmt fmt-check lint install clean

build: ## Build bin/repolens
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(CMD)

test: ## Run unit + integration tests
	go test ./...

test-race: ## Run tests with the race detector
	go test -race ./...

vet: ## go vet
	go vet ./...

fmt: ## Apply gofmt
	gofmt -w .

fmt-check: ## Fail if any file is not gofmt-clean
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

lint: vet ## Vet + govulncheck if installed
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || echo "govulncheck not installed; skipping"

install: ## go install with version stamp
	go install -ldflags '$(LDFLAGS)' $(CMD)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
