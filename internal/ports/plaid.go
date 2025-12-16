package ports

import (
	"context"
	"errors"
	"net/http"

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

type WebhookPayload struct {
	WebhookType string `json:"webhook_type"`
	WebhookCode string `json:"webhook_code"`
	ItemID      string `json:"item_id"`
	Error       *struct {
		ErrorMessage string `json:"error_message"`
		ErrorCode    string `json:"error_code"`
	} `json:"error,omitempty"`
}

type PlaidClient interface {
	FetchSyncUpdates(ctx context.Context, accessToken, cursor string) (*SyncResponse, error)
	ExchangePublicToken(ctx context.Context, publicToken string) (*TokenExchangeResponse, error)
	CreateLinkToken(ctx context.Context, userID string) (string, error)
}

type WebhookVerifier interface {
	VerifyWebhook(ctx context.Context, r *http.Request) (*WebhookPayload, error)
}
