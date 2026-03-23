# Getting Started with Aether

## Installation

```bash
go get github.com/DanteDev2102/aether
```

Requires **Go 1.22+** (uses the new `http.ServeMux` pattern matching).

## Your First App

```go
package main

import (
    "net/http"
    "github.com/DanteDev2102/aether"
)

type AppState struct{}

func main() {
    app := aether.New(&aether.Config[AppState]{
        Port: 8080,
    })

    r := app.Router()

    aether.Get(r, "/", func(c *aether.Context[AppState]) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello, Aether!",
        })
    })

    app.Listen()
}
```

## Configuration

| Field             | Type               | Default     | Description                          |
|-------------------|--------------------|-------------|--------------------------------------|
| `Port`            | `int`              | `0`         | Port to listen on                    |
| `Host`            | `string`           | `""`        | Host address                         |
| `Timeout`         | `int`              | `0`         | Request timeout in seconds           |
| `ShutdownTimeout` | `int`              | `10`        | Graceful shutdown timeout in seconds |
| `MaxBodyBytes`    | `int64`            | `2MB`       | Maximum request body size            |
| `JSON`            | `JSONEngine`       | `std json`  | JSON encoder/decoder                 |
| `XML`             | `XMLEngine`        | `std xml`   | XML encoder/decoder                  |
| `Template`        | `TemplateEngine`   | `nil`       | Template renderer                    |
| `Cache`           | `CacheStore`       | `nil`       | Cache backend                        |
| `Logger`          | `Logger`           | `stdLogger` | Logger implementation                |
| `Global`          | `T`                | zero value  | Global state accessible in handlers  |
| `ErrorHandler`    | `CustomErrorHandler` | `nil`     | Custom panic handler                 |

## Global State

Aether uses Go Generics to provide type-safe global state:

```go
type AppState struct {
    DB      *sql.DB
    Version string
}

app := aether.New(&aether.Config[AppState]{
    Global: AppState{
        DB:      myDB,
        Version: "1.0.0",
    },
})

// Access in any handler:
aether.Get(r, "/version", func(c *aether.Context[AppState]) {
    c.String(200, c.Global.Version)
})
```

## Graceful Shutdown

Aether automatically handles `SIGINT` and `SIGTERM` signals, gracefully shutting down the HTTP server and stopping all cron jobs before exiting.
