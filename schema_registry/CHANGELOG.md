# Changelog

## [1.1.0](https://github.com/aalemi-dev/stdlib-lab/compare/schema_registry/v1.0.0...schema_registry/v1.1.0) (2026-02-21)


### Features

* add schema registry pkg ([6b05226](https://github.com/aalemi-dev/stdlib-lab/commit/6b052260c4503a9556b726f4dda79b988bee2fe3))
* **makefile:** enhance test and lint commands with package filtering ([e06bbd2](https://github.com/aalemi-dev/stdlib-lab/commit/e06bbd2f994ef3b16a10ae40173e8df9bab36324))
* **schema_registry:** add support for Avro/Protobuf/JSON serializers ([7dc2566](https://github.com/aalemi-dev/stdlib-lab/commit/7dc2566fefddfbc696edb56481f5cff7cd82f3d3))

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
