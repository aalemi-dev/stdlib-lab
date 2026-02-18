# stdlib-lab

A collection of reusable, production-ready Go modules for observability, messaging, databases, storage, and AI data
infrastructure.

## Packages

| Package                            | Description                                                     | Go Reference                                                                                                                                                      |
|------------------------------------|-----------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`logger`](./logger)               | Structured logging with OpenTelemetry tracing and fx support    | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/logger.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/logger)               |
| [`tracer`](./tracer)               | Distributed tracing with OpenTelemetry and OTLP HTTP export     | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/tracer.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/tracer)               |
| [`observability`](./observability) | Unified observer interface for infrastructure packages          | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/observability.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/observability) |
| [`metrics`](./metrics)             | Prometheus metrics with dual-endpoint separation and fx support | [![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/metrics.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/metrics)             |

## Requirements

- Go 1.25+

## Contributing

Contributions are welcome. Please open an issue before submitting a pull request for significant changes.

## License

[MIT](./LICENSE)
