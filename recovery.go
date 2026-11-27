package aether

import (
	"net/http"
	"runtime/debug"
)

// CustomErrorHandler defines the function signature for custom error handling.
type CustomErrorHandler[T any] func(c *Context[T], err any)

// RecoveryMiddleware recovers from panics and logs the error stack trace.
// Without a custom handler it answers with a generic 500 JSON response; the
// panic value is only logged, never sent to the client. Nothing is written
// when the handler had already started the response.
func RecoveryMiddleware[T any](customHandler CustomErrorHandler[T]) HandlerFunc[T] {
	return func(c *Context[T]) {
		defer func() {
			err := recover()
			if err == nil {
				return
			}
			if err == http.ErrAbortHandler {
				panic(err)
			}

			c.Log().Errorf("[PANIC RECOVERED] %v\n%s", err, debug.Stack())

			if customHandler != nil {
				customHandler(c, err)
				return
			}
			if rw, ok := c.Res().(ResponseWriter); ok && rw.Written() {
				return
			}
			_ = c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Internal Server Error",
			})
		}()
		c.Next()
	}
}
