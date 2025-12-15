package plaid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/plaid/plaid-go/v20/plaid"
)

type Adapter struct {
	client *plaid.APIClient
}

func NewAdapter(clientID, secret, env string) *Adapter {
	configuration := plaid.NewConfiguration()
	configuration.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	configuration.AddDefaultHeader("PLAID-SECRET", secret)

	switch env {
	case "production":
		configuration.UseEnvironment(plaid.Production)
	case "development":
		configuration.UseEnvironment(plaid.Development)
	default:
		configuration.UseEnvironment(plaid.Sandbox)
	}

	client := plaid.NewAPIClient(configuration)
	return &Adapter{client: client}
}

func (a *Adapter) FetchSyncUpdates(ctx context.Context, accessToken, cursor string) (*ports.SyncResponse, error) {
	request := plaid.NewTransactionsSyncRequest(accessToken)
	if cursor != "" {
		request.SetCursor(cursor)
	}
	request.SetCount(500)

	resp, httpResp, err := a.client.PlaidApi.TransactionsSync(ctx).TransactionsSyncRequest(*request).Execute()
	if httpResp != nil && httpResp.Body != nil {
		defer func() { _ = httpResp.Body.Close() }()
	}
	if err != nil {
		var plaidErr plaid.GenericOpenAPIError
		if errors.As(err, &plaidErr) {
			var errModel plaid.PlaidError
			if jsonErr := json.Unmarshal(plaidErr.Body(), &errModel); jsonErr == nil {
				code := errModel.GetErrorCode()

				switch code {
				case "TRANSACTIONS_SYNC_MUTATION_LIMIT_EXCEEDED":
					return nil, ports.ErrCursorReset
				case "ITEM_LOGIN_REQUIRED",
					"ITEM_LOCKED",
					"USER_SETUP_REQUIRED",
					"INVALID_ACCESS_TOKEN",
					"ITEM_NOT_FOUND":
					return nil, ports.ErrUserActionRequired
				}
			}
		}

		return nil, fmt.Errorf("plaid sync failed: %w", err)
	}

	syncResp := &ports.SyncResponse{
		NextCursor: resp.GetNextCursor(),
		HasMore:    resp.GetHasMore(),
		Added:      make([]*domain.Transaction, 0, len(resp.GetAdded())),
		Modified:   make([]*domain.Transaction, 0, len(resp.GetModified())),
		Removed:    make([]string, 0, len(resp.GetRemoved())),
	}

	for _, pTx := range resp.GetAdded() {
		tx, err := a.mapToDomain(pTx)
		if err != nil {
			return nil, fmt.Errorf("failed to map added transaction %s: %w", pTx.GetTransactionId(), err)
		}
		syncResp.Added = append(syncResp.Added, tx)
	}

	for _, pTx := range resp.GetModified() {
		tx, err := a.mapToDomain(pTx)
		if err != nil {
			return nil, fmt.Errorf("failed to map modified transaction %s: %w", pTx.GetTransactionId(), err)
		}
		syncResp.Modified = append(syncResp.Modified, tx)
	}

	for _, rTx := range resp.GetRemoved() {
		syncResp.Removed = append(syncResp.Removed, rTx.GetTransactionId())
	}

	return syncResp, nil
}

func (a *Adapter) mapToDomain(pTx plaid.Transaction) (*domain.Transaction, error) {
	var pendingID *string
	if val, ok := pTx.GetPendingTransactionIdOk(); ok && val != nil {
		pendingID = val
	}

	status := domain.TransactionStatusPosted
	if pTx.GetPending() {
		status = domain.TransactionStatusPending
	}

	amountCents := int64(math.Round(pTx.GetAmount() * 100))

	currency := "USD"
	if val, ok := pTx.GetIsoCurrencyCodeOk(); ok && val != nil {
		currency = *val
	}

	dateStr := pTx.GetDate()
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format '%s': %w", dateStr, err)
	}

	merchantName := pTx.GetMerchantName()
	if merchantName == "" {
		merchantName = pTx.GetName()
	}

	rawPayload, err := json.Marshal(pTx)
	if err != nil {
		rawPayload = []byte("{}")
	}

	return &domain.Transaction{
		PlaidTransactionID: pTx.GetTransactionId(),
		PlaidPendingID:     pendingID,
		AmountCents:        amountCents,
		CurrencyCode:       currency,
		MerchantName:       merchantName,
		Date:               date,
		Status:             status,
		RawPayload:         rawPayload,
	}, nil
}
