package domain

import (
	"time"

	"github.com/google/uuid"
)

type SyncStatus string

const (
	SyncStatusActive    SyncStatus = "active"
	SyncStatusError     SyncStatus = "error"
	SyncStatusReSyncing SyncStatus = "resyncing"
)

type Item struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	PlaidItemID    string
	AccessTokenEnc string

	NextCursor   string
	SyncStatus   SyncStatus
	LastSyncedAt *time.Time
	ErrorMessage string

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (i *Item) CanSync() bool {
	return i.SyncStatus == SyncStatusActive || i.SyncStatus == SyncStatusReSyncing
}

func (i *Item) HasError() bool {
	return i.SyncStatus == SyncStatusError
}

func (i *Item) MarkResyncing() {
	i.SyncStatus = SyncStatusReSyncing
	i.ErrorMessage = ""
	i.UpdatedAt = time.Now()
}

func (i *Item) MarkActive() {
	i.SyncStatus = SyncStatusActive
	i.ErrorMessage = ""
	i.UpdatedAt = time.Now()
}

func (i *Item) MarkError(err error) {
	i.SyncStatus = SyncStatusError
	if err != nil {
		i.ErrorMessage = err.Error()
	}
	i.UpdatedAt = time.Now()
}

func (i *Item) UpdateSuccess(cursor string) {
	i.SyncStatus = SyncStatusActive
	i.NextCursor = cursor
	i.ErrorMessage = ""

	now := time.Now()
	i.LastSyncedAt = &now
	i.UpdatedAt = now
}
