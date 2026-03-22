package middlewares

import (
	"context"
	"net/http"
	"strings"

	"github.com/DantDev2102/aether"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const ClaimsContextKey contextKey = "aether_jwt_claims"

type JWTValidator interface {
	Validate(tokenString string) (jwt.Claims, error)
}

type defaultJWTValidator struct {
	secret        []byte
	signingMethod jwt.SigningMethod
	claimsFunc    func() jwt.Claims
}

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

type JWTConfig struct {
	Secret        []byte
	SigningMethod jwt.SigningMethod
	ClaimsFunc    func() jwt.Claims
	Validator     JWTValidator
	TokenLookup   string
	ErrorHandler  func(c *aether.Context[any], err error)
}

func extractToken(req *http.Request, lookup string) string {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		parts = []string{"header", "Authorization"}
	}

	switch parts[0] {
	case "header":
		val := req.Header.Get(parts[1])
		if parts[1] == "Authorization" && strings.HasPrefix(val, "Bearer ") {
			return strings.TrimPrefix(val, "Bearer ")
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

func JWTMiddleware[T any](cfg JWTConfig) aether.HandlerFunc[T] {
	validator := cfg.Validator
	if validator == nil {
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

	return func(c *aether.Context[T]) {
		tokenString := extractToken(c.Req(), lookup)
		if tokenString == "" {
			if cfg.ErrorHandler != nil {
				cfg.ErrorHandler(&aether.Context[any]{}, jwt.ErrTokenMalformed)
			} else {
				c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Missing or invalid token",
				})
			}
			return
		}

		claims, err := validator.Validate(tokenString)
		if err != nil {
			if cfg.ErrorHandler != nil {
				cfg.ErrorHandler(&aether.Context[any]{}, err)
			} else {
				c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Invalid or expired token",
				})
			}
			return
		}

		ctx := context.WithValue(c.Req().Context(), ClaimsContextKey, claims)
		c.SetReq(c.Req().WithContext(ctx))

		c.Next()
	}
}

func GetClaims[T any](c *aether.Context[T]) jwt.Claims {
	claims, _ := c.Req().Context().Value(ClaimsContextKey).(jwt.Claims)
	return claims
}

func GetMapClaims[T any](c *aether.Context[T]) jwt.MapClaims {
	claims, _ := c.Req().Context().Value(ClaimsContextKey).(jwt.MapClaims)
	return claims
}
