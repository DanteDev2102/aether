package middlewares

import (
	"net/http"
	"testing"

	"github.com/DanteDev2102/aether"
)

func newCORSRouter(cfg CORSConfig) *aether.Router[testGlobal] {
	r := newTestRouter(CORSMiddleware[testGlobal](cfg))
	aether.Post(r, "/items", func(c *testCtx, _ struct{}) { _ = c.String(http.StatusOK, "ok") })
	return r
}

func TestCORSMiddleware_EchoesAllowedOrigin(t *testing.T) {
	r := newCORSRouter(CORSConfig{AllowOrigins: []string{"https://a.com", "https://b.com"}})

	w := do(r, http.MethodPost, "/items", nil, map[string]string{"Origin": "https://b.com"})

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://b.com" {
		t.Errorf("expected allowed origin 'https://b.com', got %q", got)
	}
	if w.Header().Get("Vary") != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", w.Header().Get("Vary"))
	}
}

func TestCORSMiddleware_RejectsUnknownOrigin(t *testing.T) {
	r := newCORSRouter(CORSConfig{AllowOrigins: []string{"https://a.com"}})

	w := do(r, http.MethodPost, "/items", nil, map[string]string{"Origin": "https://evil.com"})

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no allow-origin header, got %q", got)
	}
}

func TestCORSMiddleware_PreflightOnPostOnlyRoute(t *testing.T) {
	r := newCORSRouter(CORSConfig{AllowOrigins: []string{"https://a.com"}, MaxAge: 600})

	w := do(r, http.MethodOptions, "/items", nil, map[string]string{
		"Origin":                        "https://a.com",
		"Access-Control-Request-Method": "POST",
	})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Errorf("expected Access-Control-Allow-Methods to be set")
	}
	if w.Header().Get("Access-Control-Max-Age") != "600" {
		t.Errorf("expected Max-Age 600, got %q", w.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSMiddleware_WildcardWithCredentialsPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("expected panic for wildcard origin with credentials")
		}
	}()
	CORSMiddleware[testGlobal](CORSConfig{AllowCredentials: true})
}

func TestCORSMiddleware_OriginFunc(t *testing.T) {
	r := newCORSRouter(CORSConfig{
		AllowCredentials: true,
		AllowOriginFunc:  func(o string) bool { return o == "https://ok.com" },
	})

	w := do(r, http.MethodPost, "/items", nil, map[string]string{"Origin": "https://ok.com"})

	if w.Header().Get("Access-Control-Allow-Origin") != "https://ok.com" {
		t.Errorf("expected origin to be echoed, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected credentials header")
	}
}
