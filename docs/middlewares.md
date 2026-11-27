# Middlewares

Middlewares in Aether are functions that process requests before and/or after the main handler. They follow a chain pattern using `c.Next()`.

## Writing Middlewares

```go
func MyMiddleware[T any]() aether.HandlerFunc[T] {
    return func(c *aether.Context[T]) {
        // Pre-processing
        c.Next()
        // Post-processing
    }
}
```

## Applying Middlewares

```go
// Global (all routes)
r.Use(MyMiddleware[AppState]())

// Group-scoped
api := aether.NewGroup("/api", r)
api.Use(AuthMiddleware[AppState]())
```

## Built-in Middlewares

### Recovery (auto-applied)
Catches panics and returns a generic 500 JSON response (the panic value is logged, never sent to the client). Configured via `Config.ErrorHandler`.

### Logger
Logs method, path, status code, and duration for every request. Not applied by default:

```go
r.Use(middlewares.LoggerMiddleware[T]())
```

---

## Middleware Package

### CORS
```go
r.Use(middlewares.CORSMiddleware[T](middlewares.CORSConfig{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Custom-Header"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

Only the request's `Origin` is echoed back when it is in `AllowOrigins` (with `Vary: Origin`). An empty list or `"*"` allows any origin, but combining a wildcard with `AllowCredentials: true` panics at startup because browsers reject it; list the origins or use `AllowOriginFunc` instead. Preflight requests (`OPTIONS` with `Access-Control-Request-Method`) are answered with `204` even for routes that only register other methods.

### Helmet (Security Headers)
```go
r.Use(middlewares.HelmetMiddleware[T](middlewares.DefaultHelmetConfig()))

// Or customize:
r.Use(middlewares.HelmetMiddleware[T](middlewares.HelmetConfig{
    XSSProtection:         "0", // legacy header; rely on CSP
    ContentTypeNosniff:    "nosniff",
    XFrameOptions:         "DENY",
    HSTSMaxAge:            31536000,
    ContentSecurityPolicy: "default-src 'self'",
    ReferrerPolicy:        "strict-origin",
}))
```

### Rate Limiter
```go
r.Use(middlewares.RateLimiterMiddleware[T](middlewares.RateLimiterConfig{
    Limit:        100,                    // Max requests per window
    Window:       time.Minute,            // Time window
    Store:        customStore,            // Custom store (optional)
    TrustProxies: []string{"10.0.0.0/8"}, // Trusted proxy IPs or CIDRs
    SkipFunc: func(req *http.Request) bool {
        return req.URL.Path == "/health"  // Skip rate limiting
    },
}))
```

### JWT Authentication
```go
r.Use(middlewares.JWTMiddleware[T](middlewares.JWTConfig[T]{
    Secret:      []byte("your-secret"),
    TokenLookup: "header:Authorization",  // or "query:token" or "cookie:jwt"
}))

// Custom error response (receives the real request context):
r.Use(middlewares.JWTMiddleware[T](middlewares.JWTConfig[T]{
    Secret: []byte("your-secret"),
    ErrorHandler: func(c *aether.Context[T], err error) {
        _ = c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
    },
}))

// JWTMiddleware panics at startup when neither Secret nor Validator is set.

// Access claims in handler:
claims := middlewares.GetMapClaims(c)
userID := claims["sub"]
```

**Custom JWT Validator:**
```go
type MyValidator struct{}

func (v *MyValidator) Validate(tokenString string) (jwt.Claims, error) {
    // Your custom validation logic
}

r.Use(middlewares.JWTMiddleware[T](middlewares.JWTConfig[T]{
    Validator: &MyValidator{},
}))
```

### Basic Auth
```go
// With static credentials
r.Use(middlewares.BasicAuthMiddleware[T](middlewares.BasicAuthConfig{
    Users: map[string]string{
        "admin": "secret",
        "user":  "password",
    },
    Realm: "My API",
}))

// With custom validation
r.Use(middlewares.BasicAuthMiddleware[T](middlewares.BasicAuthConfig{
    Validate: func(user, pass string) bool {
        return db.CheckCredentials(user, pass)
    },
}))
```

### CSRF Protection
```go
r.Use(middlewares.CSRFMiddleware[T](middlewares.CSRFConfig{
    TokenLength: 32,
    CookieName:  "_csrf",
    HeaderName:  "X-CSRF-Token",
    Secure:      true,
    HttpOnly:    true,
    SameSite:    http.SameSiteStrictMode,
    SkipFunc: func(req *http.Request) bool {
        return strings.HasPrefix(req.URL.Path, "/api/")
    },
}))
```

### Request ID
```go
r.Use(middlewares.RequestIDMiddleware[T]())
// Adds X-Request-Id header to every response. A valid incoming ID
// (max 128 chars of letters, digits and "-_.:") is reused.

// Read it in a handler:
rid := middlewares.GetRequestID(c)
```

### Gzip Compression
```go
r.Use(middlewares.GzipMiddleware[T]())
// Compresses responses for clients that accept gzip encoding
```

---

## Using net/http Middlewares

Any standard Go middleware with the signature `func(http.Handler) http.Handler` can be wrapped:

```go
import "github.com/gorilla/handlers"

r.Use(aether.WrapMiddleware[T](handlers.RecoveryHandler()))
r.Use(aether.WrapMiddleware[T](handlers.CompressHandler))
```
