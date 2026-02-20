# mariadb

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/mariadb.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/mariadb)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/mariadb)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/mariadb)

A robust MariaDB/MySQL client built on top of [GORM](https://gorm.io/), with connection pooling, automatic reconnection,
transaction support, migrations, and optional observability hooks.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/mariadb
```

## Usage

### Basic connection

```go
import "github.com/aalemi-dev/stdlib-lab/mariadb"

db, err := mariadb.NewMariaDB(mariadb.Config{
    Connection: mariadb.Connection{
        Host:      "localhost",
        Port:      "3306",
        User:      "root",
        Password:  "password",
        DbName:    "mydb",
        Charset:   "utf8mb4",
        ParseTime: true,
    },
})
if err != nil {
    log.Fatal(err)
}
defer db.GracefulShutdown()
```

### CRUD operations

```go
// Create
err := db.Create(ctx, &user)

// Query all
var users []User
err = db.Find(ctx, &users, "active = ?", true)

// Query one
var user User
err = db.First(ctx, &user, "email = ?", "alice@example.com")

// Update
rowsAffected, err := db.Update(ctx, &user, map[string]interface{}{"name": "Alice"})

// Delete
rowsAffected, err := db.Delete(ctx, &User{}, "id = ?", 42)
```

### Query builder

```go
var users []User
err := db.Query(ctx).
    Where("age > ?", 18).
    Where("status = ?", "active").
    Order("created_at DESC").
    Limit(10).
    Find(&users)
```

### Transactions

```go
err = db.Transaction(ctx, func(tx mariadb.Client) error {
    if err := tx.Create(ctx, &user); err != nil {
        return err // triggers rollback
    }
    return tx.Create(ctx, &profile)
})
```

### Migrations

```go
// Auto-migrate GORM models
err = db.AutoMigrate(ctx, &User{}, &Order{})

// File-based migrations
err = db.MigrateUp(ctx, "./migrations")
err = db.MigrateDown(ctx, "./migrations", 1)
```

### Observability

```go
import "github.com/aalemi-dev/stdlib-lab/observability"

db, err := mariadb.NewMariaDB(cfg)
if err != nil {
    return err
}
db = db.WithObserver(myObserver).WithLogger(myLogger)
```

The observer receives an `observability.OperationContext` after every operation:

| Field       | Example value                                        |
|-------------|------------------------------------------------------|
| `Component` | `"mariadb"`                                          |
| `Operation` | `"find"`, `"create"`, `"update"`, `"transaction"`, … |
| `Resource`  | table name                                           |
| `Duration`  | operation duration                                   |
| `Error`     | `nil` on success                                     |
| `Size`      | rows affected / count                                |

### FX integration

```go
app := fx.New(
    mariadb.FXModule,
    fx.Provide(
        func() mariadb.Config { return loadConfig() },
        // Optional:
        func() observability.Observer { return myObserver },
    ),
)
app.Run()
```

## Error handling

Methods return GORM errors directly, so you can use `errors.Is`:

```go
var user User
err := db.First(ctx, &user, "email = ?", "alice@example.com")
if errors.Is(err, gorm.ErrRecordNotFound) {
    // handle not found
}
```

Use `TranslateError` for package-standardized error types:

```go
if errors.Is(db.TranslateError(err), mariadb.ErrRecordNotFound) {
    // handle not found
}
```

## Configuration reference

| Field                               | Description                                     | Default     |
|-------------------------------------|-------------------------------------------------|-------------|
| `Connection.Host`                   | Database host                                   | —           |
| `Connection.Port`                   | Database port                                   | —           |
| `Connection.User`                   | Username                                        | —           |
| `Connection.Password`               | Password                                        | —           |
| `Connection.DbName`                 | Database name                                   | —           |
| `Connection.Charset`                | Character set                                   | `"utf8mb4"` |
| `Connection.ParseTime`              | Parse DATE/DATETIME to `time.Time`              | `false`     |
| `Connection.Loc`                    | Timestamp location                              | `"Local"`   |
| `Connection.TLS`                    | TLS mode (`"false"`, `"true"`, `"skip-verify"`) | `"false"`   |
| `ConnectionDetails.MaxOpenConns`    | Max open connections                            | `50`        |
| `ConnectionDetails.MaxIdleConns`    | Max idle connections                            | `25`        |
| `ConnectionDetails.ConnMaxLifetime` | Max connection lifetime                         | `1m`        |

## Testing

```sh
go test ./...
```

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) and require a Docker daemon.

## Requirements

- Go 1.25+
- Docker (for integration tests)

## License

[MIT](../LICENSE)
