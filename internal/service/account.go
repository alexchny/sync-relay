package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/google/uuid"
)

var ErrTokenAlreadyUsed = errors.New("public token already used")
var ErrItemAlreadyLinked = errors.New("item already linked")

type AccountService struct {
	plaidClient ports.PlaidClient
	itemRepo    ports.ItemRepository
	queue       ports.JobQueue
}

func NewAccountService(p ports.PlaidClient, r ports.ItemRepository, q ports.JobQueue) *AccountService {
	return &AccountService{
		plaidClient: p,
		itemRepo:    r,
		queue:       q,
	}
}

func (s *AccountService) CreateLinkToken(ctx context.Context, userID string) (string, error) {
	return s.plaidClient.CreateLinkToken(ctx, userID)
}

func (s *AccountService) LinkItem(ctx context.Context, tenantID uuid.UUID, publicToken string) (uuid.UUID, error) {
	tokenResp, err := s.plaidClient.ExchangePublicToken(ctx, publicToken)
	if err != nil {
		if errors.Is(err, ports.ErrInvalidToken) {
			slog.Info("public token already used or invalid", "tenant_id", tenantID)
			return uuid.Nil, ErrTokenAlreadyUsed
		}
		return uuid.Nil, fmt.Errorf("token exchange failed: %w", err)
	}

	itemID := uuid.New()
	item := &domain.Item{
		ID:             itemID,
		TenantID:       tenantID,
		PlaidItemID:    tokenResp.ItemID,
		AccessTokenEnc: tokenResp.AccessToken,
		SyncStatus:     "active",
		NextCursor:     "",
	}

	if err := s.itemRepo.Create(ctx, item); err != nil {
		if errors.Is(err, ports.ErrItemAlreadyExists) {
			slog.Info("item already exists, fetching existing", "plaid_item_id", tokenResp.ItemID)
			
			// fetch existing item (makes operation idempotent)
			existingItem, fetchErr := s.itemRepo.GetByPlaidItemID(ctx, tokenResp.ItemID)
			if fetchErr != nil {
				return uuid.Nil, fmt.Errorf("item exists but failed to fetch: %w", fetchErr)
			}
			
			return existingItem.ID, ErrItemAlreadyLinked
		}
		return uuid.Nil, fmt.Errorf("failed to save item: %w", err)
	}

	job := &domain.SyncJob{
		ItemID:  itemID,
		JobType: domain.JobTypeStandard,
		TraceID: uuid.NewString(),
	}
	if err := s.queue.Enqueue(ctx, job); err != nil {
		slog.Error("failed to enqueue initial sync", "item_id", itemID, "error", err)
	}

	slog.Info("item linked successfully", "item_id", itemID, "plaid_item_id", tokenResp.ItemID)
	return itemID, nil
}
