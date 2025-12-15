package redis

import (
	"context"
	"fmt"
	"time"
)

type RateLimiter struct {
	client *Client
	limit  int
	window time.Duration
}

func NewRateLimiter(client *Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	bucket := time.Now().Truncate(r.window).Format(time.RFC3339)
	redisKey := fmt.Sprintf("rate_limit:%s:%s", key, bucket)

	pipe := r.client.rdb.Pipeline()

	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, r.window+(10*time.Second))

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("redis pipeline error: %w", err)
	}

	count := incr.Val()

	if count > int64(r.limit) {
		nextWindow := time.Now().Truncate(r.window).Add(r.window)
		wait := time.Until(nextWindow)
		return false, wait, nil
	}

	return true, 0, nil
}

func (r *RateLimiter) Wait(ctx context.Context, key string) error {
	for {
		allowed, wait, err := r.Allow(ctx, key)
		if err != nil {
			return err
		}

		if allowed {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			continue
		}
	}
}
