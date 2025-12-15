package redis

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrLockBusy = errors.New("lock is already acquired")

type LockAdapter struct {
	client *Client
}

func NewLockAdapter(client *Client) *LockAdapter {
	return &LockAdapter{client: client}
}

func (l *LockAdapter) Acquire(ctx context.Context, key string, ttl time.Duration) (func() error, error) {
	token := uuid.NewString()

	success, err := l.client.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !success {
		return nil, ErrLockBusy
	}

	release := func() error {
		const script = `
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end
		`

		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := l.client.rdb.Eval(releaseCtx, script, []string{key}, token).Err()
		return err
	}

	return release, nil
}
