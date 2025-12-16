package plaid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexchny/sync-relay/internal/ports"
)

func (a *Adapter) VerifyWebhook(ctx context.Context, req *http.Request) (*ports.WebhookPayload, error) {
	// method validation
	if req.Method != http.MethodPost {
		return nil, fmt.Errorf("invalid method: %s", req.Method)
	}

	// content-type validation
	ct := req.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return nil, fmt.Errorf("invalid content-type: %s", ct)
	}

	// plaid-veriff header validation
	jwt := req.Header.Get("Plaid-Verification")
	if jwt == "" {
		return nil, fmt.Errorf("missing Plaid-Verification header")
	}

	// TODO: add full cryptographic verification for JWT

	// doS protection
	req.Body = http.MaxBytesReader(nil, req.Body, 1048576)

	// parse body
	var payload ports.WebhookPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode webhook body: %w", err)
	}

	// logic validation
	if payload.ItemID == "" {
		return nil, fmt.Errorf("webhook payload missing item_id")
	}

	return &payload, nil
}
