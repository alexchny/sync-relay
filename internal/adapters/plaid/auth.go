package plaid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/plaid/plaid-go/v20/plaid"
)

func (a *Adapter) ExchangePublicToken(ctx context.Context, publicToken string) (*ports.TokenExchangeResponse, error) {
	if publicToken == "" {
		return nil, fmt.Errorf("public token cannot be empty")
	}

	request := plaid.NewItemPublicTokenExchangeRequest(publicToken)

	resp, httpResp, err := a.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*request).Execute()
	if httpResp != nil && httpResp.Body != nil {
		defer func() { _ = httpResp.Body.Close() }()
	}
	if err != nil {
		var plaidErr plaid.GenericOpenAPIError
		if errors.As(err, &plaidErr) {
			var errModel plaid.PlaidError
			if jsonErr := json.Unmarshal(plaidErr.Body(), &errModel); jsonErr == nil {
				if errModel.GetErrorCode() == "INVALID_PUBLIC_TOKEN" {
					return nil, ports.ErrInvalidToken
				}
			}
		}
		return nil, fmt.Errorf("plaid token exchange failed: %w", err)
	}

	return &ports.TokenExchangeResponse{
		AccessToken: resp.GetAccessToken(),
		ItemID:      resp.GetItemId(),
	}, nil
}
