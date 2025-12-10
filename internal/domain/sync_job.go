package domain

import (
	"github.com/google/uuid"
)

type SyncJobType string

const (
	JobTypeStandard SyncJobType = "standard"
	JobTypeReconciliation SyncJobType = "reconciliation"
)

type SyncJob struct {
	ItemID 	uuid.UUID
	JobType SyncJobType
	TraceID string
}
