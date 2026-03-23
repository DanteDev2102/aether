package middlewares

import (
	"time"

	"github.com/DanteDev2102/aether"
)

// LoggerMiddleware logs HTTP requests with method, path, status, and duration.
func LoggerMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		c.Next()
		duration := time.Since(c.Start())
		c.Log().Infof("%s %s | %d | %v", c.Req().Method, c.Req().URL.Path, c.Res().(aether.ResponseWriter).Status(), duration)
	}
}
