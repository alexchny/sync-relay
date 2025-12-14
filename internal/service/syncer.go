package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
)

type Syncer struct {
	itemRepo ports.ItemRepository
	txRepo ports.TransactionRepository
	plaid ports.PlaidClient
	lock ports.DistributedLock
	events ports.EventPublisher
}

func NewSyncer(
	itemRepo ports.ItemRepository,
	txRepo ports.TransactionRepository,
	plaid ports.PlaidClient,
	lock ports.DistributedLock,
	events ports.EventPublisher,
) *Syncer {
	return &Syncer{
		itemRepo: itemRepo,
		txRepo: txRepo,
		plaid: plaid,
		lock: lock,
		events: events,
	}
}

func (s *Syncer) SyncItem(ctx context.Context, itemID uuid.UUID) error {
	// acquire lock
	lockKey := fmt.Sprintf("sync:lock:%s", itemID)
	release, err := s.lock.Acquire(ctx, lockKey, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for item %s: %w", itemID, err)
	}

	defer release()

	// load item
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to load item: %w", err)
	}

	// domain logic checks
	if !item.CanSync() {
		return fmt.Errorf("item %s is in status '%s' and cannot sync", itemID, item.SyncStatus)
	}

	// execute sync loop
	if err := s.processSyncLoop(ctx, item); err != nil {
		_ = s.itemRepo.MarkError(ctx, item.ID, err)
		return fmt.Errorf("sync loop failed: %w", err)
	}

	return nil
}

func (s *Syncer) processSyncLoop(ctx context.Context, item *domain.Item) error {
	cursor := item.NextCursor

	for {
		// fetch from Plaid
		resp, err := s.plaid.FetchSyncUpdates(ctx, item.AccessTokenEnc, cursor)
		if err != nil {
			return err
		}

		// handle removed transactions
		if len(resp.Removed) > 0 {
			if err := s.txRepo.MarkRemovedBatch(ctx, item.ID, resp.Removed); err != nil {
				return fmt.Errorf("failed to mark removed transactions: %w", err)
			}
		}

		// handle added & modified transactions
		batchSize := len(resp.Added) + len(resp.Modified)
		if batchSize > 0 {
			batch := make([]*domain.Transaction, 0, batchSize)

			// link to Item
			for _, tx := range resp.Added {
				tx.ItemID = item.ID
				batch = append(batch, tx)
			}
			for _, tx := range resp.Modified {
				tx.ItemID = item.ID
				batch = append(batch, tx)
			}

			// batch upsert
			if err := s.txRepo.UpsertBatch(ctx, batch); err != nil {
				return fmt.Errorf("failed to upsert batch: %w", err)
			}
		}

		// publish events
		if batchSize > 0 || len(resp.Removed) > 0 {
			if err := s.events.PublishSyncEvents(ctx, item.ID, resp.Added, resp.Modified, resp.Removed); err != nil {
				// stop sync if failed event emits
				return fmt.Errorf("failed to publish events: %w", err)
			}
		}

		// save cursor
		if err := s.itemRepo.UpdateSuccess(ctx, item.ID, resp.NextCursor); err != nil {
			return fmt.Errorf("failed to update cursor: %w", err)
		}

		// pagination check
		if !resp.HasMore {
			break
		}

		// increment cursor
		cursor = resp.NextCursor
	}

	return nil
}
