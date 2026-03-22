# Contributing to Aether

Thank you for your interest in contributing to Aether! This document provides guidelines and instructions for contributing.

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/aether.git
   cd aether
   ```
3. **Create a branch** for your feature or fix:
   ```bash
   git checkout -b feature/my-awesome-feature
   ```

## Development Setup

Make sure you have Go 1.22+ installed.

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run the example application
cd example && go run main.go
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Use meaningful variable and function names.
- All exported functions and types must have doc comments.
- Keep functions focused and small.

## Making Changes

### Adding a New Feature

1. If it's a **core feature** (e.g., a new `Context` method), add it to the appropriate file in the root package.
2. If it's a **middleware**, create a new file in the `middlewares/` directory.
3. Always add corresponding **unit tests**.
4. Update relevant **documentation** in `docs/`.

### Writing Middlewares

Follow the existing middleware pattern:

```go
package middlewares

import (
    "github.com/DantDev2102/aether"
)

type MyMiddlewareConfig struct {
    // Configuration fields
}

func MyMiddleware[T any](cfg MyMiddlewareConfig) aether.HandlerFunc[T] {
    return func(c *aether.Context[T]) {
        // Pre-processing logic
        c.Next()
        // Post-processing logic
    }
}
```

### Writing Tests

- Use the standard `testing` package and `net/http/httptest`.
- Test names should be descriptive: `TestFeatureName_Scenario`.
- Cover both success and error paths.

```go
func TestMyFeature(t *testing.T) {
    app := newTestApp()
    r := app.Router()

    aether.Get(r, "/test", func(c *aether.Context[testGlobal]) {
        c.String(http.StatusOK, "ok")
    })

    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
    }
}
```

## Submitting Changes

1. **Commit** your changes with a clear message:
   ```bash
   git commit -m "feat: add WebSocket support to Context"
   ```
2. **Push** your branch:
   ```bash
   git push origin feature/my-awesome-feature
   ```
3. Open a **Pull Request** against the `main` branch.

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix   | Usage                          |
|----------|--------------------------------|
| `feat:`  | New feature                    |
| `fix:`   | Bug fix                        |
| `docs:`  | Documentation only             |
| `test:`  | Adding or updating tests       |
| `refactor:` | Code change with no new feature or fix |
| `chore:` | Build process or tooling       |

## Pull Request Guidelines

- Keep PRs focused on a single feature or fix.
- Include tests for new functionality.
- Update documentation if the public API changes.
- Make sure all existing tests pass.
- Add a clear description of what the PR does and why.

## Reporting Issues

When reporting bugs, please include:

- Go version (`go version`)
- Aether version
- Steps to reproduce
- Expected vs actual behavior
- Error messages or stack traces

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
