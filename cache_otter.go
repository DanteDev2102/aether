package aether

import (
	"context"
	"time"

	"github.com/maypok86/otter/v2"
)

// DefaultCacheMaxSize is the default maximum number of entries in an OtterStore.
const DefaultCacheMaxSize = 10_000

// DefaultCacheTTL is the default time an OtterStore entry lives after it is
// written.
const DefaultCacheTTL = 5 * time.Minute

// OtterConfig holds configuration for an OtterStore.
type OtterConfig struct {
	// MaxSize is the maximum number of entries. Defaults to DefaultCacheMaxSize.
	MaxSize int
	// TTL is how long an entry lives after it is written. Zero uses
	// DefaultCacheTTL; a negative value disables expiration.
	TTL time.Duration
}

// OtterStore is a cache store implementation using the Otter library.
type OtterStore struct {
	cache *otter.Cache[string, any]
}

// NewOtterStore creates a new OtterStore with the given configuration.
func NewOtterStore(cfg OtterConfig) (*OtterStore, error) {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = DefaultCacheMaxSize
	}
	if cfg.TTL == 0 {
		cfg.TTL = DefaultCacheTTL
	}

	opts := &otter.Options[string, any]{MaximumSize: cfg.MaxSize}
	if cfg.TTL > 0 {
		opts.ExpiryCalculator = otter.ExpiryWriting[string, any](cfg.TTL)
	}

	cache, err := otter.New(opts)
	if err != nil {
		return nil, err
	}
	return &OtterStore{cache: cache}, nil
}

// Get retrieves a value from the cache by key.
func (o *OtterStore) Get(_ context.Context, key string) (any, bool) {
	return o.cache.GetIfPresent(key)
}

// Set stores a value in the cache with the specified key.
func (o *OtterStore) Set(_ context.Context, key string, value any) error {
	o.cache.Set(key, value)
	return nil
}

// Delete removes a value from the cache by key.
func (o *OtterStore) Delete(_ context.Context, key string) error {
	o.cache.Invalidate(key)
	return nil
}

// Clear removes all entries from the cache.
func (o *OtterStore) Clear(_ context.Context) error {
	o.cache.InvalidateAll()
	return nil
}
