# schema_registry

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/schema_registry.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/schema_registry)

A Go client for [Confluent Schema Registry](https://docs.confluent.io/platform/current/schema-registry/index.html) with
built-in caching, Avro/Protobuf/JSON serializers, and optional observability hooks.

## Features

- HTTP client for Confluent Schema Registry
- Schema registration and retrieval with in-memory caching
- Compatibility checking for schema evolution
- Confluent wire format encoding/decoding (magic byte + schema ID header)
- Serializers/deserializers for Avro, Protobuf, and JSON Schema
- Generic `WrapperSerializer` / `WrapperDeserializer` for custom formats
- Optional observability via the `observability.Observer` interface
- Optional structured logging via the `Logger` interface
- Uber `fx` module for dependency injection

## Installation

```bash
go get github.com/aalemi-dev/stdlib-lab/schema_registry
```

## Usage

### Direct (without fx)

```go
client, err := schema_registry.NewClient(schema_registry.Config{
    URL:      "http://localhost:8081",
    Username: "user",     // optional
    Password: "pass",     // optional
    Timeout:  10 * time.Second,
})
if err != nil {
    log.Fatal(err)
}

// Register a schema
id, err := client.RegisterSchema("users-value", avroSchema, "AVRO")

// Retrieve a schema by ID
schema, err := client.GetSchemaByID(id)

// Get latest schema for a subject
meta, err := client.GetLatestSchema("users-value")

// Check compatibility
ok, err := client.CheckCompatibility("users-value", newSchema, "AVRO")
```

### With fx

```go
app := fx.New(
    schema_registry.FXModule,
    fx.Provide(func() schema_registry.Config {
        return schema_registry.Config{URL: os.Getenv("SCHEMA_REGISTRY_URL")}
    }),
)
```

### Avro Serialization

```go
serializer, err := schema_registry.NewAvroSerializer(schema_registry.AvroSerializerConfig{
    Registry:    client,
    Subject:     "users-value",
    Schema:      avroSchema,
    MarshalFunc: codec.BinaryFromNative,
})

encoded, err := serializer.Serialize(data)
```

### Wire Format

All serializers produce messages in Confluent wire format:

```
[magic_byte (1 byte 0x00)] [schema_id (4 bytes big-endian)] [payload]
```

```go
header := schema_registry.EncodeSchemaID(42)
id, payload, err := schema_registry.DecodeSchemaID(message)
```
