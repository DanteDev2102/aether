package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/DanteDev2102/aether"
)

func TestGzipMiddleware_CompressesBody(t *testing.T) {
	r := newTestRouter(GzipMiddleware[testGlobal]())
	payload := strings.Repeat("hello aether ", 100)
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, payload) })

	w := do(r, http.MethodGet, "/", nil, map[string]string{"Accept-Encoding": "gzip, deflate"})

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", w.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("invalid gzip stream: %v", err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(got) != payload {
		t.Errorf("decompressed body mismatch")
	}
	if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("expected Content-Type to be preserved, got %q", w.Header().Get("Content-Type"))
	}
}

func TestGzipMiddleware_SkipsWithoutAcceptEncoding(t *testing.T) {
	r := newTestRouter(GzipMiddleware[testGlobal]())
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, "plain") })

	for _, ae := range []string{"", "br", "gzip;q=0"} {
		w := do(r, http.MethodGet, "/", nil, map[string]string{"Accept-Encoding": ae})
		if w.Header().Get("Content-Encoding") != "" {
			t.Errorf("Accept-Encoding %q: expected no encoding, got %q", ae, w.Header().Get("Content-Encoding"))
		}
		if w.Body.String() != "plain" {
			t.Errorf("Accept-Encoding %q: expected 'plain', got %q", ae, w.Body.String())
		}
	}
}

func TestGzipMiddleware_NoBodyStatus(t *testing.T) {
	r := newTestRouter(GzipMiddleware[testGlobal]())
	aether.Delete(r, "/x", func(c *testCtx) { c.Res().WriteHeader(http.StatusNoContent) })

	w := do(r, http.MethodDelete, "/x", nil, map[string]string{"Accept-Encoding": "gzip"})

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
	if w.Body.Len() != 0 || w.Header().Get("Content-Encoding") != "" {
		t.Errorf("expected empty, unencoded 204; got encoding %q and %d bytes", w.Header().Get("Content-Encoding"), w.Body.Len())
	}
}
