# tracer

[![Go Reference](https://pkg.go.dev/badge/github.com/Abolfazl-Alemi/stdlib-lab/tracer.svg)](https://pkg.go.dev/github.com/Abolfazl-Alemi/stdlib-lab/tracer)
[![Go Report Card](https://goreportcard.com/badge/github.com/Abolfazl-Alemi/stdlib-lab/tracer)](https://goreportcard.com/report/github.com/Abolfazl-Alemi/stdlib-lab/tracer)

Distributed tracing for Go, built on [OpenTelemetry](https://opentelemetry.io/)
with [OTLP HTTP](https://opentelemetry.io/docs/specs/otlp/) export and [Uber fx](https://github.com/uber-go/fx) support.

## Installation

```sh
go get github.com/Abolfazl-Alemi/stdlib-lab/tracer
```

## Usage

### Direct usage

```go
import (
    "context"
    "github.com/Abolfazl-Alemi/stdlib-lab/tracer"
)

client, err := tracer.NewClient(tracer.Config{
    ServiceName:  "my-service",
    AppEnv:       "production",
    EnableExport: true,
})
if err != nil {
    log.Fatal(err)
}

// Create a span — always defer End() immediately
ctx, span := client.StartSpan(context.Background(), "process-request")
defer span.End()

// Add attributes
span.SetAttributes(map[string]interface{}{
    "user.id":    "123",
    "request.id": "abc-xyz",
})

// Record errors
if err != nil {
    span.RecordError(err)
    return err
}
```

### fx integration

The `FXModule` provides both `*TracerClient` and the `Tracer` interface into the fx container. A `tracer.Config` must be
provided separately.

```go
import (
    "github.com/Abolfazl-Alemi/stdlib-lab/tracer"
    "go.uber.org/fx"
)

app := fx.New(
    tracer.FXModule,
    fx.Provide(func() tracer.Config {
        return tracer.Config{
            ServiceName:  "my-service",
            AppEnv:       "production",
            EnableExport: true,
        }
    }),
    fx.Invoke(func(t tracer.Tracer) {
        ctx, span := t.StartSpan(context.Background(), "app-startup")
        defer span.End()
    }),
)
app.Run()
```

### Using the interface

Depend on `tracer.Tracer` rather than `*TracerClient` to keep your code decoupled and testable:

```go
import "github.com/Abolfazl-Alemi/stdlib-lab/tracer"

type MyService struct {
    tracer tracer.Tracer
}

func NewMyService(t tracer.Tracer) *MyService {
    return &MyService{tracer: t}
}

func (s *MyService) Process(ctx context.Context) error {
    ctx, span := s.tracer.StartSpan(ctx, "process")
    defer span.End()
    // ...
    return nil
}
```

### Type aliases (recommended for consumer code)

```go
package observability

import stdTracer "github.com/Abolfazl-Alemi/stdlib-lab/tracer"

type Tracer = stdTracer.Tracer
type Span   = stdTracer.Span
```

## Distributed tracing across services

Trace context is propagated using [W3C Trace Context](https://www.w3.org/TR/trace-context/) headers (`traceparent`,
`tracestate`).

**Outgoing request (inject)**

```go
ctx, span := client.StartSpan(ctx, "call-downstream")
defer span.End()

headers := client.GetCarrier(ctx)
for k, v := range headers {
    req.Header.Set(k, v)
}
```

**Incoming request (extract)**

```go
func httpHandler(w http.ResponseWriter, r *http.Request) {
    headers := make(map[string]string)
    for k, v := range r.Header {
        if len(v) > 0 {
            headers[k] = v[0]
        }
    }

    ctx := client.SetCarrierOnContext(r.Context(), headers)

    ctx, span := client.StartSpan(ctx, "handle-request")
    defer span.End()
}
```

## Configuration

| Field          | Default | Description                                                                                                                                                         |
|----------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ServiceName`  | `""`    | Identifies the service in traces. Required.                                                                                                                         |
| `AppEnv`       | `""`    | Deployment environment (`development`, `staging`, `production`). Sets `deployment.environment` and `environment` resource attributes.                               |
| `EnableExport` | `false` | When `true`, exports traces via OTLP HTTP to a configured collector. When `false`, tracing is functional for context propagation but spans are not sent externally. |

## Span attributes

`SetAttributes` handles type conversion automatically:

| Go type   | OTel attribute type   |
|-----------|-----------------------|
| `string`  | String                |
| `int`     | Int                   |
| `int64`   | Int64                 |
| `float64` | Float64               |
| `bool`    | Bool                  |
| other     | String (`fmt.Sprint`) |

## Testing

Run the test suite:

```sh
go test ./...
```

Run with the race detector and coverage:

```sh
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

All tests run in parallel and require no external dependencies (no running collector needed).

## Requirements

- Go 1.25+
- `go.opentelemetry.io/otel` v1.40+
- `go.opentelemetry.io/otel/sdk` v1.40+
- `go.uber.org/fx` v1.24+ _(only for fx integration)_

## License

[MIT](../LICENSE)
