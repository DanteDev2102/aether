package middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/DantDev2102/aether"
)

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

func CORSMiddleware[T any](cfg CORSConfig) aether.HandlerFunc[T] {
	allowOrigins := "*"
	if len(cfg.AllowOrigins) > 0 {
		allowOrigins = strings.Join(cfg.AllowOrigins, ", ")
	}

	allowMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD"
	if len(cfg.AllowMethods) > 0 {
		allowMethods = strings.Join(cfg.AllowMethods, ", ")
	}

	allowHeaders := "Origin, Content-Type, Accept, Authorization, X-Requested-With"
	if len(cfg.AllowHeaders) > 0 {
		allowHeaders = strings.Join(cfg.AllowHeaders, ", ")
	}

	exposeHeaders := ""
	if len(cfg.ExposeHeaders) > 0 {
		exposeHeaders = strings.Join(cfg.ExposeHeaders, ", ")
	}

	maxAge := fmt.Sprintf("%d", cfg.MaxAge)

	return func(c *aether.Context[T]) {
		h := c.Res().Header()

		h.Set("Access-Control-Allow-Origin", allowOrigins)
		h.Set("Access-Control-Allow-Methods", allowMethods)
		h.Set("Access-Control-Allow-Headers", allowHeaders)

		if exposeHeaders != "" {
			h.Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge > 0 {
			h.Set("Access-Control-Max-Age", maxAge)
		}

		if c.Req().Method == http.MethodOptions {
			c.Res().WriteHeader(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
