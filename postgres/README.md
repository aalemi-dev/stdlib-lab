# postgres

[![Go Reference](https://pkg.go.dev/badge/github.com/aalemi-dev/stdlib-lab/postgres.svg)](https://pkg.go.dev/github.com/aalemi-dev/stdlib-lab/postgres)
[![Go Report Card](https://goreportcard.com/badge/github.com/aalemi-dev/stdlib-lab/postgres)](https://goreportcard.com/report/github.com/aalemi-dev/stdlib-lab/postgres)

A PostgreSQL client built on top of [GORM](https://gorm.io/), with connection pooling, automatic reconnection,
transaction support, migrations, and optional observability hooks.

## Installation

```sh
go get github.com/aalemi-dev/stdlib-lab/postgres
```

## Usage

### Basic connection

```go
import "github.com/aalemi-dev/stdlib-lab/postgres"

pg, err := postgres.NewPostgres(postgres.Config{
    Connection: postgres.Connection{
        Host:     "localhost",
        Port:     "5432",
        User:     "postgres",
        Password: "password",
        DbName:   "mydb",
        SSLMode:  "disable",
    },
})
if err != nil {
    log.Fatal(err)
}
defer pg.GracefulShutdown()
```

### CRUD operations

```go
// Create
err := pg.Create(ctx, &user)

// Query all
var users []User
err = pg.Find(ctx, &users, "active = ?", true)

// Query one
var user User
err = pg.First(ctx, &user, "email = ?", "alice@example.com")

// Update
rowsAffected, err := pg.Update(ctx, &user, map[string]interface{}{"name": "Alice"})

// Delete
rowsAffected, err := pg.Delete(ctx, &User{}, "id = ?", 42)
```

### Query builder

```go
var users []User
err := pg.Query(ctx).
    Where("age > ?", 18).
    Where("status = ?", "active").
    Order("created_at DESC").
    Limit(10).
    Find(&users)
```

### Transactions

```go
err = pg.Transaction(ctx, func(tx postgres.Client) error {
    if err := tx.Create(ctx, &user); err != nil {
        return err // triggers rollback
    }
    return tx.Create(ctx, &profile)
})
```

### Migrations

```go
// Auto-migrate GORM models
err = pg.AutoMigrate(ctx, &User{}, &Order{})

// File-based migrations
err = pg.MigrateUp(ctx, "./migrations")
err = pg.MigrateDown(ctx, "./migrations", 1)
```

### Observability

```go
import "github.com/aalemi-dev/stdlib-lab/observability"

pg, err := postgres.NewPostgres(cfg)
if err != nil {
    return err
}
pg = pg.WithObserver(myObserver).WithLogger(myLogger)
```

The observer receives an `observability.OperationContext` after every operation:

| Field       | Example value                                        |
|-------------|------------------------------------------------------|
| `Component` | `"postgres"`                                         |
| `Operation` | `"find"`, `"create"`, `"update"`, `"transaction"`, … |
| `Resource`  | table name                                           |
| `Duration`  | operation duration                                   |
| `Error`     | `nil` on success                                     |
| `Size`      | rows affected / count                                |

### FX integration

```go
app := fx.New(
    postgres.FXModule,
    fx.Provide(
        func() postgres.Config { return loadConfig() },
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
err := pg.First(ctx, &user, "email = ?", "alice@example.com")
if errors.Is(err, gorm.ErrRecordNotFound) {
    // handle not found
}
```

Use `TranslateError` for package-standardized error types:

```go
if errors.Is(pg.TranslateError(err), postgres.ErrRecordNotFound) {
    // handle not found
}
```

## Configuration reference

| Field                               | Description                                          | Default |
|-------------------------------------|------------------------------------------------------|---------|
| `Connection.Host`                   | Database host                                        | —       |
| `Connection.Port`                   | Database port                                        | —       |
| `Connection.User`                   | Username                                             | —       |
| `Connection.Password`               | Password                                             | —       |
| `Connection.DbName`                 | Database name                                        | —       |
| `Connection.SSLMode`                | SSL mode (`"disable"`, `"require"`, `"verify-full"`) | —       |
| `ConnectionDetails.MaxOpenConns`    | Max open connections                                 | `50`    |
| `ConnectionDetails.MaxIdleConns`    | Max idle connections                                 | `25`    |
| `ConnectionDetails.ConnMaxLifetime` | Max connection lifetime                              | `1m`    |

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
