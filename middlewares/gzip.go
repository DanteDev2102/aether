package middlewares

import (
	"compress/gzip"
	"net/http"
	"strings"

	"github.com/DantDev2102/aether"
)

type gzipResponseWriter struct {
	aether.ResponseWriter 
	gw             *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if g.Header().Get("Content-Type") == "" {
		g.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return g.gw.Write(b)
}

func GzipMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		if !strings.Contains(c.Req.Header.Get("Accept-Encoding"), "gzip") || strings.Contains(c.Req.Header.Get("Connection"), "Upgrade") {
			c.Next()
			return
		}

		c.Res.Header().Set("Content-Encoding", "gzip")
		c.Res.Header().Add("Vary", "Accept-Encoding")

		gz := gzip.NewWriter(c.Res)

		ogRes := c.Res
		c.Res = &gzipResponseWriter{
			ResponseWriter: ogRes,
			gw:             gz,
		}

		defer func() {
			gz.Close()
			c.Res = ogRes
		}()

		c.Next()
	}
}
