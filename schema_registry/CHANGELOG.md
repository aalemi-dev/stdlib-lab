# Changelog

## [1.0.0](https://github.com/aalemi-dev/stdlib-lab/releases/tag/schema_registry/v1.0.0) - Initial Release

### Features

- HTTP client for Confluent Schema Registry with basic auth support
- Schema registration, retrieval, and compatibility checking
- In-memory caching for schemas by ID and subject
- Confluent wire format encoding/decoding
- Avro, Protobuf, and JSON serializers/deserializers
- Generic `WrapperSerializer` and `WrapperDeserializer`
- Optional observability via `observability.Observer`
- Optional structured logging via `Logger` interface
- Uber `fx` module for dependency injection
