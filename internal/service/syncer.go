package service

import (
	"context"
	"fmt"
	"time"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/google/uuid"
)

type Syncer struct {
	itemRepo      ports.ItemRepository
	txRepo        ports.TransactionRepository
	plaid         ports.PlaidClient
	lock          ports.DistributedLock
	publisher     ports.EventPublisher
	globalLimiter ports.RateLimiter
	itemLimiter   ports.RateLimiter
}

func NewSyncer(
	itemRepo ports.ItemRepository,
	txRepo ports.TransactionRepository,
	plaid ports.PlaidClient,
	lock ports.DistributedLock,
	publisher ports.EventPublisher,
	globalLimiter ports.RateLimiter,
	itemLimiter ports.RateLimiter,
) *Syncer {
	return &Syncer{
		itemRepo:      itemRepo,
		txRepo:        txRepo,
		plaid:         plaid,
		lock:          lock,
		publisher:     publisher,
		globalLimiter: globalLimiter,
		itemLimiter:   itemLimiter,
	}
}

func (s *Syncer) SyncItem(ctx context.Context, itemID uuid.UUID) error {
	// acquire lock
	lockKey := fmt.Sprintf("sync:lock:%s", itemID)
	release, err := s.lock.Acquire(ctx, lockKey, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for item %s: %w", itemID, err)
	}

	defer func() {
		if err := release(); err != nil {
			_ = err
		}
	}()

	// global rate limit
	if err := s.globalLimiter.Wait(ctx, "plaid_client"); err != nil {
		return fmt.Errorf("global rate limit error: %w", err)
	}

	// item rate limit
	itemKey := fmt.Sprintf("plaid_item:%s", itemID)
	if err := s.itemLimiter.Wait(ctx, itemKey); err != nil {
		return fmt.Errorf("item rate limit error: %w", err)
	}

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
			if err := s.publisher.PublishSyncEvents(ctx, item.ID, resp.Added, resp.Modified, resp.Removed); err != nil {
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
