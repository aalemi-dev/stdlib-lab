# minio

MinIO/S3-compatible object storage client for Go with fx dependency injection support.

## Features

- Object operations: upload, download, delete, copy, move, stat
- Presigned URL generation for GET and PUT with configurable expiry
- Bucket notification support
- Multipart upload handling
- Connection health checking with automatic reconnection
- Observer hooks for operation monitoring
- fx lifecycle integration for clean startup and shutdown

## Installation

```bash
go get github.com/aalemi-dev/stdlib-lab/minio
```

## Usage

### Standalone

```go
import "github.com/aalemi-dev/stdlib-lab/minio"

client, err := minio.NewMinioClient(minio.Config{
    Connection: minio.Connection{
        Endpoint:  "localhost:9000",
        AccessKey: "minioadmin",
        SecretKey: "minioadmin",
        UseSSL:    false,
    },
})
if err != nil {
    log.Fatal(err)
}

// Upload an object
err = client.UploadObject(ctx, "my-bucket", "my-key", reader, size, "application/octet-stream")

// Download an object
object, err := client.GetObject(ctx, "my-bucket", "my-key")

// Generate a presigned GET URL
url, err := client.PresignedGetObject(ctx, "my-bucket", "my-key", 15*time.Minute)
```

### With fx

```go
import (
    "github.com/aalemi-dev/stdlib-lab/minio"
    "go.uber.org/fx"
)

fx.New(
    minio.Module,
    fx.Provide(func() minio.Config {
        return minio.Config{
            Connection: minio.Connection{
                Endpoint:  "localhost:9000",
                AccessKey: "minioadmin",
                SecretKey: "minioadmin",
            },
        }
    }),
).Run()
```

## Configuration

| Field | Type | Description |
|---|---|---|
| `Connection.Endpoint` | `string` | MinIO server endpoint (host:port) |
| `Connection.AccessKey` | `string` | Access key for authentication |
| `Connection.SecretKey` | `string` | Secret key for authentication |
| `Connection.UseSSL` | `bool` | Enable TLS/SSL connection |
| `Connection.Region` | `string` | Storage region (optional) |

## Requirements

- Go 1.25+
- MinIO or any S3-compatible object storage

## License

[MIT](../LICENSE)
