package ports

import (
	"context"

	"github.com/alexchny/sync-relay/internal/domain"
)

type JobQueue interface {
	Enqueue(ctx context.Context, job domain.SyncJob) error
}