package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type TransactionRepo struct {
	db *DB
}

func NewTransactionRepo(db *DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) UpsertBatch(ctx context.Context, txs []*domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	values := []interface{}{}
	placeholders := []string{}

	const paramsPerTx = 9

	for i, tx := range txs {
		base := i * paramsPerTx

		row := fmt.Sprintf(
			"(gen_random_uuid(), $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW(), NOW())",
			base+1, base+2, base+3, base+4, base+5,
			base+6, base+7, base+8, base+9,
		)
		placeholders = append(placeholders, row)

		values = append(values,
			tx.ItemID,
			tx.PlaidTransactionID,
			tx.PlaidPendingID,
			tx.AmountCents,
			tx.CurrencyCode,
			tx.Date,
			tx.MerchantName,
			tx.Status,
			tx.RawPayload,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO transactions (
			id, 
			item_id, 
			plaid_transaction_id, 
			plaid_pending_id,
			amount_cents,
			currency_code,
			date,
			merchant_name,
			status,
			raw_payload,
			created_at,
			updated_at
		)
		VALUES %s
		ON CONFLICT (plaid_transaction_id) DO UPDATE SET
			amount_cents = EXCLUDED.amount_cents,
			currency_code = EXCLUDED.currency_code,
			date = EXCLUDED.date,
			merchant_name = EXCLUDED.merchant_name,
			plaid_pending_id = EXCLUDED.plaid_pending_id,
			status = EXCLUDED.status,
			raw_payload = EXCLUDED.raw_payload,
			is_removed = FALSE,
			updated_at = NOW()
	`, strings.Join(placeholders, ","))

	// execute
	if _, err := r.db.ExecContext(ctx, query, values...); err != nil {
		return fmt.Errorf("failed to upsert batch: %w", err)
	}

	return nil
}

func (r *TransactionRepo) MarkRemovedBatch(ctx context.Context, itemID uuid.UUID, plaidTxIDs []string) error {
	if len(plaidTxIDs) == 0 {
		return nil
	}

	query := `
		UPDATE transactions 
		SET is_removed = TRUE, updated_at = NOW()
		WHERE item_id = $1 AND plaid_transaction_id = ANY($2)
	`

	if _, err := r.db.ExecContext(ctx, query, itemID, pq.Array(plaidTxIDs)); err != nil {
		return fmt.Errorf("failed to mark removed: %w", err)
	}

	return nil
}
