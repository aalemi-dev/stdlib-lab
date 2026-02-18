# observability

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/observability.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/observability)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/observability)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/observability)

A unified observability interface for Go infrastructure packages. Provides a single `Observer` interface that decouples
infrastructure packages (Kafka, Postgres, MinIO, RabbitMQ, etc.) from any specific metrics, tracing, or logging
implementation.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/observability
```

## Usage

### Implement the Observer interface

```go
import "github.com/aalemi-dev/stdlib-lab/observability"

type MetricsObserver struct{}

func (o *MetricsObserver) ObserveOperation(ctx observability.OperationContext) {
fmt.Printf("[%s] %s on %s took %s (err: %v)\n",
ctx.Component, ctx.Operation, ctx.Resource, ctx.Duration, ctx.Error)
}
```

### Pass it to any infrastructure package

```go
cfg := kafka.Config{
Brokers:  []string{"localhost:9092"},
Topic:    "events",
Observer: &MetricsObserver{},
}
```

### No-op observer (useful in tests)

```go
observer := observability.NewNoOpObserver()
```

### Multi-purpose observer

A single observer can handle metrics, tracing, and logging together:

```go
type CompositeObserver struct {
metrics *prometheus.Registry
tracer  tracer.Tracer
logger  logger.Logger
}

func (o *CompositeObserver) ObserveOperation(ctx observability.OperationContext) {
// metrics
o.metrics.RecordDuration(ctx.Component, ctx.Operation, ctx.Duration)

// tracing
_, span := o.tracer.StartSpan(context.Background(), ctx.Component+"."+ctx.Operation)
span.SetAttributes(map[string]interface{}{"resource": ctx.Resource})
if ctx.Error != nil {
span.RecordError(ctx.Error)
}
span.End()

// logging
if ctx.Error != nil {
o.logger.Error("operation failed", ctx.Error, map[string]interface{}{
"component": ctx.Component,
"operation": ctx.Operation,
})
}
}
```

## OperationContext fields

| Field         | Type                     | Description                                             |
|---------------|--------------------------|---------------------------------------------------------|
| `Component`   | `string`                 | Infrastructure package name (`postgres`, `kafka`, etc.) |
| `Operation`   | `string`                 | Action performed (`insert`, `produce`, `put`, etc.)     |
| `Resource`    | `string`                 | Primary resource (table, topic, bucket, etc.)           |
| `SubResource` | `string`                 | Secondary resource (key, partition, routing key, etc.)  |
| `Duration`    | `time.Duration`          | How long the operation took                             |
| `Error`       | `error`                  | Error returned by the operation, `nil` on success       |
| `Size`        | `int64`                  | Data size (rows affected, bytes transferred, etc.)      |
| `Metadata`    | `map[string]interface{}` | Additional operation-specific context                   |

## Examples by infrastructure type

**Kafka:**

```go
observability.OperationContext{
Component:   "kafka",
Operation:   "produce",
Resource:    "user-events",
SubResource: "3", // partition
Duration:    12 * time.Millisecond,
Size:        2048,
Metadata:    map[string]interface{}{"offset": 12345},
}
```

**Postgres:**

```go
observability.OperationContext{
Component: "postgres",
Operation: "insert",
Resource:  "users",
Duration:  23 * time.Millisecond,
Size:      1, // rows affected
}
```

**MinIO:**

```go
observability.OperationContext{
Component:   "minio",
Operation:   "put",
Resource:    "uploads",
SubResource: "files/123/data.json",
Duration:    145 * time.Millisecond,
Size:        1024000,
}
```

## Design

- **Optional** — infrastructure packages work without an observer; it is always a pointer/interface field that defaults
  to `nil`
- **Unified** — one interface covers all infrastructure types
- **Zero dependencies** — the package has no external dependencies
- **Thread-safe** — observer implementations must be safe for concurrent use

## Testing

```sh
go test ./...
```

## Requirements

- Go 1.25+

## License

[MIT](../LICENSE)
