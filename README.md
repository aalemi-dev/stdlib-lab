# stdlib-lab

A collection of reusable, production-ready Go modules for observability, messaging, databases, storage, and AI data
infrastructure.

## Packages

| Package                                | Description                                                                         | Go Reference                                                                                                                                                          |
|----------------------------------------|-------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`logger`](./logger)                   | Structured logging with OpenTelemetry tracing and fx support                        | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/logger.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/logger)                   |
| [`tracer`](./tracer)                   | Distributed tracing with OpenTelemetry and OTLP HTTP export                         | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/tracer.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/tracer)                   |
| [`observability`](./observability)     | Unified observer interface for infrastructure packages                              | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/observability.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/observability)     |
| [`metrics`](./metrics)                 | Prometheus metrics with dual-endpoint separation and fx support                     | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/metrics.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/metrics)                 |
| [`kafka`](./kafka)                     | Apache Kafka client with serialization, SASL/TLS and fx support                     | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/kafka.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/kafka)                     |
| [`mariadb`](./mariadb)                 | MariaDB/MySQL client with GORM, migrations, and fx support                          | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/mariadb.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/mariadb)                 |
| [`minio`](./minio)                     | MinIO/S3-compatible object storage client with fx support                           | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/minio.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/minio)                     |
| [`postgres`](./postgres)               | PostgreSQL client with GORM, migrations, and fx support                             | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/postgres.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/postgres)               |
| [`schema_registry`](./schema_registry) | Confluent Schema Registry client with Avro/Protobuf/JSON serializers and fx support | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/schema_registry.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/schema_registry) |

## Requirements

- Go 1.25+
- Docker (for packages that use testcontainers: `kafka`, `mariadb`, `minio`, `postgres`)

## Development

### Install tools

```bash
make install-tools
```

### Test

```bash
make test                                # all packages
make test PKG=schema_registry            # single package
make test PKG=schema_registry,kafka      # multiple packages (comma-separated)
```

Coverage threshold is enforced at **80%**. Packages that require Docker (`kafka`, `mariadb`, `minio`, `postgres`) automatically set `DOCKER_HOST` and disable Ryuk.

### Lint

```bash
make lint                                # all packages
make lint PKG=schema_registry            # single package
make lint PKG=schema_registry,logger     # multiple packages (comma-separated)
```

### Format

```bash
make fmt
```

### Docs

```bash
make docs        # generates docs/ from Go source comments
```

### Clean

```bash
make clean       # removes coverage files and test binaries
```

### Open a PR

```bash
make pr          # pushes current branch and opens a PR against main
```

## Contributing

Contributions are welcome. Please open an issue before submitting a pull request for significant changes.

## License

[MIT](./LICENSE)
