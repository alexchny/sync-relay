package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alexchny/sync-relay/internal/domain"
)

type QueueAdapter struct {
	client 		*Client
	queueName	string
}

func NewQueueAdapter(client *Client, queueName string) *QueueAdapter {
	return &QueueAdapter{
		client:		client,
		queueName:	queueName,
	}
}

type jobDTO struct {
	ItemID  string `json:"item_id"`
	JobType string `json:"job_type"`
	TraceID string `json:"trace_id"`
}

const queueKey = "sync_jobs"

func (q *QueueAdapter) Enqueue(ctx context.Context, job domain.SyncJob) error {
	// map domain to DTO
	dto := jobDTO{
		ItemID:  job.ItemID.String(),
		JobType: string(job.JobType),
		TraceID: job.TraceID,
	}

	// serialize
	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// push to Redis
	if err := q.client.rdb.LPush(ctx, q.queueName, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}
