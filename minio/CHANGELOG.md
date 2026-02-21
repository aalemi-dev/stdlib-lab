# Changelog

## [1.1.0](https://github.com/aalemi-dev/stdlib-lab/compare/minio/v1.0.0...minio/v1.1.0) (2026-02-21)


### Features

* add minio pkg ([3890375](https://github.com/aalemi-dev/stdlib-lab/commit/38903754bc20ae9cf3868c882864f7f667b86a47))
* **minio:** add MinIO/S3-compatible object storage client ([f97c6bd](https://github.com/aalemi-dev/stdlib-lab/commit/f97c6bd2dbb8e9f0789787d8b960a83016486c6a))
* **tests:** add comprehensive integration and unit tests for MinIO client ([ae85749](https://github.com/aalemi-dev/stdlib-lab/commit/ae85749b6a341d3e45f37a35de1a8992b07bb671))


### Bug Fixes

* **configs:** secure sensitive fields in JSON serialization ([6f4b572](https://github.com/aalemi-dev/stdlib-lab/commit/6f4b572484d5c29b8bd6e331dab61cadcc43535e))

## [1.0.0](https://github.com/aalemi-dev/stdlib-lab/releases/tag/minio/v1.0.0) - Initial Release

### Features

* MinIO/S3-compatible object storage client
* Object operations: upload, download, delete, copy, stat
* Presigned URL generation for GET and PUT
* Bucket notification support
* fx lifecycle support for dependency injection
* Observer hooks for operation monitoring
* Connection health checking with automatic reconnection
