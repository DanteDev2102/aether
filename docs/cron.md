# Cron Jobs

Aether includes a built-in cron job scheduler that runs alongside your HTTP server with graceful shutdown support.

## Adding Cron Jobs

```go
app.AddCron("job-name", interval, func(ctx context.Context, log aether.Logger) {
    log.Info("Running job...")
})
```

- **name**: Identifier for the cron job (used in logs).
- **interval**: `time.Duration` between executions.
- **job**: Function that receives a cancellable context and a logger.

## Example

```go
// Run every 5 minutes
app.AddCron("cleanup-sessions", 5*time.Minute, func(ctx context.Context, log aether.Logger) {
    log.Info("Cleaning expired sessions...")
    // Your cleanup logic here
})

// Run every hour
app.AddCron("sync-data", 1*time.Hour, func(ctx context.Context, log aether.Logger) {
    log.Info("Syncing data with external API...")
})
```

## Behavior

- Jobs execute **immediately on startup**, then repeat at the configured interval.
- Jobs must be added **before** calling `app.Listen()`.
- Each job runs in its own goroutine.
- Panics in cron jobs are **recovered** automatically and logged.
- On shutdown (`SIGINT`/`SIGTERM`), all cron jobs are gracefully stopped before the server shuts down.

## Context Cancellation

Use the provided `ctx` to handle shutdowns gracefully:

```go
app.AddCron("long-task", 10*time.Minute, func(ctx context.Context, log aether.Logger) {
    select {
    case <-ctx.Done():
        log.Info("Job cancelled, shutting down...")
        return
    default:
        // Do work
    }
})
```
