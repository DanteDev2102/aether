package aether

import (
	"time"
)

func LoggerMiddleware[T any]() HandlerFunc[T] {
	return func(c *Context[T]) {
		c.Next()
		duration := time.Since(c.Start())
		c.Log().Infof("%s %s | %d | %v", c.Req().Method, c.Req().URL.Path, c.res.Status(), duration)
	}
}
