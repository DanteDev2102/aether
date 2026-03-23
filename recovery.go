package aether

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

// CustomErrorHandler defines the function signature for custom error handling.
type CustomErrorHandler[T any] func(c *Context[T], err any)

// RecoveryMiddleware recovers from panics and logs the error stack trace.
func RecoveryMiddleware[T any](customHandler CustomErrorHandler[T]) HandlerFunc[T] {
	return func(c *Context[T]) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				c.Log().Errorf("[PANIC RECOVERED] %v\n%s", err, stack)

				if customHandler != nil {
					customHandler(c, err)
				} else {
					_ = c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "Internal Server Error",
						"panic": fmt.Sprintf("%v", err),
					})
				}
			}
		}()
		c.Next()
	}
}
