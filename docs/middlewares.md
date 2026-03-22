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
Catches panics and returns a 500 response. Configured via `Config.ErrorHandler`.

### Logger (auto-applied)
Logs method, path, status code, and duration for every request.

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

### Helmet (Security Headers)
```go
r.Use(middlewares.HelmetMiddleware[T](middlewares.DefaultHelmetConfig()))

// Or customize:
r.Use(middlewares.HelmetMiddleware[T](middlewares.HelmetConfig{
    XSSProtection:         "1; mode=block",
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
    TrustProxies: []string{"10.0.0.1"},   // Trusted proxy IPs
    SkipFunc: func(req *http.Request) bool {
        return req.URL.Path == "/health"  // Skip rate limiting
    },
}))
```

### JWT Authentication
```go
r.Use(middlewares.JWTMiddleware[T](middlewares.JWTConfig{
    Secret:      []byte("your-secret"),
    TokenLookup: "header:Authorization",  // or "query:token" or "cookie:jwt"
}))

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

r.Use(middlewares.JWTMiddleware[T](middlewares.JWTConfig{
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
// Adds X-Request-Id header to every response
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
