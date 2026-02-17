# stdlib-lab

Reusable Go modules for observability, messaging, databases, storage, and AI data infrastructure.

## Versioning Model

This repository uses independent Go modules so each package can be released separately.

For example, the logger module is versioned independently with tags like:

```bash
git tag logger/v1.0.0
```

Consumers import the module directly:

```go
import "github.com/Abolfazl-Alemi/stdlib-lab/logger"
```

## Packages

### `logger`

Structured logging built on [Uber Zap](https://github.com/uber-go/zap) with optional OpenTelemetry tracing integration
and [Uber fx](https://github.com/uber-go/fx) support.

**Features:**

- Structured JSON logging with ISO8601 timestamps
- Log levels: `Debug`, `Info`, `Warning`, `Error`, `Fatal`
- Context-aware methods that automatically attach `trace_id` and `span_id`
- fx dependency injection module
- Configurable via environment variables

**Quick start:**

```go
import "github.com/Abolfazl-Alemi/stdlib-lab/logger"

log := logger.NewLoggerClient(logger.Config{
Level:         logger.Info,
ServiceName:   "my-service",
EnableTracing: true,
})

log.Info("Service started", nil)
log.Error("Something failed", err, map[string]interface{}{"retry": 3})
log.InfoWithContext(ctx, "Handling request", nil, map[string]interface{}{"request_id": "abc-123"})
```

**fx integration:**

```go
app := fx.New(
logger.FXModule, // provides *LoggerClient and Logger interface
fx.Provide(func () logger.Config {
return logger.Config{
Level:         logger.Info,
ServiceName:   "my-service",
EnableTracing: true,
}
}),
)
```

**Environment variables:**

| Variable                | Default | Description                                     |
|-------------------------|---------|-------------------------------------------------|
| `ZAP_LOGGER_LEVEL`      | `info`  | Log level (`debug`, `info`, `warning`, `error`) |
| `LOGGER_ENABLE_TRACING` | `false` | Enable OpenTelemetry trace/span injection       |
| `LOGGER_CALLER_SKIP`    | `1`     | Stack frames to skip for caller reporting       |

## Requirements

- Go 1.25+

## License

MIT
