package ports

import (
	"context"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, time.Duration, error)
	Wait(ctx context.Context, key string) error
}
