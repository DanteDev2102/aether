package middlewares

import (
	"context"

	"github.com/DanteDev2102/aether"
	"github.com/google/uuid"
)

const requestIDKey contextKey = "RequestID"

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		rid := c.Req().Header.Get("X-Request-Id")
		if rid == "" {
			rid = uuid.New().String()
		}

		c.Res().Header().Set("X-Request-Id", rid)

		ctx := c.Req().Context()
		ctx = context.WithValue(ctx, requestIDKey, rid)
		c.SetReq(c.Req().WithContext(ctx))

		c.Next()
	}
}
