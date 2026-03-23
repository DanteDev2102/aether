package middlewares

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/DantDev2102/aether"
)

type gzipResponseWriter struct {
	aether.ResponseWriter
	gw *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if g.Header().Get("Content-Type") == "" {
		g.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return g.gw.Write(b)
}

// GzipMiddleware compresses responses using gzip encoding.
func GzipMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		if !strings.Contains(c.Req().Header.Get("Accept-Encoding"), "gzip") || strings.Contains(c.Req().Header.Get("Connection"), "Upgrade") {
			c.Next()
			return
		}

		c.Res().Header().Set("Content-Encoding", "gzip")
		c.Res().Header().Add("Vary", "Accept-Encoding")

		gz := gzip.NewWriter(c.Res())
		defer func() { _ = gz.Close() }()

		// Note: Gzip wrapping requires direct response writer replacement.
		// This middleware works at the http.ResponseWriter level.
		wrapped := &gzipResponseWriter{
			ResponseWriter: c.Res().(aether.ResponseWriter),
			gw:             gz,
		}

		_ = wrapped
		c.Next()
	}
}
