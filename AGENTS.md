# AGENTS.md

This file contains guidelines for agentic coding agents operating in this repository.

## Project Overview

**Aether** is a lightweight Go web framework with routing, middleware, caching, logging, and cron support. It uses generic type parameters throughout (e.g., `App[T any]`, `Context[T any]`) to allow custom global state per application.

## Build, Lint, and Test Commands

All commands can be run via `task <command>` (requires [go-task](https://taskfile.dev/)) or directly with the underlying tool.

### Build

```bash
go build ./...
# or
task build
```

### Test

```bash
# All tests with race detection
go test -race -v ./...

# Single test function
go test -race -v -run TestFunctionName ./...

# Single test file
go test -race -v ./... -run TestFileName

# With coverage report
task test-coverage
```

### Lint

```bash
golangci-lint run ./...
# or
task lint
```

### Security

```bash
govulncheck ./...
# or
task security
```

### Benchmarks

```bash
go test -bench=. -benchmem ./...
# or
task bench
```

### Full Setup

```bash
task setup
```

## Code Style Guidelines

### Formatting

- Code is formatted with **gofumpt** (extra-rules enabled) and **gci** for imports
- Run formatters automatically before committing: `gofumpt -w .` and `gci write .`
- Golangci-lint handles formatting enforcement in CI

### Import Organization

Imports are ordered via gci sections:

1. **Standard library** (e.g., `context`, `net/http`)
2. **Default/third-party** (e.g., `github.com/golang-jwt/jwt/v5`)
3. **Internal prefix** (`github.com/DanteDev2102/aether`)

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"

    "github.com/DanteDev2102/aether"
)
```

### Naming Conventions

- **Exported types/functions**: PascalCase (e.g., `App`, `NewRouter`, `Get`)
- **Unexported types/functions**: camelCase (e.g., `registerHelper`, `responseWriter`)
- **Interfaces**: PascalCase with descriptive names (e.g., `CacheStore`, `JSONEngine`, `Logger`)
- **Config structs**: `<Feature>Config` suffix (e.g., `JWTConfig`, `CORSConfig`)
- **Middleware functions**: `<Feature>Middleware` or `<Feature>Middleware[T any]` suffix
- **Constants**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase; acronyms are kept uppercase (ID, URL, API, HTTP, UUID)

### Documentation

- **All exported** functions, types, constants, and interfaces **must have doc comments**
- Comments start with the name of the element being documented
- Use clear, concise descriptions; no markdown formatting needed

```go
// App represents the main Aether application instance.
type App[T any] struct { ... }

// Get registers a handler for GET requests at the specified path.
func Get[T any](r *Router[T], path string, h HandlerFunc[T]) { ... }
```

### Cyclomatic Complexity

- Maximum complexity: **15** (enforced by gocyclo)
- Break complex functions into smaller, focused helpers

### Error Handling

- Return errors from functions when operations can fail
- Use named returns for functions with multiple error conditions when idiomatic
- Ignore predictable errors with `_` only when safe (e.g., `io.NopCloser`)
- Never silently ignore errors in middleware or critical paths
- Panic recovery is only for truly unrecoverable states (see `recovery.go` pattern)

### Generic Type Parameters

The framework uses Go generics extensively. Follow these patterns:

```go
// Generic app with custom global state
func New[T any](conf *Config[T]) *App[T]

// Generic handler functions
type HandlerFunc[T any] func(c *Context[T])
type HandlerWithBody[T any, B any] func(c *Context[T], body B)

// Generic router routes
func Get[T any](r *Router[T], path string, h HandlerFunc[T])
func Post[T, B any](r *Router[T], path string, h HandlerWithBody[T, B])
```

### Middleware Pattern

All middleware should follow this pattern:

```go
package middlewares

import "github.com/DanteDev2102/aether"

type MyMiddlewareConfig struct {
    // Configuration fields with zero values as defaults
    Option string
}

func MyMiddleware[T any](cfg MyMiddlewareConfig) aether.HandlerFunc[T] {
    return func(c *aether.Context[T]) {
        // Pre-processing
        c.Next()
        // Post-processing (optional)
    }
}
```

### File Structure

- **Root package**: Core framework (`aether.go`, `router.go`, `context.go`, `cache.go`, etc.)
- **middlewares/**: Each middleware in its own file (`jwt.go`, `cors.go`, `rate_limiter.go`, etc.)
- **docs/**: Documentation for each feature
- **example/**: Runnable example application

### Tests

- Use `net/http/httptest` for HTTP testing
- Test names: `Test<Feature>_<Scenario>` format
- Always test both success and error paths
- Use helper `newTestApp()` pattern shown in `aether_test.go`
- Use `t.Errorf` for assertions, `t.Fatalf` for setup failures

```go
func TestGetRoute_Success(t *testing.T) {
    app := newTestApp()
    r := app.Router()

    Get(r, "/hello", func(c *Context[testGlobal]) {
        _ = c.String(http.StatusOK, "Hello, World!")
    })

    req := httptest.NewRequestWithContext(context.Background(), "GET", "/hello", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
    }
}
```

### Commit Messages

Follow **Conventional Commits** (enforced via lefthook):

```
feat: add WebSocket support to Context
fix(auth): resolve JWT validation timeout
docs: update routing documentation
test: add cache integration tests
refactor: simplify middleware chain execution
chore: update dependencies
```

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

## Technical Stack

- **Go**: 1.25.0
- **Dependencies**: `golang-jwt/jwt/v5`, `google/uuid`, `maypok86/otter`
- **Dev tools**: golangci-lint, govulncheck, go-task, lefthook, air

## Pre-commit Hooks

Lefthook is configured to run on commit:

- **pre-commit**: Runs `task lint` on `*.go` files
- **commit-msg**: Validates conventional commit format

Install hooks: `lefthook install`
