package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/DantDev2102/aether"
)

type BasicAuthConfig struct {
	Users    map[string]string
	Validate func(user, password string) bool
	Realm    string
}

func BasicAuthMiddleware[T any](cfg BasicAuthConfig) aether.HandlerFunc[T] {
	realm := cfg.Realm
	if realm == "" {
		realm = "Restricted"
	}

	return func(c *aether.Context[T]) {
		user, pass, ok := c.Req().BasicAuth()
		if !ok {
			c.Res().Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		valid := false

		if cfg.Validate != nil {
			valid = cfg.Validate(user, pass)
		} else if cfg.Users != nil {
			if expectedPass, exists := cfg.Users[user]; exists {
				userHash := sha256.Sum256([]byte(pass))
				expectedHash := sha256.Sum256([]byte(expectedPass))
				if subtle.ConstantTimeCompare(userHash[:], expectedHash[:]) == 1 {
					valid = true
				}
			}
		}

		if !valid {
			c.Res().Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid credentials",
			})
			return
		}

		c.Next()
	}
}
