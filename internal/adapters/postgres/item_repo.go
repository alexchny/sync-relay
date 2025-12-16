package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type ItemRepo struct {
	db *DB
}

func NewItemRepo(db *DB) *ItemRepo {
	return &ItemRepo{db: db}
}

func (r *ItemRepo) scanItem(row *sql.Row) (*domain.Item, error) {
	var item domain.Item
	var lastSyncedAt sql.NullTime
	var errorMessage sql.NullString

	err := row.Scan(
		&item.ID,
		&item.TenantID,
		&item.PlaidItemID,
		&item.AccessTokenEnc,
		&item.SyncStatus,
		&item.NextCursor,
		&errorMessage,
		&lastSyncedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("item not found: %w", err)
	}
	if err != nil {
		return nil, err
	}

	if lastSyncedAt.Valid {
		t := lastSyncedAt.Time
		item.LastSyncedAt = &t
	}

	if errorMessage.Valid {
		item.ErrorMessage = errorMessage.String
	}

	return &item, nil
}

func (r *ItemRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Item, error) {
	query := `
		SELECT 
			id, tenant_id, plaid_item_id, access_token_enc, 
			sync_status, next_cursor, error_message, last_synced_at, 
			created_at, updated_at
		FROM items WHERE id = $1
	`

	return r.scanItem(r.db.QueryRowContext(ctx, query, id))
}

func (r *ItemRepo) GetByPlaidItemID(ctx context.Context, plaidItemID string) (*domain.Item, error) {
	query := `
		SELECT 
			id, tenant_id, plaid_item_id, access_token_enc, 
			sync_status, next_cursor, error_message, last_synced_at, 
			created_at, updated_at
		FROM items WHERE plaid_item_id = $1
	`
	return r.scanItem(r.db.QueryRowContext(ctx, query, plaidItemID))
}

func (r *ItemRepo) Create(ctx context.Context, item *domain.Item) error {
	query := `
		INSERT INTO items (
			id, tenant_id, plaid_item_id, access_token_enc, 
			sync_status, next_cursor, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query,
		item.ID,
		item.TenantID,
		item.PlaidItemID,
		item.AccessTokenEnc,
		item.SyncStatus,
		item.NextCursor,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ports.ErrItemAlreadyExists
		}
		return fmt.Errorf("failed to create item: %w", err)
	}
	return nil
}

func (r *ItemRepo) UpdateSuccess(ctx context.Context, id uuid.UUID, cursor string) error {
	query := `
		UPDATE items 
		SET next_cursor = $1, 
		    sync_status = 'active', 
		    error_message = NULL,
		    last_synced_at = NOW(), 
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, cursor, id)
	return err
}

func (r *ItemRepo) MarkResyncing(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE items 
		SET sync_status = 'resyncing', 
		    updated_at = NOW() 
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ItemRepo) MarkError(ctx context.Context, id uuid.UUID, syncErr error) error {
	var errText string
	if syncErr != nil {
		errText = syncErr.Error()
	} else {
		errText = "unknown error"
	}

	query := `
		UPDATE items 
		SET sync_status = 'error', 
		    error_message = $1,
		    updated_at = NOW() 
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, errText, id)
	return err
}
