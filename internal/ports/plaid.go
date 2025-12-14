package ports

import (
	"context"

	"github.com/alexchny/sync-relay/internal/domain"
)

type SyncResponse struct {
	Added 		[]*domain.Transaction
	Modified	[]*domain.Transaction
	Removed 	[]string

	NextCursor 	string
	HasMore		bool
}

type PlaidClient interface {
	FetchSyncUpdates(ctx context.Context, accessToken, cursor string) (*SyncResponse, error)
}
