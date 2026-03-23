package aether

import (
	"time"
)

// LoggerMiddleware logs HTTP requests with method, path, status, and duration.
func LoggerMiddleware[T any]() HandlerFunc[T] {
	return func(c *Context[T]) {
		c.Next()
		duration := time.Since(c.Start())
		c.Log().Infof("%s %s | %d | %v", c.Req().Method, c.Req().URL.Path, c.Res().(ResponseWriter).Status(), duration)
	}
}
