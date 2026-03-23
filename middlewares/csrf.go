package middlewares

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/DantDev2102/aether"
)

// CSRFConfig holds configuration for CSRF protection middleware.
type CSRFConfig struct {
	TokenLength int
	CookieName  string
	HeaderName  string
	CookiePath  string
	Secure      bool
	HttpOnly    bool
	SameSite    http.SameSite
	SkipFunc    func(req *http.Request) bool
}

func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRFMiddleware provides CSRF token generation and validation.
func CSRFMiddleware[T any](cfg CSRFConfig) aether.HandlerFunc[T] {
	if cfg.TokenLength == 0 {
		cfg.TokenLength = 32
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "_csrf"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if cfg.SameSite == 0 {
		cfg.SameSite = http.SameSiteStrictMode
	}

	safeMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodTrace:   true,
	}

	return func(c *aether.Context[T]) {
		if cfg.SkipFunc != nil && cfg.SkipFunc(c.Req()) {
			c.Next()
			return
		}

		cookie, err := c.Cookie(cfg.CookieName)
		var token string

		if err != nil || cookie.Value == "" {
			token, err = generateToken(cfg.TokenLength)
			if err != nil {
				_ = c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Failed to generate CSRF token",
				})
				return
			}
			c.SetCookie(&http.Cookie{
				Name:     cfg.CookieName,
				Value:    token,
				Path:     cfg.CookiePath,
				Secure:   cfg.Secure,
				HttpOnly: cfg.HttpOnly,
				SameSite: cfg.SameSite,
			})
		} else {
			token = cookie.Value
		}

		c.Res().Header().Set(cfg.HeaderName, token)

		if safeMethods[c.Req().Method] {
			c.Next()
			return
		}

		clientToken := c.Req().Header.Get(cfg.HeaderName)
		if clientToken == "" {
			clientToken = c.Req().FormValue(cfg.CookieName)
		}

		if clientToken == "" || clientToken != token {
			_ = c.JSON(http.StatusForbidden, map[string]string{
				"error": "Invalid CSRF token",
			})
			return
		}

		c.Next()
	}
}
