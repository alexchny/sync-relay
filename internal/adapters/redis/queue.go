package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type QueueAdapter struct {
	client   *Client
	queueKey string
}

func NewQueueAdapter(client *Client, queueKey string) *QueueAdapter {
	return &QueueAdapter{
		client:   client,
		queueKey: queueKey,
	}
}

func (q *QueueAdapter) Dequeue(ctx context.Context, timeout time.Duration) (*domain.SyncJob, error) {
	result, err := q.client.rdb.BLPop(ctx, timeout, q.queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis blpop failed: %w", err)
	}

	var job domain.SyncJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

func (q *QueueAdapter) Enqueue(ctx context.Context, job *domain.SyncJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.rdb.RPush(ctx, q.queueKey, data).Err()
}

func (q *QueueAdapter) PublishSyncEvents(ctx context.Context, itemID uuid.UUID, added, modified []*domain.Transaction, removed []string) error {
	event := map[string]interface{}{
		"type":      "SYNC_UPDATES",
		"item_id":   itemID,
		"counts":    map[string]int{"added": len(added), "modified": len(modified), "removed": len(removed)},
		"timestamp": time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return q.client.rdb.Publish(ctx, "sync-events", data).Err()
}
