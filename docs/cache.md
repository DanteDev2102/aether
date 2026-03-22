# Cache

Aether provides a pluggable caching system through the `CacheStore` interface.

## Interface

```go
type CacheStore interface {
    Get(ctx context.Context, key string) (any, bool)
    Set(ctx context.Context, key string, value any) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

## Default: Otter

Aether ships with a default in-memory cache powered by [Otter](https://github.com/maypok86/otter), an extremely fast concurrent cache for Go.

```go
store, err := aether.NewOtterStore(10000)  // max 10,000 entries
if err != nil {
    panic(err)
}

app := aether.New(&aether.Config[AppState]{
    Cache: store,
})
```

## Usage in Handlers

```go
aether.Get(r, "/data", func(c *aether.Context[AppState]) {
    ctx := c.Req().Context()

    // Try cache first
    if val, ok := c.Cache().Get(ctx, "my_key"); ok {
        c.JSON(200, val)
        return
    }

    // Compute and cache
    result := expensiveComputation()
    c.Cache().Set(ctx, "my_key", result)
    c.JSON(200, result)
})
```

## Custom Implementations

### Redis Example

```go
type RedisStore struct {
    client *redis.Client
}

func (r *RedisStore) Get(ctx context.Context, key string) (any, bool) {
    val, err := r.client.Get(ctx, key).Result()
    if err != nil {
        return nil, false
    }
    return val, true
}

func (r *RedisStore) Set(ctx context.Context, key string, value any) error {
    return r.client.Set(ctx, key, value, 0).Err()
}

func (r *RedisStore) Delete(ctx context.Context, key string) error {
    return r.client.Del(ctx, key).Err()
}

func (r *RedisStore) Clear(ctx context.Context) error {
    return r.client.FlushDB(ctx).Err()
}
```

Then configure:

```go
app := aether.New(&aether.Config[AppState]{
    Cache: &RedisStore{client: redisClient},
})
```
