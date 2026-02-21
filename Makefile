GO ?= go
SHELL = bash

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

# Run tests with coverage for all modules (sequential)
test:
	@TMPDIR=$$(mktemp -d); \
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)" | sort); do \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		pkgname=$$(basename $$pkg); \
		if [ "$$pkgname" = "kafka" ] || [ "$$pkgname" = "mariadb" ] || [ "$$pkgname" = "minio" ] || [ "$$pkgname" = "postgres" ]; then \
			TEST_CMD="DOCKER_HOST=$(DOCKER_SOCK) TESTCONTAINERS_RYUK_DISABLED=true $(GO) test -v -race -count=1 -coverprofile=$$TMPDIR/$$pkgname.cov -covermode=atomic ./..."; \
		else \
			TEST_CMD="$(GO) test -v -race -count=1 -coverprofile=$$TMPDIR/$$pkgname.cov -covermode=atomic ./..."; \
		fi; \
		echo ""; \
		echo "── $$pkgname ──────────────────────────────────────"; \
		( cd $$pkg && eval "$$TEST_CMD" 2>&1 ) | tee "$$TMPDIR/$$pkgname.out"; \
		echo $${PIPESTATUS[0]} > "$$TMPDIR/$$pkgname.exit"; \
	done; \
	TOTAL_PASS=0; TOTAL_FAIL=0; ANY_ERROR=0; \
	for outfile in $$(ls "$$TMPDIR"/*.out 2>/dev/null | sort); do \
		pkgname=$$(basename "$$outfile" .out); \
		PKG_EXIT=$$(cat "$$TMPDIR/$$pkgname.exit" 2>/dev/null || echo 1); \
		PKG_PASS=$$(grep -c '^--- PASS:' "$$outfile" || true); \
		PKG_FAIL=$$(grep -c '^--- FAIL:' "$$outfile" || true); \
		TOTAL_PASS=$$((TOTAL_PASS + PKG_PASS)); \
		TOTAL_FAIL=$$((TOTAL_FAIL + PKG_FAIL)); \
		grep '^--- FAIL:' "$$outfile" | awk '{print $$3}' | while read -r name; do \
			echo "    $$pkgname/$$name"; \
		done >> "$$TMPDIR/failed_tests"; \
		if [ "$$PKG_EXIT" -eq 0 ] && [ -f "$$TMPDIR/$$pkgname.cov" ]; then \
			COVERAGE=$$($(GO) tool cover -func="$$TMPDIR/$$pkgname.cov" | grep total | awk '{print $$3}' | tr -d '%'); \
			echo "  Coverage: $${COVERAGE}%  (threshold: $(COVERAGE_THRESHOLD)%)"; \
			if [ "$$(echo "$$COVERAGE < $(COVERAGE_THRESHOLD)" | bc -l)" -eq 1 ]; then \
				echo "  ❌ Coverage $${COVERAGE}% is below threshold $(COVERAGE_THRESHOLD)%"; \
				ANY_ERROR=1; \
			else \
				echo "  ✅ Coverage ok"; \
			fi; \
		else \
			[ "$$PKG_EXIT" -ne 0 ] && ANY_ERROR=1; \
		fi; \
	done; \
	TOTAL=$$((TOTAL_PASS + TOTAL_FAIL)); \
	echo ""; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	echo "  Test Summary"; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	printf "  Total:     %d\n" "$$TOTAL"; \
	printf "  ✅ Passed: %d\n" "$$TOTAL_PASS"; \
	printf "  ❌ Failed: %d\n" "$$TOTAL_FAIL"; \
	if [ -s "$$TMPDIR/failed_tests" ]; then \
		echo ""; \
		echo "  Failed tests:"; \
		cat "$$TMPDIR/failed_tests"; \
	fi; \
	echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; \
	rm -rf "$$TMPDIR"; \
	exit $$ANY_ERROR

# Open a pull request against main, deriving the title from the branch name
# Branch format: type/short-description → "type: short description"
pr:
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
	TYPE=$$(echo $$BRANCH | cut -d'/' -f1); \
	DESC=$$(echo $$BRANCH | cut -d'/' -f2- | tr '-' ' '); \
	TITLE="$$TYPE: $$DESC"; \
	echo "Opening PR with title: $$TITLE"; \
	git push upstream HEAD; \
	gh auth switch --user aalemi-dev; \
	gh pr create --title "$$TITLE" --fill --base main --repo aalemi-dev/stdlib-lab

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
	for pkg in $$(find . -maxdepth 1 -mindepth 1 -type d | grep -v -E "^\./(\.|docs|vendor)" | sort); do \
		pkgname=$$(basename $$pkg); \
		if [ ! -f "$$pkg/go.mod" ]; then continue; fi; \
		echo "Processing $$pkgname..."; \
		(cd $$pkg && $$GOMARKDOC ./... \
			--output "../docs/$$pkgname.md" \
			--repository.url "https://github.com/aalemi-dev/stdlib-lab" \
			--format github); \
	done; \
	echo "Documentation generated in docs/ directory"
