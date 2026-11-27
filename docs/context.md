# Context

The `Context[T]` is the core request-scoped object passed to every handler. It provides access to the request, response, global state, and framework utilities.

## Request & Response

```go
// Access the underlying net/http objects
req := c.Req()           // *http.Request
res := c.Res()           // http.ResponseWriter
c.SetReq(newReq)         // Replace the request (useful in middlewares)
```

## Parameters

```go
id := c.Param("id")       // Path parameter
q := c.Query("search")    // Query string parameter
```

## Body

```go
// Raw body bytes
body, err := c.GetBody()

// Automatic binding (used internally by Post/Put/Patch)
var data MyStruct
err := c.Bind(&data)
```

## Responses

```go
c.JSON(200, data)                       // JSON response
c.XML(200, data)                        // XML response
c.String(200, "Hello")                    // Plain text, written as is
c.Stringf(200, "Hello %s", name)         // Plain text with formatting
c.Render(200, "template_name", data)    // HTML template (requires TemplateEngine)
```

## File Responses

```go
c.File("/path/to/file.pdf")                        // Serve file inline
c.Attachment("/path/to/file.pdf", "download.pdf")  // Force download
c.SaveFile(fileHeader, "/uploads/file.jpg")         // Save uploaded file
```

## Cookies

```go
// Set
c.SetCookie(&http.Cookie{
    Name:     "session",
    Value:    "token123",
    Path:     "/",
    MaxAge:   86400,
    HttpOnly: true,
})

// Get
cookie, err := c.Cookie("session")

// Delete
c.ClearCookie("session")
```

## Cache

```go
ctx := c.Req().Context()

// Set a value
c.Cache().Set(ctx, "key", "value")

// Get a value
val, ok := c.Cache().Get(ctx, "key")

// Delete a value
c.Cache().Delete(ctx, "key")

// Clear all
c.Cache().Clear(ctx)
```

## SSE (Server-Sent Events)

```go
rc, err := c.SSE()
if err != nil {
    return
}

for i := 0; i < 10; i++ {
    fmt.Fprintf(c.Res(), "data: message %d\n\n", i)
    rc.Flush()
    time.Sleep(time.Second)
}
```

## WebSocket (via Hijack)

```go
conn, bufrw, err := c.Hijack()
if err != nil {
    return
}
defer conn.Close()
// Use conn for WebSocket communication
```

## Middleware Control

```go
c.Next()    // Call the next handler in the middleware chain
```

## Logging

```go
c.Log().Info("Processing request")
c.Log().Errorf("Error: %v", err)
```

## Global State

`c.Global` is a pointer to the single `Config.Global` value shared by the whole
app. Changes persist across requests, so protect mutable fields:

```go
dbConn := c.Global.DB       // Access typed global state
version := c.Global.Version

type AppState struct {
    Hits atomic.Int64       // safe to mutate concurrently
}
c.Global.Hits.Add(1)
```

## Background Work (Copy)

The `Context` is reused for later requests once the handler returns, so do not
keep it or use it from goroutines. Use `c.Copy()` instead: the copy keeps the
request (its context is no longer canceled when the request ends), path
parameters, `Global`, logger and cache, but it cannot write a response
(`ErrDetachedContext`) and the request body must not be read from it.

```go
aether.Post[T, Order](r, "/orders", func(c *aether.Context[T], o Order) {
    cp := c.Copy()
    go func() {
        sendConfirmationEmail(cp.Req().Context(), o)
        cp.Log().Infof("email sent for order %s", cp.Param("id"))
    }()
    _ = c.JSON(202, map[string]string{"status": "accepted"})
})
```

## Timeouts

`Config.Timeout` sets a deadline on `c.Req().Context()`; it never cuts the
connection. Pass that context to databases and HTTP clients so they stop when
it expires. For long responses, drop the deadline (client disconnects still
cancel the context):

```go
c.ClearTimeout()  // c.SSE() does this automatically
```

## Timing

```go
startTime := c.Start()  // When the request started processing
```
