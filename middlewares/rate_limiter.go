package middlewares

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DanteDev2102/aether"
)

// RateLimiterStore defines the interface for rate limiting storage.
type RateLimiterStore interface {
	Increment(key string, window time.Duration) (int, error)
}

// MemStoreItem represents a rate limiter entry with count and expiration.
type MemStoreItem struct {
	mu        sync.Mutex
	Count     int
	ExpiresAt time.Time
}

// MemoryRateLimiterStore is an in-memory implementation of RateLimiterStore.
// Expired entries are swept lazily, at most once per window, so memory stays
// bounded by the number of clients seen within one window.
type MemoryRateLimiterStore struct {
	m         sync.Map
	lastSweep atomic.Int64
}

// NewMemoryRateLimiterStore creates a new in-memory rate limiter store.
func NewMemoryRateLimiterStore() *MemoryRateLimiterStore {
	s := &MemoryRateLimiterStore{}
	s.lastSweep.Store(time.Now().UnixNano())
	return s
}

// Increment increases the counter for the given key within the time window.
func (s *MemoryRateLimiterStore) Increment(key string, window time.Duration) (int, error) {
	now := time.Now()
	s.maybeSweep(now, window)

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

func (s *MemoryRateLimiterStore) maybeSweep(now time.Time, window time.Duration) {
	last := s.lastSweep.Load()
	if now.UnixNano()-last < int64(window) || !s.lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	s.m.Range(func(key, val any) bool {
		item := val.(*MemStoreItem)
		item.mu.Lock()
		if now.After(item.ExpiresAt) {
			s.m.CompareAndDelete(key, item)
		}
		item.mu.Unlock()
		return true
	})
}

// RateLimiterConfig holds configuration for the rate limiter middleware.
type RateLimiterConfig struct {
	Limit    int
	Window   time.Duration
	Store    RateLimiterStore
	SkipFunc func(req *http.Request) bool
	// TrustProxies lists proxy IPs or CIDR ranges (e.g. "10.0.0.0/8") whose
	// X-Forwarded-For header is trusted to find the client IP.
	TrustProxies []string
}

type trustedProxies []netip.Prefix

func parseTrustedProxies(list []string) trustedProxies {
	var out trustedProxies
	for _, p := range list {
		if prefix, err := netip.ParsePrefix(p); err == nil {
			out = append(out, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(p)
		if err != nil {
			panic("aether/middlewares: invalid TrustProxies entry " + strconv.Quote(p))
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out
}

func (t trustedProxies) contains(ip string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range t {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// clientIP returns the address of the client. When the direct peer is a
// trusted proxy, X-Forwarded-For is read from right to left and the first
// address that is not a trusted proxy is used, since entries to the left can
// be forged by the client.
func (t trustedProxies) clientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		ip = req.RemoteAddr
	}
	if len(t) == 0 || !t.contains(ip) {
		return ip
	}

	hops := strings.Split(req.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		if !t.contains(hop) {
			return hop
		}
		ip = hop
	}
	return ip
}

// RateLimiterMiddleware limits request rate per client IP. Rejected requests
// get 429 with a Retry-After header.
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

	proxies := parseTrustedProxies(cfg.TrustProxies)
	limit := strconv.Itoa(cfg.Limit)
	retryAfter := strconv.Itoa(int((cfg.Window + time.Second - 1) / time.Second))

	return func(c *aether.Context[T]) {
		if cfg.SkipFunc != nil && cfg.SkipFunc(c.Req()) {
			c.Next()
			return
		}

		count, err := cfg.Store.Increment(proxies.clientIP(c.Req()), cfg.Window)
		if err != nil {
			c.Log().Errorf("RateLimiter error: %v", err)
			c.Next()
			return
		}

		h := c.Res().Header()
		h.Set("X-RateLimit-Limit", limit)
		h.Set("X-RateLimit-Remaining", strconv.Itoa(max(cfg.Limit-count, 0)))

		if count > cfg.Limit {
			h.Set("Retry-After", retryAfter)
			_ = c.JSON(http.StatusTooManyRequests, map[string]any{
				"error":   "Too Many Requests",
				"message": "Rate limit of " + limit + " exceeded.",
			})
			return
		}

		c.Next()
	}
}
