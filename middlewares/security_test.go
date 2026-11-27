package middlewares

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/DanteDev2102/aether"
)

func TestRequestIDMiddleware(t *testing.T) {
	r := newTestRouter(RequestIDMiddleware[testGlobal]())
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, GetRequestID(c)) })

	w := do(r, http.MethodGet, "/", nil, map[string]string{RequestIDHeader: "abc-123"})
	if w.Body.String() != "abc-123" || w.Header().Get(RequestIDHeader) != "abc-123" {
		t.Errorf("expected valid incoming ID to be reused, got %q", w.Body.String())
	}

	w = do(r, http.MethodGet, "/", nil, map[string]string{RequestIDHeader: "bad\nid injected=1"})
	if got := w.Body.String(); got == "" || strings.ContainsAny(got, "\n ") {
		t.Errorf("expected invalid incoming ID to be replaced, got %q", got)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	r := newTestRouter(CSRFMiddleware[testGlobal](CSRFConfig{}))
	aether.Get(r, "/form", func(c *testCtx) { _ = c.String(http.StatusOK, "form") })
	aether.Post(r, "/submit", func(c *testCtx, _ struct{}) { _ = c.String(http.StatusOK, "done") })

	w := do(r, http.MethodGet, "/form", nil, nil)
	token := w.Header().Get("X-CSRF-Token")
	if token == "" {
		t.Fatalf("expected a CSRF token")
	}
	cookie := "_csrf=" + token

	if w := do(r, http.MethodPost, "/submit", nil, map[string]string{"Cookie": cookie}); w.Code != http.StatusForbidden {
		t.Errorf("missing token: expected 403, got %d", w.Code)
	}
	if w := do(r, http.MethodPost, "/submit", nil, map[string]string{"Cookie": cookie, "X-CSRF-Token": "wrong"}); w.Code != http.StatusForbidden {
		t.Errorf("wrong token: expected 403, got %d", w.Code)
	}
	if w := do(r, http.MethodPost, "/submit", nil, map[string]string{"Cookie": cookie, "X-CSRF-Token": token}); w.Code != http.StatusOK {
		t.Errorf("valid header token: expected 200, got %d", w.Code)
	}

	form := url.Values{"_csrf": {token}}.Encode()
	w = do(r, http.MethodPost, "/submit", strings.NewReader(form), map[string]string{
		"Cookie":       cookie,
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if w.Code != http.StatusOK {
		t.Errorf("valid form token: expected 200, got %d", w.Code)
	}
}

func TestHelmetMiddleware_Defaults(t *testing.T) {
	r := newTestRouter(HelmetMiddleware[testGlobal](DefaultHelmetConfig()))
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, "ok") })

	w := do(r, http.MethodGet, "/", nil, nil)

	want := map[string]string{
		"X-XSS-Protection":          "0",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "SAMEORIGIN",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s: expected %q, got %q", k, v, got)
		}
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	r := newTestRouter(BasicAuthMiddleware[testGlobal](BasicAuthConfig{Users: map[string]string{"admin": "s3cret"}}))
	aether.Get(r, "/", func(c *testCtx) { _ = c.String(http.StatusOK, "ok") })

	cases := []struct {
		name, user, pass string
		set              bool
		want             int
	}{
		{"no credentials", "", "", false, http.StatusUnauthorized},
		{"wrong password", "admin", "nope", true, http.StatusUnauthorized},
		{"unknown user", "root", "s3cret", true, http.StatusUnauthorized},
		{"valid", "admin", "s3cret", true, http.StatusOK},
	}
	for _, tc := range cases {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if tc.set {
			req.SetBasicAuth(tc.user, tc.pass)
		}
		w := do(r, http.MethodGet, "/", nil, map[string]string{"Authorization": req.Header.Get("Authorization")})
		if w.Code != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.want, w.Code)
		}
		if tc.want == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: expected WWW-Authenticate header", tc.name)
		}
	}
}
