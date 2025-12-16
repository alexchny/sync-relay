package handlers

import (
	"log/slog"
	"net/http"

	"github.com/alexchny/sync-relay/internal/domain"
	"github.com/alexchny/sync-relay/internal/ports"
	"github.com/google/uuid"
)

type WebhookHandler struct {
	verifier ports.WebhookVerifier
	itemRepo ports.ItemRepository
	queue    ports.JobQueue
}

func NewWebhookHandler(v ports.WebhookVerifier, r ports.ItemRepository, q ports.JobQueue) *WebhookHandler {
	return &WebhookHandler{
		verifier: v,
		itemRepo: r,
		queue:    q,
	}
}

func (h *WebhookHandler) HandlePlaidWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := h.verifier.VerifyWebhook(r.Context(), r)
	if err != nil {
		slog.Warn("invalid webhook attempt", "error", err, "ip", r.RemoteAddr)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if payload.WebhookType != "TRANSACTIONS" || payload.WebhookCode != "SYNC_UPDATES_AVAILABLE" {
		slog.Debug("ignoring webhook", "type", payload.WebhookType, "code", payload.WebhookCode)
		w.WriteHeader(http.StatusOK)
		return
	}

	item, err := h.itemRepo.GetByPlaidItemID(r.Context(), payload.ItemID)
	if err != nil {
		slog.Error("unknown item in webhook", "plaid_item_id", payload.ItemID, "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	job := &domain.SyncJob{
		ItemID:  item.ID,
		JobType: domain.JobTypeStandard,
		TraceID: uuid.NewString(),
	}

	if err := h.queue.Enqueue(r.Context(), job); err != nil {
		slog.Error("failed to enqueue sync job", "item_id", item.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("sync job enqueued", "item_id", item.ID, "webhook_type", payload.WebhookType)
	w.WriteHeader(http.StatusAccepted)
}
