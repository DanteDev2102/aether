package aether

import "context"

type CacheStore interface {
	Get(ctx context.Context, key string) (any, bool)
	Set(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}
