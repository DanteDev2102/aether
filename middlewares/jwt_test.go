package middlewares

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/DanteDev2102/aether"
)

var testSecret = []byte("test-secret")

func signToken(t *testing.T, secret []byte, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "42", "exp": exp.Unix()})
	s, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func newJWTRouter(cfg JWTConfig[testGlobal]) *aether.Router[testGlobal] {
	r := newTestRouter(JWTMiddleware(cfg))
	aether.Get(r, "/me", func(c *testCtx) {
		_ = c.String(http.StatusOK, GetMapClaims(c)["sub"].(string))
	})
	return r
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	r := newJWTRouter(JWTConfig[testGlobal]{Secret: testSecret})

	w := do(r, http.MethodGet, "/me", nil, map[string]string{
		"Authorization": "bearer " + signToken(t, testSecret, time.Now().Add(time.Hour)),
	})

	if w.Code != http.StatusOK || w.Body.String() != "42" {
		t.Errorf("expected 200 '42', got %d %q", w.Code, w.Body.String())
	}
}

func TestJWTMiddleware_InvalidTokens(t *testing.T) {
	r := newJWTRouter(JWTConfig[testGlobal]{Secret: testSecret})

	cases := map[string]string{
		"missing":    "",
		"expired":    "Bearer " + signToken(t, testSecret, time.Now().Add(-time.Hour)),
		"bad secret": "Bearer " + signToken(t, []byte("other"), time.Now().Add(time.Hour)),
	}
	for name, header := range cases {
		w := do(r, http.MethodGet, "/me", nil, map[string]string{"Authorization": header})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected status %d, got %d", name, http.StatusUnauthorized, w.Code)
		}
	}
}

func TestJWTMiddleware_ErrorHandlerGetsRealContext(t *testing.T) {
	var gotErr error
	r := newJWTRouter(JWTConfig[testGlobal]{
		Secret: testSecret,
		ErrorHandler: func(c *testCtx, err error) {
			gotErr = err
			_ = c.JSON(http.StatusTeapot, map[string]string{"path": c.Req().URL.Path})
		},
	})

	w := do(r, http.MethodGet, "/me", nil, nil)

	if w.Code != http.StatusTeapot {
		t.Errorf("expected custom status %d, got %d", http.StatusTeapot, w.Code)
	}
	if !errors.Is(gotErr, ErrMissingToken) {
		t.Errorf("expected ErrMissingToken, got %v", gotErr)
	}
}

func TestJWTMiddleware_RequiresSecret(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("expected panic without Secret or Validator")
		}
	}()
	JWTMiddleware(JWTConfig[testGlobal]{})
}
