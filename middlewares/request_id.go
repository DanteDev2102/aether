package middlewares

import (
	"context"

	"github.com/DantDev2102/aether"
	"github.com/google/uuid"
)

func RequestIDMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		rid := c.Req.Header.Get("X-Request-Id")
		if rid == "" {
			rid = uuid.New().String()
		}

		c.Res.Header().Set("X-Request-Id", rid)

		ctx := c.Req.Context()
		ctx = context.WithValue(ctx, "RequestID", rid)
		c.Req = c.Req.WithContext(ctx)

		c.Next()
	}
}
