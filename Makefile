GO ?= go

.PHONY: test pr clean lint fmt install-tools docs
COVERAGE_THRESHOLD ?= 80

# Resolve the Docker socket for testcontainers.
# Precedence: DOCKER_HOST env var → active docker context → Rancher Desktop → Colima → standard socket.
DOCKER_SOCK ?= $(shell \
	if [ -n "$$DOCKER_HOST" ]; then \
		echo "$$DOCKER_HOST"; \
	elif CTX_SOCK=$$(docker context inspect --format '{{.Endpoints.docker.Host}}' 2>/dev/null) && [ -n "$$CTX_SOCK" ]; then \
		echo "$$CTX_SOCK"; \
	elif [ -S "$$HOME/.rd/docker.sock" ]; then \
		echo "unix://$$HOME/.rd/docker.sock"; \
	elif [ -S "$$HOME/.colima/default/docker.sock" ]; then \
		echo "unix://$$HOME/.colima/default/docker.sock"; \
	else \
		echo "unix:///var/run/docker.sock"; \
	fi)

# Run tests with coverage for all modules
test:
	@echo "Running tests..."
	@for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		pkgname=$$(basename $$pkg); \
		echo "Testing $$pkgname..."; \
		if [ "$$pkgname" = "kafka" ]; then \
			TEST_CMD="DOCKER_HOST=$(DOCKER_SOCK) TESTCONTAINERS_RYUK_DISABLED=true $(GO) test -v -race -count=1 -coverprofile=coverage.out -covermode=atomic ./..."; \
		else \
			TEST_CMD="$(GO) test -v -race -count=1 -coverprofile=coverage.out -covermode=atomic ./..."; \
		fi; \
		(cd $$pkg && eval "$$TEST_CMD" && \
		COVERAGE=$$($(GO) tool cover -func=coverage.out | grep total | awk '{print $$3}' | tr -d '%') && \
		echo "  Coverage: $${COVERAGE}%  (threshold: $(COVERAGE_THRESHOLD)%)" && \
		if [ "$$(echo "$$COVERAGE < $(COVERAGE_THRESHOLD)" | bc -l)" -eq 1 ]; then \
			echo "  ❌ Coverage $${COVERAGE}% is below threshold $(COVERAGE_THRESHOLD)%"; \
			rm -f coverage.out; \
			exit 1; \
		else \
			echo "  ✅ Coverage ok"; \
			rm -f coverage.out; \
		fi); \
	done

# Open a pull request against main, deriving the title from the branch name
# Branch format: type/short-description → "type: short description"
pr:
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
	TYPE=$$(echo $$BRANCH | cut -d'/' -f1); \
	DESC=$$(echo $$BRANCH | cut -d'/' -f2- | tr '-' ' '); \
	TITLE="$$TYPE: $$DESC"; \
	echo "Opening PR with title: $$TITLE"; \
	gh pr create --title "$$TITLE" --fill --base main

# Remove build and test artifacts
clean:
	@find . -name "coverage.out" -delete
	@find . -name "*.test" -delete
	@find . -name "dist" -type d -exec rm -rf {} + 2>/dev/null; true
	@echo "Cleaned"

# Run linter for all modules (installs golangci-lint from source if not present)
lint:
	@echo "Running linter..."
	@GOBIN="$$($(GO) env GOBIN)"; \
	if [ -z "$$GOBIN" ]; then GOBIN="$$($(GO) env GOPATH)/bin"; fi; \
	GOLANGCI="$$GOBIN/golangci-lint"; \
	if [ ! -f "$$GOLANGCI" ]; then \
		echo "golangci-lint not found, installing from source..."; \
		$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi; \
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		pkgname=$$(basename $$pkg); \
		echo "Linting $$pkgname..."; \
		(cd $$pkg && $$GOLANGCI run ./...); \
	done

# Format code
fmt:
	@echo "Formatting code..."
	goimports -w .

# Install development tools
install-tools:
	@echo "Installing tools..."
	$(GO) install github.com/evilmartians/lefthook@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "All tools installed"

# Generate documentation using gomarkdoc
docs:
	@echo "Generating Markdown documentation..."
	@GOBIN="$$($(GO) env GOBIN)"; \
	if [ -z "$$GOBIN" ]; then GOBIN="$$($(GO) env GOPATH)/bin"; fi; \
	GOMARKDOC="$$GOBIN/gomarkdoc"; \
	if [ ! -f "$$GOMARKDOC" ]; then \
		echo "gomarkdoc not found, installing..."; \
		$(GO) install github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0; \
	fi; \
	mkdir -p docs; \
	rm -f docs/*.md; \
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		pkgname=$$(basename $$pkg); \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		echo "Processing $$pkgname..."; \
		$$GOMARKDOC ./$$pkg/... \
			--output "docs/$$pkgname.md" \
			--repository.url "https://github.com/aalemi-dev/stdlib-lab" \
			--format github; \
	done; \
	echo "Documentation generated in docs/ directory"
