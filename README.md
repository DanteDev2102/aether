# ⚡ Aether

A blazing-fast, type-safe Go web framework built on top of `net/http` with first-class Generics support.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)

## Features

- 🧬 **Generics-first** — Type-safe global state, handlers, and middleware chains via `Context[T]`
- 🔌 **Pluggable engines** — Bring your own JSON, XML, Template, and Cache engines
- 🔒 **Security built-in** — Helmet, CORS, CSRF, Rate Limiter, JWT, and Basic Auth middlewares
- ⚡ **Zero-alloc routing** — Uses Go 1.22+ `http.ServeMux` with `sync.Pool` context recycling
- 📡 **SSE & WebSocket ready** — Native support via `http.ResponseController` and `Hijack()`
- ⏰ **Cron jobs** — Built-in scheduler with graceful shutdown
- 🗂️ **net/http compatible** — Wrap any standard `func(http.Handler) http.Handler` middleware
- 🧪 **Testable** — Works directly with `httptest` out of the box
- 🪵 **High-performance logger** — Async buffered logging with file output support

## Quick Start

```bash
go get github.com/DantDev2102/aether
```

```go
package main

import (
    "net/http"
    "github.com/DantDev2102/aether"
)

type AppState struct {
    Version string
}

func main() {
    app := aether.New(&aether.Config[AppState]{
        Port: 8080,
        Global: AppState{Version: "1.0.0"},
    })

    r := app.Router()

    aether.Get(r, "/", func(c *aether.Context[AppState]) {
        c.JSON(http.StatusOK, map[string]string{
            "message": "Hello from Aether!",
            "version": c.Global.Version,
        })
    })

    app.Listen()
}
```

## Routing

```go
r := app.Router()

// Simple routes
aether.Get(r, "/users", listUsers)
aether.Delete(r, "/users/{id}", deleteUser)

// Routes with automatic body binding
aether.Post[AppState, CreateUserBody](r, "/users", createUser)
aether.Put[AppState, UpdateUserBody](r, "/users/{id}", updateUser)
aether.Patch[AppState, PatchUserBody](r, "/users/{id}", patchUser)

// Route groups
api := aether.NewGroup("/api/v1", r)
aether.Get(api, "/health", healthCheck)

// Static files
aether.Static(r, "/assets/", "./public")
```

## Context Helpers

```go
// Path & query parameters
id := c.Param("id")
page := c.Query("page")

// Responses
c.JSON(200, data)
c.XML(200, data)
c.String(200, "Hello %s", name)
c.Render(200, "template_name", data)   // Requires configured TemplateEngine

// Files
c.File("/path/to/file.pdf")
c.Attachment("/path/to/file.pdf", "download.pdf")

// Cookies
c.SetCookie(&http.Cookie{Name: "session", Value: "abc"})
cookie, _ := c.Cookie("session")
c.ClearCookie("session")

// SSE
rc, _ := c.SSE()
fmt.Fprintf(c.Res(), "data: hello\n\n")
rc.Flush()

// WebSocket (via Hijack)
conn, bufrw, _ := c.Hijack()

// Cache
c.Cache().Set(ctx, "key", "value")
val, ok := c.Cache().Get(ctx, "key")

// Request/Response access for net/http interop
req := c.Req()
res := c.Res()
```

## Middlewares

### Built-in (root package)

Recovery and Logger middlewares are automatically applied.

### Middleware Package

```go
import "github.com/DantDev2102/aether/middlewares"

// CORS
r.Use(middlewares.CORSMiddleware[AppState](middlewares.CORSConfig{
    AllowOrigins: []string{"https://example.com"},
}))

// Helmet (Security Headers)
r.Use(middlewares.HelmetMiddleware[AppState](middlewares.DefaultHelmetConfig()))

// Rate Limiter
r.Use(middlewares.RateLimiterMiddleware[AppState](middlewares.RateLimiterConfig{
    Limit:  100,
    Window: time.Minute,
}))

// JWT Authentication
r.Use(middlewares.JWTMiddleware[AppState](middlewares.JWTConfig{
    Secret: []byte("your-secret-key"),
}))

// Basic Auth
r.Use(middlewares.BasicAuthMiddleware[AppState](middlewares.BasicAuthConfig{
    Users: map[string]string{"admin": "password"},
}))

// CSRF Protection
r.Use(middlewares.CSRFMiddleware[AppState](middlewares.CSRFConfig{}))

// Request ID
r.Use(middlewares.RequestIDMiddleware[AppState]())

// Gzip Compression
r.Use(middlewares.GzipMiddleware[AppState]())
```

### Using net/http Middlewares

```go
import "github.com/gorilla/handlers"

r.Use(aether.WrapMiddleware[AppState](handlers.RecoveryHandler()))
```

## Cron Jobs

```go
app.AddCron("cleanup", 1*time.Hour, func(ctx context.Context, log aether.Logger) {
    log.Info("Running cleanup...")
})
```

## Cache

```go
// Use the default Otter in-memory cache
store, _ := aether.NewOtterStore(10000)

app := aether.New(&aether.Config[AppState]{
    Cache: store,
})

// Or implement your own CacheStore interface
type CacheStore interface {
    Get(ctx context.Context, key string) (any, bool)
    Set(ctx context.Context, key string, value any) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

## Custom Engines

```go
app := aether.New(&aether.Config[AppState]{
    JSON:     myCustomJSONEngine{},     // implements JSONEngine
    XML:      myCustomXMLEngine{},      // implements XMLEngine
    Template: myHandlebarsEngine{},     // implements TemplateEngine
    Cache:    myRedisStore{},           // implements CacheStore
    Logger:   myZapLogger{},            // implements Logger
})
```

## Documentation

Full documentation is available in the [`docs/`](docs/) directory:

- [Getting Started](docs/getting_started.md)
- [Routing](docs/routing.md)
- [Context](docs/context.md)
- [Middlewares](docs/middlewares.md)
- [Cache](docs/cache.md)
- [Cron Jobs](docs/cron.md)
- [SSE & WebSockets](docs/realtime.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
