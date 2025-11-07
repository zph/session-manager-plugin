.DEFAULT_GOAL := help

# Variables
GORELEASER := goreleaser
GO := go

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: checkstyle
checkstyle: ## Run checkstyle checks
	./Tools/src/checkstyle.sh

.PHONY: test
test: ## Run all tests
	$(GO) clean -testcache
	$(GO) test -cover -gcflags "-N -l" ./src/... -test.paniconexit0=false

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	$(GO) clean -testcache
	$(GO) test -v -cover -gcflags "-N -l" ./src/... -test.paniconexit0=false

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
	@echo "Installing goreleaser..."
	@if ! command -v $(GORELEASER) >/dev/null 2>&1; then \
		echo "Installing goreleaser via go install..."; \
		$(GO) install github.com/goreleaser/goreleaser@latest; \
	else \
		echo "goreleaser is already installed"; \
	fi

.PHONY: check-goreleaser
check-goreleaser: ## Validate goreleaser configuration
	$(GORELEASER) check

.PHONY: version
version: ## Generate version file
	$(GO) run ./src/version/versiongenerator/version-gen.go
