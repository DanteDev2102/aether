package aether

import (
	"context"
	"time"

	"github.com/maypok86/otter"
)

// OtterStore is a cache store implementation using the Otter library.
type OtterStore struct {
	cache otter.Cache[string, any]
}

// NewOtterStore creates a new OtterStore with the specified maximum size.
func NewOtterStore(maxSize int) (*OtterStore, error) {
	if maxSize <= 0 {
		maxSize = 10000
	}
	cache, err := otter.MustBuilder[string, any](maxSize).
		WithTTL(5 * time.Minute).
		Build()
	if err != nil {
		return nil, err
	}
	return &OtterStore{cache: cache}, nil
}

// Get retrieves a value from the cache by key.
func (o *OtterStore) Get(_ context.Context, key string) (any, bool) {
	return o.cache.Get(key)
}

// Set stores a value in the cache with the specified key.
func (o *OtterStore) Set(_ context.Context, key string, value any) error {
	o.cache.Set(key, value)
	return nil
}

// Delete removes a value from the cache by key.
func (o *OtterStore) Delete(_ context.Context, key string) error {
	o.cache.Delete(key)
	return nil
}

// Clear removes all entries from the cache.
func (o *OtterStore) Clear(_ context.Context) error {
	o.cache.Clear()
	return nil
}
