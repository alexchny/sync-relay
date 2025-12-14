package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/alexchny/sync-relay/internal/domain"
)

type DistributedLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

type EventPublisher interface {
	PublishSyncEvents(ctx context.Context, itemID uuid.UUID, added, modified []*domain.Transaction, removedIDs []string) error
}
