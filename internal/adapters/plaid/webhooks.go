package plaid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type WebhookPayload struct {
	WebhookType string `json:"webhook_type"`
	WebhookCode string `json:"webhook_code"`
	ItemID      string `json:"item_id"`
	Error       *struct {
		ErrorMessage string `json:"error_message"`
		ErrorCode    string `json:"error_code"`
	} `json:"error,omitempty"`
}

func (a *Adapter) VerifyWebhook(ctx context.Context, req *http.Request) (*WebhookPayload, error) {
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
	var payload WebhookPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode webhook body: %w", err)
	}

	// logic validation
	if payload.ItemID == "" {
		return nil, fmt.Errorf("webhook payload missing item_id")
	}

	return &payload, nil
}
