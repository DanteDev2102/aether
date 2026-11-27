package middlewares

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"github.com/DanteDev2102/aether"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(nil)
	},
}

// gzipResponseWriter compresses the response body. Compression is decided when
// the status is written: responses that cannot carry a body (1xx, 204, 304,
// HEAD) or that already declare a Content-Encoding are passed through.
type gzipResponseWriter struct {
	aether.ResponseWriter
	method      string
	gw          *gzip.Writer
	compress    bool
	wroteHeader bool
}

// WriteHeader decides whether to compress and sends the status code.
func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true

	h := g.Header()
	g.compress = bodyAllowed(code, g.method) && h.Get("Content-Encoding") == ""
	if g.compress {
		h.Del("Content-Length")
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
	}
	g.ResponseWriter.WriteHeader(code)
}

// Write compresses b when compression is enabled for this response.
func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if !g.compress {
		return g.ResponseWriter.Write(b)
	}
	if g.gw == nil {
		g.gw = gzipWriterPool.Get().(*gzip.Writer)
		g.gw.Reset(g.ResponseWriter)
	}
	return g.gw.Write(b)
}

// Flush flushes compressed data to the client, which keeps streaming responses
// such as Server-Sent Events working.
func (g *gzipResponseWriter) Flush() {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.gw != nil {
		_ = g.gw.Flush()
	}
	_ = http.NewResponseController(g.ResponseWriter).Flush()
}

// Unwrap returns the wrapped writer so http.ResponseController can reach it.
func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

// Written reports whether the response status has already been sent.
func (g *gzipResponseWriter) Written() bool {
	return g.wroteHeader
}

func (g *gzipResponseWriter) close() error {
	if g.gw == nil {
		return nil
	}
	err := g.gw.Close()
	g.gw.Reset(nil)
	gzipWriterPool.Put(g.gw)
	g.gw = nil
	return err
}

func bodyAllowed(code int, method string) bool {
	if method == http.MethodHead {
		return false
	}
	return code >= 200 && code != http.StatusNoContent && code != http.StatusNotModified
}

func acceptsGzip(req *http.Request) bool {
	for _, enc := range strings.Split(req.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(enc), ";")
		if strings.EqualFold(strings.TrimSpace(name), "gzip") {
			return strings.ReplaceAll(strings.TrimSpace(params), " ", "") != "q=0"
		}
	}
	return false
}

// GzipMiddleware compresses responses using gzip encoding when the client
// accepts it. WebSocket upgrades are left untouched.
func GzipMiddleware[T any]() aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		if !acceptsGzip(c.Req()) || c.Req().Header.Get("Upgrade") != "" {
			c.Next()
			return
		}

		orig, ok := c.Res().(aether.ResponseWriter)
		if !ok {
			c.Next()
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: orig, method: c.Req().Method}
		c.SetRes(gw)
		defer func() {
			if err := gw.close(); err != nil {
				c.Log().Errorf("gzip: failed to finish response: %v", err)
			}
			c.SetRes(orig)
		}()

		c.Next()
	}
}
