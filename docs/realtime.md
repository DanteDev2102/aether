# Real-time: SSE & WebSockets

Aether provides native support for Server-Sent Events (SSE) and WebSocket connections via `http.ResponseController` and connection hijacking.

## Server-Sent Events (SSE)

SSE allows the server to push events to the client over a single HTTP connection.

```go
aether.Get(r, "/events", func(c *aether.Context[AppState]) {
    rc, err := c.SSE()
    if err != nil {
        c.Log().Errorf("SSE setup failed: %v", err)
        return
    }

    // Send events
    for i := 0; i < 100; i++ {
        fmt.Fprintf(c.Res(), "event: update\n")
        fmt.Fprintf(c.Res(), "data: {\"count\": %d}\n\n", i)
        rc.Flush()
        time.Sleep(time.Second)
    }
})
```

The `c.SSE()` method automatically sets the appropriate headers:
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`

### SSE Event Format

```
event: eventName
data: your data here

```

Each event must end with two newlines (`\n\n`).

### Client-Side

```javascript
const es = new EventSource("/events");
es.addEventListener("update", (e) => {
    console.log(JSON.parse(e.data));
});
```

## WebSockets (via Hijack)

For full-duplex communication, use `c.Hijack()` to take over the connection:

```go
aether.Get(r, "/ws", func(c *aether.Context[AppState]) {
    conn, bufrw, err := c.Hijack()
    if err != nil {
        c.Log().Errorf("Hijack failed: %v", err)
        return
    }
    defer conn.Close()

    // Send HTTP 101 Switching Protocols
    bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
    bufrw.WriteString("Upgrade: websocket\r\n")
    bufrw.WriteString("Connection: Upgrade\r\n\r\n")
    bufrw.Flush()

    // Now use conn for raw TCP communication
    // For production WebSockets, use a library like gorilla/websocket
})
```

> **Tip:** For production WebSocket usage, consider using [gorilla/websocket](https://github.com/gorilla/websocket) with `c.Hijack()` or wrapping its upgrader with `aether.WrapMiddleware`.
