package middlewares

import (
	"fmt"
	
	"github.com/DantDev2102/aether"
)

type HelmetConfig struct {
	XSSProtection         string
	ContentTypeNosniff    string
	XFrameOptions         string
	HSTSMaxAge            int
	HSTSExcludeSubdomains bool
	ContentSecurityPolicy string
	ReferrerPolicy        string
}

func DefaultHelmetConfig() HelmetConfig {
	return HelmetConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: false,
	}
}

func HelmetMiddleware[T any](cfg HelmetConfig) aether.HandlerFunc[T] {
	return func(c *aether.Context[T]) {
		h := c.Res().Header()

		if cfg.XSSProtection != "" {
			h.Set("X-XSS-Protection", cfg.XSSProtection)
		}
		if cfg.ContentTypeNosniff != "" {
			h.Set("X-Content-Type-Options", cfg.ContentTypeNosniff)
		}
		if cfg.XFrameOptions != "" {
			h.Set("X-Frame-Options", cfg.XFrameOptions)
		}
		if cfg.HSTSMaxAge > 0 {
			val := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
			if !cfg.HSTSExcludeSubdomains {
				val += "; includeSubDomains"
			}
			h.Set("Strict-Transport-Security", val)
		}
		if cfg.ContentSecurityPolicy != "" {
			h.Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
		}
		if cfg.ReferrerPolicy != "" {
			h.Set("Referrer-Policy", cfg.ReferrerPolicy)
		}

		c.Next()
	}
}
