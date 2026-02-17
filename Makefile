GO ?= go

# Run linter
lint:
	@echo "Running linter..."
	GOROOT=$$($(GO) env GOROOT) PATH="$$($(GO) env GOPATH)/bin:$$($(GO) env GOROOT)/bin:$$PATH" golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	goimports -w .

# Install development tools
install-tools:
	@echo "Installing tools..."
	$(GO) install github.com/evilmartians/lefthook@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing golangci-lint from source..."
	GOPROXY=https://proxy.golang.org,direct $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

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
