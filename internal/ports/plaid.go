package ports

import (
	"context"
	"errors"

	"github.com/alexchny/sync-relay/internal/domain"
)

var ErrCursorReset = errors.New("plaid cursor reset required")

var ErrUserActionRequired = errors.New("user action required")

var ErrInvalidToken = errors.New("invalid or expired access token")

type SyncResponse struct {
	Added    []*domain.Transaction
	Modified []*domain.Transaction
	Removed  []string

	NextCursor string
	HasMore    bool
}

type TokenExchangeResponse struct {
	AccessToken string
	ItemID      string
}

type PlaidClient interface {
	FetchSyncUpdates(ctx context.Context, accessToken, cursor string) (*SyncResponse, error)
	ExchangePublicToken(ctx context.Context, publicToken string) (*TokenExchangeResponse, error)
}
