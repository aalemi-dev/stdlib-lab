# logger

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/logger.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/logger)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/logger)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/logger)

Structured logging for Go, built on [Uber Zap](https://github.com/uber-go/zap) with
optional [OpenTelemetry](https://opentelemetry.io/) tracing integration and [Uber fx](https://github.com/uber-go/fx)
support.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/logger
```

## Usage

### Direct usage

```go
import "github.com/aalemi-dev/stdlib-lab/logger"

log := logger.NewLoggerClient(logger.Config{
    Level:         logger.Info,
    ServiceName:   "my-service",
    EnableTracing: true,
})

// Basic logging
log.Info("server started", nil)
log.Warn("high memory usage", nil, map[string]interface{}{"usage_mb": 1024})
log.Error("request failed", err, map[string]interface{}{"path": "/api/users"})

// Context-aware logging — automatically attaches trace_id and span_id
log.InfoWithContext(ctx, "handling request", nil, map[string]interface{}{"request_id": "abc-123"})
log.ErrorWithContext(ctx, "db query failed", err)
```

### fx integration

The `FXModule` provides both `*LoggerClient` and the `Logger` interface into the fx container. A `logger.Config` must be
provided separately.

```go
import (
    "github.com/aalemi-dev/stdlib-lab/logger"
    "go.uber.org/fx"
)

app := fx.New(
    logger.FXModule,
    fx.Provide(func() logger.Config {
        return logger.Config{
            Level:         logger.Info,
            ServiceName:   "my-service",
            EnableTracing: true,
        }
    }),
    fx.Invoke(func(log logger.Logger) {
        log.Info("app initialised", nil)
    }),
)
app.Run()
```

### Using the interface

Depend on the `Logger` interface rather than the concrete `*LoggerClient` to keep your code decoupled:

```go
import logger "github.com/aalemi-dev/stdlib-lab/logger"

type MyService struct {
    log logger.Logger
}

func NewMyService(log logger.Logger) *MyService {
    return &MyService{log: log}
}
```

## Configuration

| Field           | Env var                 | Default | Description                                              |
|-----------------|-------------------------|---------|----------------------------------------------------------|
| `Level`         | `ZAP_LOGGER_LEVEL`      | `info`  | Minimum log level (`debug`, `info`, `warning`, `error`)  |
| `EnableTracing` | `LOGGER_ENABLE_TRACING` | `false` | Inject `trace_id` / `span_id` from OpenTelemetry context |
| `ServiceName`   | —                       | `""`    | Populates the `service` field on every log entry         |
| `CallerSkip`    | `LOGGER_CALLER_SKIP`    | `1`     | Stack frames to skip for accurate caller reporting       |

## Log levels

```go
logger.Debug   // "debug"
logger.Info    // "info"
logger.Warning // "warning"
logger.Error   // "error"
```

## Tracing

When `EnableTracing` is `true`, the `*WithContext` methods extract the active OpenTelemetry span from the context and
attach the following fields to every log entry:

- `trace_id`
- `span_id`

No span context in the context means no fields are added — there is no error or panic.

## Output format

All logs are written to **stderr** as **JSON** with the following standard fields:

| Field       | Description                       |
|-------------|-----------------------------------|
| `timestamp` | ISO8601 timestamp                 |
| `level`     | Log level in capitals (`INFO`, …) |
| `caller`    | Full file path and line number    |
| `message`   | Log message                       |
| `pid`       | Process ID                        |
| `service`   | Value of `Config.ServiceName`     |
| `error`     | Error string (when `err != nil`)  |
| `trace_id`  | OTel trace ID (when tracing on)   |
| `span_id`   | OTel span ID (when tracing on)    |

## Requirements

- Go 1.25+
- `go.uber.org/zap` v1.27+
- `go.uber.org/fx` v1.24+ _(only for fx integration)_
- `go.opentelemetry.io/otel/trace` v1.40+ _(only for tracing)_

## License

[MIT](../LICENSE)
