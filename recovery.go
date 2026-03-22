package aether

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

type CustomErrorHandler[T any] func(c *Context[T], err any)

func RecoveryMiddleware[T any](customHandler CustomErrorHandler[T]) HandlerFunc[T] {
	return func(c *Context[T]) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				c.Log().Errorf("[PANIC RECOVERED] %v\n%s", err, stack)

				if customHandler != nil {
					customHandler(c, err)
				} else {
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
