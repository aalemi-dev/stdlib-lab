# metrics

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/metrics.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/metrics)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/metrics)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/metrics)

Prometheus metrics for Go, with dual-endpoint separation between system and application metrics, automatic service
labelling, and [Uber fx](https://github.com/uber-go/fx) support.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/metrics
```

## Usage

### Direct usage

```go
import "github.com/aalemi-dev/stdlib-lab/metrics"

m := metrics.NewMetrics(metrics.Config{
    ServiceName: "my-service",
})

// System metrics available at :9090/metrics
// Application metrics available at :9091/metrics
go m.SystemServer.ListenAndServe()
go m.ApplicationServer.ListenAndServe()
```

### Creating metrics

```go
// Counter — increments only
requests := m.CreateCounter("http_requests_total", "Total HTTP requests", []string{"method", "status"})
requests.WithLabelValues("GET", "200").Inc()
requests.WithLabelValues("POST", "500").Add(3)

// Gauge — can go up and down
connections := m.CreateGauge("active_connections", "Active connections", []string{"pool"})
connections.WithLabelValues("postgres").Set(42)
connections.WithLabelValues("postgres").Inc()
connections.WithLabelValues("postgres").Dec()

// Histogram — tracks distributions
duration := m.CreateHistogram(
    "request_duration_seconds",
    "Request duration",
    []string{"endpoint"},
    []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
)
duration.WithLabelValues("/api/search").Observe(0.042)

// Summary — streaming quantiles
latency := m.CreateSummary(
    "api_latency_seconds",
    "API latency",
    []string{"endpoint"},
    map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
)
latency.WithLabelValues("/api/users").Observe(0.013)
```

### fx integration

```go
import (
    "github.com/aalemi-dev/stdlib-lab/metrics"
    "go.uber.org/fx"
)

app := fx.New(
    metrics.FXModule,
    fx.Provide(func() metrics.Config {
        return metrics.Config{
            ServiceName:               "my-service",
            SystemMetricsAddress:      ":9090",
            ApplicationMetricsAddress: ":9091",
        }
    }),
    fx.Invoke(func(m metrics.MetricsCollector) {
        requests := m.CreateCounter("requests_total", "Total requests", []string{"endpoint"})
        requests.WithLabelValues("/api/search").Inc()
    }),
)
app.Run()
```

### Disabling an endpoint

```go
cfg := metrics.Config{
    ServiceName:               "my-service",
    SystemMetricsAddress:      metrics.Ptr(""),  // disable system metrics
    ApplicationMetricsAddress: nil,              // use default :9091
}
```

## Configuration

| Field                       | Env var                       | Default   | Description                                                    |
|-----------------------------|-------------------------------|-----------|----------------------------------------------------------------|
| `ServiceName`               | `METRICS_SERVICE_NAME`        | `""`      | Added as `service` label on all metrics                        |
| `SystemMetricsAddress`      | `METRICS_SYSTEM_ADDRESS`      | `":9090"` | Address for Go runtime/process metrics. Set to `""` to disable |
| `ApplicationMetricsAddress` | `METRICS_APPLICATION_ADDRESS` | `":9091"` | Address for application metrics. Set to `""` to disable        |

## Endpoints

| Endpoint                 | Default port | Contents                                             |
|--------------------------|--------------|------------------------------------------------------|
| `/metrics` (system)      | `:9090`      | Go runtime, process stats, build info                |
| `/metrics` (application) | `:9091`      | User-defined counters, gauges, histograms, summaries |

## Metric types

| Type        | Use case                                 | Methods                                                           |
|-------------|------------------------------------------|-------------------------------------------------------------------|
| `Counter`   | Totals that only increase                | `Inc()`, `Add(float64)`                                           |
| `Gauge`     | Values that go up and down               | `Set()`, `Inc()`, `Dec()`, `Add()`, `Sub()`, `SetToCurrentTime()` |
| `Histogram` | Distributions with server-side quantiles | `Observe(float64)`                                                |
| `Summary`   | Streaming client-side quantiles          | `Observe(float64)`                                                |

All metric types support `WithLabelValues(lvs ...string)` to select a specific label combination before recording.

## Requirements

- Go 1.25+
- `github.com/prometheus/client_golang` v1.23+
- `go.uber.org/fx` v1.24+ _(only for fx integration)_
- `github.com/aalemi-dev/stdlib-lab/logger` _(only for fx integration)_

## License

[MIT](../LICENSE)
