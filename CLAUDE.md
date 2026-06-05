# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository shape

`stdlib-lab` is a **Go workspace** (`go.work`) containing one independent Go module per directory. Each top-level folder (`logger`, `tracer`, `observability`, `metrics`, `kafka`, `mariadb`, `minio`, `postgres`, `schema_registry`) has its own `go.mod`, `CHANGELOG.md`, and is versioned and released independently via release-please with the tag format `<component>/vX.Y.Z`.

Implications:
- When adding a dependency, edit the `go.mod` of the specific module, not a root one.
- When running Go commands (`go test`, `go build`, `go vet`) you must `cd` into the module directory; the root has no module.
- The Makefile iterates module directories by looking for `go.mod` and skips anything without one. New packages only need a `go.mod` and an entry in `go.work` + `release-please-config.json` to be picked up.
- Target Go version is **1.25+** (see `.tool-versions`).

## Common commands

All workflows go through the root `Makefile`. `PKG=` accepts a single package or a comma-separated list; omitting it runs across all modules.

```bash
make install-tools                        # lefthook, goimports, gomarkdoc, golangci-lint
make test                                 # all modules, coverage enforced at 80%
make test PKG=kafka                       # single module
make test PKG=kafka,schema_registry       # subset
make lint PKG=logger                      # golangci-lint v2 (auto-installs if missing)
make fmt                                  # goimports -w .
make docs                                 # regenerate docs/<pkg>.md via gomarkdoc
make pr                                   # push branch + open PR (branch `type/short-desc` → "type: short desc")
```

Running a single test inside a module:

```bash
cd kafka && go test -run TestKafkaPublish -race -v ./...
```

To skip Docker-backed integration tests: `go test -short ./...` (integration tests guard on `testing.Short()`).

## Testcontainers / Docker

`kafka`, `mariadb`, `minio`, and `postgres` spin up real services via `testcontainers-go`. The `test` target auto-resolves the Docker socket (precedence: `DOCKER_HOST` → `docker context` → Rancher Desktop → Colima → `/var/run/docker.sock`) and sets `TESTCONTAINERS_RYUK_DISABLED=true` for these four packages only. If you invoke `go test` directly instead of `make test`, export `DOCKER_HOST` yourself on setups where the default socket path is wrong (common on macOS with Rancher/Colima).

## Architectural spine: the `observability.Observer` interface

The infrastructure packages (`kafka`, `mariadb`, `minio`, `postgres`, `schema_registry`) are intentionally **decoupled from concrete observability backends**. Instead they accept an optional `observability.Observer` and call `ObserveOperation(OperationContext)` after each operation. This is the pattern that ties the repo together:

- `observability/interface.go` defines `Observer` and the generic `OperationContext` (Component, Operation, Resource, Duration, Error, Size, Metadata). All infra packages emit this struct — they never import `metrics`, `tracer`, or `logger` directly.
- Consumers wire a custom `Observer` implementation that fans out to whichever of `metrics` / `tracer` / `logger` they want. The `observability` package is the only shared dependency between infra and observability implementations.
- When adding a new infra package or a new operation, follow the existing pattern: accept `observability.Observer` as an `optional:"true"` fx dependency, call `ObserveOperation` on every terminal operation with a consistent `Component` string.

## Fx dependency injection pattern

Every package (except `observability`) exposes an `fx.Module` named `FXModule` in `fx_module.go` following the same shape:

1. `fx.Provide` the concrete client (e.g. `NewClientWithDI`) — which takes a `*Params` struct embedding `fx.In` with optional dependencies (`Logger`, `Observer`, serializers).
2. `fx.Annotate` the concrete type so it's also bindable to the package's exported interface (e.g. `Client`, `Logger`).
3. `fx.Invoke(RegisterXxxLifecycle)` to register shutdown hooks (Sync, Close, etc.).

When adding a new infra package, mirror this layout exactly — the consistency matters because users compose modules together and expect the same provide/annotate/invoke surface everywhere.

## Lint / formatting conventions

`.golangci.yml` is v2-style. Notable settings:
- `goimports.local-prefixes = github.com/docket-legal/go-std-libs` — keep intra-repo imports in their own group.
- `gosec` is disabled in `_test.go` files; `unparam` is too.
- `nakedret` max-func-lines is 30; `misspell` locale is US.
- `gosec/G104` is excluded (errcheck handles it).

Do not silence linters inline without cause — prefer fixing.

## Release / PR conventions

- Commits follow Conventional Commits (see `.github/actions/conventional-commits`). release-please parses these per-component to derive version bumps.
- `make pr` expects branch names shaped `type/short-description` and derives the PR title `type: short description`. It pushes to the `upstream` remote and uses the `aalemi-dev` gh auth user.
- Release-please operates per-component with separate PRs and tags of the form `<component>/vX.Y.Z`. Adding a new top-level module requires an entry in `.github/release-please-config.json`.

## Things that are easy to get wrong

- Editing `go.work` or one module's `go.mod` without the other: if you bump a shared internal dep (e.g. `observability`), every consuming module's `go.mod` needs its `require` updated to the released version — internal modules are consumed via the public module path, not a `replace` directive.
- Running `go test ./...` from the repo root — there is no module there. Always `cd` into a package.
- Assuming `make test` without Docker works for all packages — the four testcontainer-backed modules will fail; use `PKG=` to target the pure-Go ones or pass `-short` when invoking `go test` directly.
