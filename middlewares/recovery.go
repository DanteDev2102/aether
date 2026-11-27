package middlewares

import (
	"github.com/DanteDev2102/aether"
)

// ExternalRecoveryMiddleware recovers from panics with custom error handling.
//
// Deprecated: aether.New already installs aether.RecoveryMiddleware; use
// Config.ErrorHandler or aether.RecoveryMiddleware directly.
func ExternalRecoveryMiddleware[T any](customHandler func(c *aether.Context[T], err any)) aether.HandlerFunc[T] {
	return aether.RecoveryMiddleware[T](customHandler)
}
