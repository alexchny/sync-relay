package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alexchny/sync-relay/internal/service"
	"github.com/google/uuid"
)

type AccountHandler struct {
	service *service.AccountService
}

func NewAccountHandler(s *service.AccountService) *AccountHandler {
	return &AccountHandler{service: s}
}

func (h *AccountHandler) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := "00000000-0000-0000-0000-000000000001"

	token, err := h.service.CreateLinkToken(r.Context(), userID)
	if err != nil {
		slog.Error("failed to create link token", "user_id", userID, "error", err)
		http.Error(w, "failed to create link token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"link_token": token,
	})
}

func (h *AccountHandler) ConnectItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PublicToken string `json:"public_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.PublicToken == "" {
		http.Error(w, "public_token is required", http.StatusBadRequest)
		return
	}

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	itemID, err := h.service.LinkItem(r.Context(), tenantID, req.PublicToken)
	if err != nil {
		// token already used
		if errors.Is(err, service.ErrTokenAlreadyUsed) {
			slog.Info("token already used", "tenant_id", tenantID)
			http.Error(w, "this connection is already being processed", http.StatusConflict)
			return
		}

		// item already linked
		if errors.Is(err, service.ErrItemAlreadyLinked) {
			slog.Info("item already linked, returning existing", "item_id", itemID, "tenant_id", tenantID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"item_id": itemID.String(),
				"status":  "already_linked",
			})
			return
		}

		// other errors
		slog.Error("failed to connect item", "tenant_id", tenantID, "error", err)
		http.Error(w, "failed to connect item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"item_id": itemID.String(),
		"status":  "sync_queued",
	})
}
