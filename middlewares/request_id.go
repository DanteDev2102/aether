package middlewares

import (
	"context"

	"github.com/google/uuid"

	"github.com/DanteDev2102/aether"
)

const requestIDKey contextKey = "RequestID"

// RequestIDHeader is the header used to read and return the request ID.
const RequestIDHeader = "X-Request-Id"

const maxRequestIDLength = 128

// validRequestID reports whether an incoming ID is safe to reuse: short and
// made only of letters, digits and "-_.:", so it cannot inject into logs.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
		case ch == '-', ch == '_', ch == '.', ch == ':':
		default:
			return false
		}
	}
	return true
}

// RequestIDMiddleware adds a unique request ID to each request. A valid
// incoming X-Request-Id is reused; otherwise a new UUID is generated.
func RequestIDMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		rid := c.Req().Header.Get(RequestIDHeader)
		if !validRequestID(rid) {
			rid = uuid.New().String()
		}

		c.Res().Header().Set(RequestIDHeader, rid)

		ctx := context.WithValue(c.Req().Context(), requestIDKey, rid)
		c.SetReq(c.Req().WithContext(ctx))

		c.Next()
	}
}

// GetRequestID returns the request ID set by RequestIDMiddleware, or "" when
// the middleware did not run.
func GetRequestID[T any](c *aether.Context[T]) string {
	rid, _ := c.Req().Context().Value(requestIDKey).(string)
	return rid
}
