package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DantDev2102/aether"
)

type RateLimiterStore interface {
	Increment(key string, window time.Duration) (int, error)
}

type MemStoreItem struct {
	mu        sync.Mutex
	Count     int
	ExpiresAt time.Time
}

type MemoryRateLimiterStore struct {
	m sync.Map
}

func NewMemoryRateLimiterStore() *MemoryRateLimiterStore {
	return &MemoryRateLimiterStore{}
}

func (s *MemoryRateLimiterStore) Increment(key string, window time.Duration) (int, error) {
	now := time.Now()

	val, _ := s.m.LoadOrStore(key, &MemStoreItem{
		ExpiresAt: now.Add(window),
	})

	item := val.(*MemStoreItem)

	item.mu.Lock()
	defer item.mu.Unlock()

	if now.After(item.ExpiresAt) {
		item.Count = 1
		item.ExpiresAt = now.Add(window)
		return 1, nil
	}

	item.Count++
	return item.Count, nil
}

type RateLimiterConfig struct {
	Limit        int
	Window       time.Duration
	Store        RateLimiterStore
	SkipFunc     func(req *http.Request) bool
	TrustProxies []string
}

func RateLimiterMiddleware[T any](cfg RateLimiterConfig) aether.HandlerFunc[T] {
	if cfg.Store == nil {
		cfg.Store = NewMemoryRateLimiterStore()
	}
	if cfg.Limit == 0 {
		cfg.Limit = 100
	}
	if cfg.Window == 0 {
		cfg.Window = 1 * time.Minute
	}

	return func(c *aether.Context[T]) {
		if cfg.SkipFunc != nil && cfg.SkipFunc(c.Req()) {
			c.Next()
			return
		}

		key := c.Req().RemoteAddr
		if idx := strings.LastIndex(key, ":"); idx != -1 {
			key = key[:idx]
		}

		if len(cfg.TrustProxies) > 0 {
			fromTrusted := false
			for _, proxy := range cfg.TrustProxies {
				if key == proxy {
					fromTrusted = true
					break
				}
			}

			if fromTrusted {
				if xff := c.Req().Header.Get("X-Forwarded-For"); xff != "" {
					parts := strings.Split(xff, ",")
					key = strings.TrimSpace(parts[0])
				}
			}
		}

		count, err := cfg.Store.Increment(key, cfg.Window)
		if err != nil {
			c.Log().Errorf("RateLimiter error: %v", err)
			c.Next()
			return
		}

		if count > cfg.Limit {
			c.JSON(http.StatusTooManyRequests, map[string]any{
				"error":   "Too Many Requests",
				"message": fmt.Sprintf("Rate limit of %d exceeded.", cfg.Limit),
			})
			return
		}

		c.Res().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.Limit))
		c.Res().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", cfg.Limit-count))

		c.Next()
	}
}
