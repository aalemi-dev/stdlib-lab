# kafka

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/kafka.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/kafka)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/kafka)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/kafka)

Apache Kafka client for Go, built on [segmentio/kafka-go](https://github.com/segmentio/kafka-go) with pluggable
serialization, SASL/TLS support, observability hooks, and [Uber fx](https://github.com/uber-go/fx) integration.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/kafka
```

## Usage

### Producer

```go
import "github.com/aalemi-dev/stdlib-lab/kafka"

client, err := kafka.NewClient(kafka.Config{
    Brokers: []string{"localhost:9092"},
    Topic:   "events",
})
if err != nil {
    log.Fatal(err)
}
defer client.GracefulShutdown()

err = client.Publish(ctx, "user-123", map[string]string{"event": "login"})
```

### Consumer

```go
client, err := kafka.NewClient(kafka.Config{
    Brokers:    []string{"localhost:9092"},
    Topic:      "events",
    GroupID:    "my-service",
    IsConsumer: true,
})
if err != nil {
    log.Fatal(err)
}
defer client.GracefulShutdown()

wg := &sync.WaitGroup{}
msgs := client.Consume(ctx, wg)

for msg := range msgs {
    var payload MyEvent
    if err := msg.BodyAs(&payload); err != nil {
        log.Println("deserialize error:", err)
        continue
    }
    // process...
    if err := msg.CommitMsg(); err != nil {
        log.Println("commit error:", err)
    }
}
wg.Wait()
```

### Parallel consumer

```go
msgs := client.ConsumeParallel(ctx, wg, 4) // 4 concurrent workers
```

### fx integration

```go
import (
    "github.com/aalemi-dev/stdlib-lab/kafka"
    "go.uber.org/fx"
)

app := fx.New(
    kafka.FXModule,
    fx.Provide(func() kafka.Config {
        return kafka.Config{
            Brokers:    []string{"localhost:9092"},
            Topic:      "events",
            IsConsumer: false,
        }
    }),
    fx.Invoke(func(client kafka.Client) {
        client.Publish(ctx, "key", payload)
    }),
)
app.Run()
```

## Configuration

| Field              | Default            | Description                                                        |
|--------------------|--------------------|--------------------------------------------------------------------|
| `Brokers`          | required           | List of broker addresses                                           |
| `Topic`            | required           | Topic to produce to or consume from                                |
| `GroupID`          | `""`               | Consumer group ID (required for consumers)                         |
| `IsConsumer`       | `false`            | Set `true` to enable consumer mode                                 |
| `MinBytes`         | `1`                | Minimum bytes per fetch request                                    |
| `MaxBytes`         | `10MB`             | Maximum bytes per fetch request                                    |
| `MaxWait`          | `10s`              | Max wait time for `MinBytes`                                       |
| `CommitInterval`   | `1s`               | Auto-commit interval (when `EnableAutoCommit` is true)             |
| `EnableAutoCommit` | `false`            | Auto-commit offsets. `false` = manual commit via `msg.CommitMsg()` |
| `StartOffset`      | `FirstOffset (-2)` | Where to start when no committed offset exists                     |
| `RequiredAcks`     | `RequireAll (-1)`  | Acknowledgment mode: `0` none, `1` leader, `-1` all ISR            |
| `WriteTimeout`     | `10s`              | Producer write timeout                                             |
| `Async`            | `false`            | Async batched writes                                               |
| `BatchSize`        | `100`              | Max messages per async batch                                       |
| `BatchTimeout`     | `1s`               | Max wait time before flushing a batch                              |
| `CompressionCodec` | `""`               | Compression: `gzip`, `snappy`, `lz4`, `zstd`                       |
| `MaxAttempts`      | `10`               | Max delivery attempts                                              |
| `DataType`         | `json`             | Default serializer: `json`, `string`, `bytes`, `gob`               |

## Serialization

The client uses pluggable serializers. The default is JSON. You can override:

```go
client.SetSerializer(&kafka.JSONSerializer{})
client.SetDeserializer(&kafka.JSONDeserializer{})
```

Available types: `JSONSerializer`, `JSONDeserializer`, `StringSerializer`, `StringDeserializer`, `BytesSerializer`,
`BytesDeserializer`, `GobSerializer`, `GobDeserializer`.

## TLS

```go
kafka.Config{
    TLS: kafka.TLSConfig{
        Enabled:        true,
        CACertPath:     "/certs/ca.crt",
        ClientCertPath: "/certs/client.crt",
        ClientKeyPath:  "/certs/client.key",
    },
}
```

## SASL

```go
kafka.Config{
    SASL: kafka.SASLConfig{
        Enabled:   true,
        Mechanism: "SCRAM-SHA-256", // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
        Username:  "user",
        Password:  "pass",
    },
}
```

## Observability

Pass any `observability.Observer` implementation to track all produce/consume operations:

```go
import "github.com/aalemi-dev/stdlib-lab/observability"

client.WithObserver(myObserver)
```

Each operation emits an `observability.OperationContext` with `Component: "kafka"` and `Operation: "produce"` or
`"consume"`.

## Error handling

```go
err := client.Publish(ctx, "key", payload)
translated := client.TranslateError(err)

switch {
case client.IsRetryableError(translated):
    // retry
case client.IsPermanentError(translated):
    // dead-letter or alert
case client.IsAuthenticationError(translated):
    // check credentials
}
```

## Testing

Run unit tests (no Kafka required):

```sh
go test -short ./...
```

Run integration tests (requires Docker):

```sh
go test -race -count=1 ./...
```

Integration tests spin up a Kafka container automatically
via [testcontainers-go](https://github.com/testcontainers/testcontainers-go).

## Requirements

- Go 1.25+
- `github.com/segmentio/kafka-go` v0.4+
- `go.uber.org/fx` v1.24+ _(only for fx integration)_
- Docker _(only for integration tests)_

## License

[MIT](../LICENSE)
