package middlewares

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/DantDev2102/aether"
)

type CustomErrorHandler[T any] func(c *aether.Context[T], err any)

func RecoveryMiddleware[T any](customHandler CustomErrorHandler[T]) aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				c.Log.Errorf("[PANIC RECOVERED] %v\n%s", err, stack)

				if customHandler != nil {
					customHandler(c, err)
				} else {
					c.Res.WriteHeader(http.StatusInternalServerError)
					c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "Internal Server Error",
						"panic": fmt.Sprintf("%v", err),
					})
				}
			}
		}()
		c.Next()
	}
}
