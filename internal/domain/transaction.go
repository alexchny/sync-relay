package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "pending"
	TransactionStatusPosted  TransactionStatus = "posted"
)

type Transaction struct {
	ID                 uuid.UUID
	ItemID             uuid.UUID
	PlaidTransactionID string
	PlaidPendingID     *string

	AmountCents  int64
	CurrencyCode string
	Date         time.Time
	MerchantName string
	Status       TransactionStatus

	IsRemoved  bool
	RawPayload []byte

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Transaction) IsPosted() bool {
	return t.Status == TransactionStatusPosted
}

func (t *Transaction) IsPending() bool {
	return t.Status == TransactionStatusPending
}

func (t *Transaction) MarkRemoved() {
	t.IsRemoved = true
	t.UpdatedAt = time.Now()
}

func (t *Transaction) UpdateTransaction(incoming Transaction) {
	t.AmountCents = incoming.AmountCents
	t.CurrencyCode = incoming.CurrencyCode
	t.Date = incoming.Date
	t.MerchantName = incoming.MerchantName
	t.Status = incoming.Status
	t.PlaidPendingID = incoming.PlaidPendingID

	// update debug payload
	t.RawPayload = incoming.RawPayload

	t.IsRemoved = false
	t.UpdatedAt = time.Now()
}
