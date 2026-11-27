package middlewares

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/DanteDev2102/aether"
)

// CORSConfig holds configuration for Cross-Origin Resource Sharing middleware.
type CORSConfig struct {
	// AllowOrigins lists the allowed origins. Empty or "*" allows any origin.
	AllowOrigins []string
	// AllowOriginFunc, when set, decides whether an origin is allowed and
	// takes precedence over AllowOrigins.
	AllowOriginFunc  func(origin string) bool
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

type corsPolicy struct {
	anyOrigin     bool
	origins       map[string]struct{}
	originFunc    func(string) bool
	allowMethods  string
	allowHeaders  string
	exposeHeaders string
	credentials   bool
	maxAge        string
}

func newCORSPolicy(cfg CORSConfig) *corsPolicy {
	p := &corsPolicy{
		origins:       make(map[string]struct{}, len(cfg.AllowOrigins)),
		originFunc:    cfg.AllowOriginFunc,
		allowMethods:  "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD",
		allowHeaders:  "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		exposeHeaders: strings.Join(cfg.ExposeHeaders, ", "),
		credentials:   cfg.AllowCredentials,
	}

	for _, o := range cfg.AllowOrigins {
		if o == "*" {
			p.anyOrigin = true
		}
		p.origins[strings.ToLower(strings.TrimRight(o, "/"))] = struct{}{}
	}
	if len(cfg.AllowOrigins) == 0 && cfg.AllowOriginFunc == nil {
		p.anyOrigin = true
	}
	if p.anyOrigin && p.credentials && p.originFunc == nil {
		panic("aether/middlewares: CORS AllowCredentials cannot be used with a wildcard origin; list the allowed origins or set AllowOriginFunc")
	}

	if len(cfg.AllowMethods) > 0 {
		p.allowMethods = strings.Join(cfg.AllowMethods, ", ")
	}
	if len(cfg.AllowHeaders) > 0 {
		p.allowHeaders = strings.Join(cfg.AllowHeaders, ", ")
	}
	if cfg.MaxAge > 0 {
		p.maxAge = strconv.Itoa(cfg.MaxAge)
	}
	return p
}

// allowedOrigin returns the value for Access-Control-Allow-Origin, or "" when
// the origin is not allowed.
func (p *corsPolicy) allowedOrigin(origin string) string {
	if p.originFunc != nil {
		if p.originFunc(origin) {
			return origin
		}
		return ""
	}
	if p.anyOrigin {
		return "*"
	}
	if _, ok := p.origins[strings.ToLower(origin)]; ok {
		return origin
	}
	return ""
}

// CORSMiddleware handles Cross-Origin Resource Sharing headers. Only the
// request's origin is echoed back when it is allowed, and preflight requests
// are answered with 204 without reaching the route handler.
//
// It panics when AllowCredentials is combined with a wildcard origin, since
// browsers reject that combination.
func CORSMiddleware[T any](cfg CORSConfig) aether.HandlerFunc[T] {
	p := newCORSPolicy(cfg)

	return func(c *aether.Context[T]) {
		h := c.Res().Header()
		req := c.Req()

		if !p.anyOrigin || p.originFunc != nil {
			h.Add("Vary", "Origin")
		}

		origin := req.Header.Get("Origin")
		allowed := ""
		if origin != "" {
			allowed = p.allowedOrigin(origin)
		}

		if allowed != "" {
			h.Set("Access-Control-Allow-Origin", allowed)
			if p.credentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
		}

		preflight := req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != ""
		if !preflight {
			if allowed != "" && p.exposeHeaders != "" {
				h.Set("Access-Control-Expose-Headers", p.exposeHeaders)
			}
			c.Next()
			return
		}

		h.Add("Vary", "Access-Control-Request-Method")
		h.Add("Vary", "Access-Control-Request-Headers")
		if allowed != "" {
			h.Set("Access-Control-Allow-Methods", p.allowMethods)
			h.Set("Access-Control-Allow-Headers", p.allowHeaders)
			if p.maxAge != "" {
				h.Set("Access-Control-Max-Age", p.maxAge)
			}
		}
		c.Res().WriteHeader(http.StatusNoContent)
	}
}
