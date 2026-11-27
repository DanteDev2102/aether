package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DanteDev2102/aether"
)

func doFrom(r http.Handler, remote, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newLimitedRouter(cfg RateLimiterConfig) *aether.Router[testGlobal] {
	r := newTestRouter(RateLimiterMiddleware[testGlobal](cfg))
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, "ok") })
	return r
}

func TestRateLimiterMiddleware_BlocksAfterLimit(t *testing.T) {
	r := newLimitedRouter(RateLimiterConfig{Limit: 2, Window: time.Minute})

	for i := 0; i < 2; i++ {
		if w := doFrom(r, "1.2.3.4:1000", ""); w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
	w := doFrom(r, "1.2.3.4:1000", "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After 60, got %q", w.Header().Get("Retry-After"))
	}
	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected remaining 0, got %q", w.Header().Get("X-RateLimit-Remaining"))
	}

	if w := doFrom(r, "5.6.7.8:1000", ""); w.Code != http.StatusOK {
		t.Errorf("other client should not be limited, got %d", w.Code)
	}
}

func TestRateLimiterMiddleware_IgnoresXFFFromUntrustedPeer(t *testing.T) {
	r := newLimitedRouter(RateLimiterConfig{Limit: 1, Window: time.Minute, TrustProxies: []string{"10.0.0.0/8"}})

	doFrom(r, "1.2.3.4:1000", "9.9.9.1")
	if w := doFrom(r, "1.2.3.4:1000", "9.9.9.2"); w.Code != http.StatusTooManyRequests {
		t.Errorf("spoofed XFF from untrusted peer bypassed the limit, got %d", w.Code)
	}
}

func TestRateLimiterMiddleware_UsesRightmostUntrustedXFF(t *testing.T) {
	r := newLimitedRouter(RateLimiterConfig{Limit: 1, Window: time.Minute, TrustProxies: []string{"10.0.0.0/8"}})

	doFrom(r, "10.0.0.1:1000", "forged-1, 8.8.8.8, 10.0.0.2")
	if w := doFrom(r, "10.0.0.1:1000", "forged-2, 8.8.8.8, 10.0.0.2"); w.Code != http.StatusTooManyRequests {
		t.Errorf("forged left-most XFF entry bypassed the limit, got %d", w.Code)
	}
}

func TestRateLimiterMiddleware_IPv6RemoteAddr(t *testing.T) {
	ip := trustedProxies(nil).clientIP(&http.Request{RemoteAddr: "[2001:db8::1]:443"})
	if ip != "2001:db8::1" {
		t.Errorf("expected '2001:db8::1', got %q", ip)
	}
}

func TestMemoryRateLimiterStore_SweepsExpiredEntries(t *testing.T) {
	s := NewMemoryRateLimiterStore()
	window := 10 * time.Millisecond

	for _, k := range []string{"a", "b", "c"} {
		_, _ = s.Increment(k, window)
	}
	time.Sleep(3 * window)
	_, _ = s.Increment("d", window)

	n := 0
	s.m.Range(func(_, _ any) bool { n++; return true })
	if n != 1 {
		t.Errorf("expected expired entries to be swept, %d remain", n)
	}
}
