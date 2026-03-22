# Routing

Aether uses Go 1.22+ `http.ServeMux` for pattern matching, supporting path parameters natively.

## HTTP Methods

```go
r := app.Router()

aether.Get(r, "/path", handler)
aether.Post[T, Body](r, "/path", handlerWithBody)
aether.Put[T, Body](r, "/path", handlerWithBody)
aether.Patch[T, Body](r, "/path", handlerWithBody)
aether.Delete(r, "/path", handler)
aether.Head(r, "/path", handler)
aether.Options(r, "/path", handler)
aether.Connect(r, "/path", handler)
aether.Trace(r, "/path", handler)
```

## Path Parameters

```go
aether.Get(r, "/users/{id}", func(c *aether.Context[T]) {
    id := c.Param("id")
    c.String(200, "User: %s", id)
})

// Wildcard (catch-all)
aether.Get(r, "/files/{path...}", func(c *aether.Context[T]) {
    path := c.Param("path")
})
```

## Route Groups

Groups share a common prefix and middleware chain:

```go
api := aether.NewGroup("/api/v1", r)
api.Use(authMiddleware)

aether.Get(api, "/users", listUsers)    // GET /api/v1/users
aether.Post[T, B](api, "/users", createUser) // POST /api/v1/users
```

Groups can be nested:

```go
admin := aether.NewGroup("/admin", api)  // prefix: /api/v1/admin
aether.Get(admin, "/stats", getStats)    // GET /api/v1/admin/stats
```

## Auto Body Binding

`Post`, `Put`, and `Patch` automatically deserialize the request body:

```go
type CreateUser struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

aether.Post[AppState, CreateUser](r, "/users", func(c *aether.Context[AppState], body CreateUser) {
    // body is already parsed and typed
    c.JSON(201, body)
})
```

Supports: `application/json`, `application/xml`, `application/x-www-form-urlencoded`, `multipart/form-data`.

## Static Files

```go
aether.Static(r, "/assets/", "./public")
```

This serves files from the `./public` directory under the `/assets/` URL prefix.
