package middlewares

import (
	"time"

	"github.com/DantDev2102/aether"
)

func LoggerMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		c.Next()
		duration := time.Since(c.Start())
		c.Log().Infof("%s %s | %d | %v", c.Req().Method, c.Req().URL.Path, c.Res().Status(), duration)
	}
}
