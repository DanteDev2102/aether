package middlewares

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/DanteDev2102/aether"
)

type contextKey string

// ClaimsContextKey is the context key for storing JWT claims.
const ClaimsContextKey contextKey = "aether_jwt_claims"

// ErrMissingToken is passed to JWTConfig.ErrorHandler when the request carries
// no token.
var ErrMissingToken = errors.New("aether/middlewares: missing JWT token")

// JWTValidator defines the interface for JWT token validation.
type JWTValidator interface {
	Validate(tokenString string) (jwt.Claims, error)
}

type defaultJWTValidator struct {
	secret        []byte
	signingMethod jwt.SigningMethod
	claimsFunc    func() jwt.Claims
}

// Validate checks and validates a JWT token string.
func (v *defaultJWTValidator) Validate(tokenString string) (jwt.Claims, error) {
	claims := v.claimsFunc()
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != v.signingMethod.Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return token.Claims, nil
}

// JWTConfig holds configuration for JWT authentication middleware.
type JWTConfig[T any] struct {
	// Secret is the HMAC key used by the default validator. It is required
	// unless Validator is set.
	Secret        []byte
	SigningMethod jwt.SigningMethod
	ClaimsFunc    func() jwt.Claims
	Validator     JWTValidator
	// TokenLookup is "<source>:<name>" where source is header, query or
	// cookie. Defaults to "header:Authorization".
	TokenLookup string
	// ErrorHandler writes the response when the token is missing or invalid.
	// It receives the request context; without it a 401 JSON is sent.
	ErrorHandler func(c *aether.Context[T], err error)
}

func extractToken(req *http.Request, lookup string) string {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		parts = []string{"header", "Authorization"}
	}

	switch parts[0] {
	case "header":
		val := req.Header.Get(parts[1])
		if strings.EqualFold(parts[1], "Authorization") {
			scheme, token, ok := strings.Cut(val, " ")
			if ok && strings.EqualFold(scheme, "Bearer") {
				return strings.TrimSpace(token)
			}
		}
		return val
	case "query":
		return req.URL.Query().Get(parts[1])
	case "cookie":
		cookie, err := req.Cookie(parts[1])
		if err != nil {
			return ""
		}
		return cookie.Value
	}
	return ""
}

// JWTMiddleware provides JWT token validation and authentication.
//
// It panics when neither Secret nor Validator is configured.
func JWTMiddleware[T any](cfg JWTConfig[T]) aether.HandlerFunc[T] {
	validator := cfg.Validator
	if validator == nil {
		if len(cfg.Secret) == 0 {
			panic("aether/middlewares: JWTMiddleware requires a Secret or a Validator")
		}
		signingMethod := cfg.SigningMethod
		if signingMethod == nil {
			signingMethod = jwt.SigningMethodHS256
		}
		claimsFunc := cfg.ClaimsFunc
		if claimsFunc == nil {
			claimsFunc = func() jwt.Claims { return jwt.MapClaims{} }
		}
		validator = &defaultJWTValidator{
			secret:        cfg.Secret,
			signingMethod: signingMethod,
			claimsFunc:    claimsFunc,
		}
	}

	lookup := cfg.TokenLookup
	if lookup == "" {
		lookup = "header:Authorization"
	}

	fail := func(c *aether.Context[T], err error, msg string) {
		if cfg.ErrorHandler != nil {
			cfg.ErrorHandler(c, err)
			return
		}
		_ = c.JSON(http.StatusUnauthorized, map[string]string{"error": msg})
	}

	return func(c *aether.Context[T]) {
		tokenString := extractToken(c.Req(), lookup)
		if tokenString == "" {
			fail(c, ErrMissingToken, "Missing or invalid token")
			return
		}

		claims, err := validator.Validate(tokenString)
		if err != nil {
			fail(c, err, "Invalid or expired token")
			return
		}

		ctx := context.WithValue(c.Req().Context(), ClaimsContextKey, claims)
		c.SetReq(c.Req().WithContext(ctx))

		c.Next()
	}
}

// GetClaims retrieves JWT claims from the request context.
func GetClaims[T any](c *aether.Context[T]) jwt.Claims {
	claims, _ := c.Req().Context().Value(ClaimsContextKey).(jwt.Claims)
	return claims
}

// GetMapClaims retrieves JWT claims as a MapClaims from the request context.
func GetMapClaims[T any](c *aether.Context[T]) jwt.MapClaims {
	claims, _ := c.Req().Context().Value(ClaimsContextKey).(jwt.MapClaims)
	return claims
}
