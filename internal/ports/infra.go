package ports

import (
	"context"
	"time"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/google/uuid"
)

type DistributedLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func() error, err error)
}

type EventPublisher interface {
	PublishSyncEvents(ctx context.Context, itemID uuid.UUID, added, modified []*domain.Transaction, removedIDs []string) error
}
