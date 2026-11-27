# Routing

Aether uses Go's `http.ServeMux` for pattern matching, supporting path parameters natively.

> **Note on `/`:** in `http.ServeMux` a pattern ending in `/` matches every path
> under it, so `aether.Get(r, "/", h)` also answers `/anything` with 200 instead
> of 404. Use `"/{$}"` to match only the root path.

```go
aether.Get(r, "/{$}", home)   // only "/"
aether.Get(r, "/", fallback)  // "/" and every unmatched GET path
```

Registering the same method and pattern twice panics at startup.

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
    c.Stringf(200, "User: %s", id)
})

// Wildcard (catch-all)
aether.Get(r, "/files/{path...}", func(c *aether.Context[T]) {
    path := c.Param("path")
})
```

## Route Groups

Groups share a common prefix and inherit the middlewares of their parent at
creation time:

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

- An empty body is valid and yields the zero value of the body type.
- Malformed bodies, and form values that do not fit the field type (`age=abc`
  for an `int`), get a `400`; bodies over `MaxBodyBytes` get a `413`.
- Form fields are matched by the `form` tag, then the `json` tag (options such
  as `,omitempty` are ignored, `-` skips the field), then the lowercase field
  name. Slice fields receive every value (`?tag=a&tag=b`).

## Static Files

```go
aether.Static(r, "/assets/", "./public")
```

This serves files from the `./public` directory under the `/assets/` URL prefix.
