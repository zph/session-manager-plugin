.DEFAULT_GOAL := help

# Variables
GORELEASER := goreleaser
GO := go
GOLANGCI_LINT := golangci-lint

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Format Go code
	$(GO) fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: golangci-lint
golangci-lint: ## Run golangci-lint
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run --fast ./...; \
	else \
		echo "golangci-lint not found. Skipping (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"; \
	fi

.PHONY: lint
lint: fmt vet ## Run all linting checks (fmt + vet)

.PHONY: checkstyle
checkstyle: lint ## Run checkstyle checks (alias for lint)

.PHONY: test
test: ## Run all tests
	$(GO) clean -testcache
	$(GO) test -cover -gcflags "-N -l" ./src/... -test.paniconexit0=false

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	$(GO) clean -testcache
	$(GO) test -v -cover -gcflags "-N -l" ./src/... -test.paniconexit0=false

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GO) clean -testcache
	$(GO) test -race ./src/...

.PHONY: test-bench
test-bench: ## Run benchmarks and show test durations
	$(GO) test -v -bench=. -benchmem -run=^$$ ./src/... || true
	@echo "\n=== Test Performance ==="
	$(GO) test -v ./src/... 2>&1 | grep -E "^(--- PASS:|--- FAIL:|ok\s+|FAIL\s+)" | grep -v "coverage:"

.PHONY: test-short
test-short: ## Run only fast unit tests (skip slow integration tests)
	$(GO) clean -testcache
	$(GO) test -short -cover ./src/...

.PHONY: test-integration
test-integration: ## Run only slow integration tests
	$(GO) clean -testcache
	$(GO) test -tags=integration -cover ./src/...

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf build/ bin/ dist/ pkg/ vendor/bin/ vendor/pkg/ .cover/
	find . -type f -name '*.log' -delete

.PHONY: build
build: ## Build binaries for all platforms using goreleaser
	$(GORELEASER) build --clean --snapshot

.PHONY: build-local
build-local: ## Build binary for current platform only
	$(GO) build -ldflags "-s -w" -o bin/session-manager-plugin ./src/sessionmanagerplugin-main/main.go
	$(GO) build -ldflags "-s -w" -o bin/ssmcli ./src/ssmcli-main/main.go
	$(GO) build -ldflags "-s -w" -o bin/ssm-port-forward ./src/ssm-port-forward-main/main.go

.PHONY: snapshot
snapshot: checkstyle test ## Create a snapshot release (no git tag required)
	$(GORELEASER) release --snapshot --clean

.PHONY: release
release: checkstyle test ## Create a full release (requires git tag)
	$(GORELEASER) release --clean

.PHONY: release-dry-run
release-dry-run: ## Test the release process without publishing
	$(GORELEASER) release --skip=publish --clean

.PHONY: install-tools
install-tools: ## Install required build tools
	@echo "Installing build tools..."
	@if ! command -v $(GORELEASER) >/dev/null 2>&1; then \
		echo "Installing goreleaser..."; \
		$(GO) install github.com/goreleaser/goreleaser@latest; \
	else \
		echo "✓ goreleaser is already installed"; \
	fi
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	else \
		echo "✓ golangci-lint is already installed"; \
	fi

.PHONY: ci
ci: lint test ## Run CI checks (lint + test)

.PHONY: check-goreleaser
check-goreleaser: ## Validate goreleaser configuration
	$(GORELEASER) check

.PHONY: version
version: ## Generate version file
	$(GO) run ./src/version/versiongenerator/version-gen.go

.PHONY: tag
tag:
	git tag -a v0.0.0-$(shell cat VERSION) -m v0.0.0-$(shell cat VERSION)
