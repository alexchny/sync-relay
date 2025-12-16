package ports

import (
	"context"
	"errors"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/google/uuid"
)

var ErrItemAlreadyExists = errors.New("item already exists")

type ItemRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Item, error)
	GetByPlaidItemID(ctx context.Context, plaidItemID string) (*domain.Item, error)
	Create(ctx context.Context, item *domain.Item) error
	UpdateSuccess(ctx context.Context, id uuid.UUID, cursor string) error
	MarkResyncing(ctx context.Context, id uuid.UUID) error
	MarkError(ctx context.Context, id uuid.UUID, err error) error
}

type TransactionRepository interface {
	UpsertBatch(ctx context.Context, txs []*domain.Transaction) error
	MarkRemovedBatch(ctx context.Context, itemID uuid.UUID, plaidTXIDs []string) error
	DeleteAllForItem(ctx context.Context, itemID uuid.UUID) error
}
