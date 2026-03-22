package aether

import (
	"context"
	"time"

	"github.com/maypok86/otter"
)

type OtterStore struct {
	cache otter.Cache[string, any]
}

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

func (o *OtterStore) Get(_ context.Context, key string) (any, bool) {
	return o.cache.Get(key)
}

func (o *OtterStore) Set(_ context.Context, key string, value any) error {
	o.cache.Set(key, value)
	return nil
}

func (o *OtterStore) Delete(_ context.Context, key string) error {
	o.cache.Delete(key)
	return nil
}

func (o *OtterStore) Clear(_ context.Context) error {
	o.cache.Clear()
	return nil
}
