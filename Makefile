GO ?= go
COVERAGE_THRESHOLD ?= 80

# Run tests with coverage for all modules
test:
	@echo "Running tests..."
	@for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		pkgname=$$(basename $$pkg); \
		echo "Testing $$pkgname..."; \
		(cd $$pkg && $(GO) test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./... && \
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

# Remove build and test artifacts
clean:
	@find . -name "coverage.out" -delete
	@find . -name "*.test" -delete
	@find . -name "dist" -type d -exec rm -rf {} + 2>/dev/null; true
	@echo "Cleaned"

# Run linter for all modules (installs golangci-lint from source if not present)
lint:
	@echo "Running linter..."
	@export PATH="$$($(GO) env GOPATH)/bin:$$PATH"; \
	which golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found, installing from source..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}; \
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		pkgname=$$(basename $$pkg); \
		echo "Linting $$pkgname..."; \
		(cd $$pkg && golangci-lint run ./...); \
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
	$(GO) install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "All tools installed"

# Generate documentation using gomarkdoc
docs:
	@echo "Generating Markdown documentation..."
	@export PATH="$$($(GO) env GOPATH)/bin:$$PATH"; \
	which gomarkdoc >/dev/null 2>&1 || { \
		echo "gomarkdoc not found, installing..."; \
		$(GO) install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest; \
		echo "gomarkdoc installed successfully"; \
	}; \
	mkdir -p docs; \
	rm -f docs/*.md; \
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)"); do \
		pkgname=$$(basename $$pkg); \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		echo "Processing $$pkgname..."; \
		gomarkdoc ./$$pkg/... \
			--output "docs/$$pkgname.md" \
			--repository.url "https://github.com/Abolfazl-Alemi/stdlib-lab" \
			--format github; \
	done; \
	echo "Documentation generated in docs/ directory"
