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


.PHONY: release
release: ## Cross-build linux amd64+arm64 artifacts + checksums
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY)-linux-amd64 $(CMD)
	GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY)-linux-arm64 $(CMD)
	cd $(BIN_DIR) && sha256sum $(BINARY)-linux-* > sha256sums.txt
	@ls -la $(BIN_DIR)
install: ## go install with version stamp
	go install -ldflags '$(LDFLAGS)' $(CMD)

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
