package settings

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type settingsAuditRecord struct {
	AuditID       string
	UserID        string
	ActorUserID   string
	RequestID     string
	Source        string
	Before        Snapshot
	After         Snapshot
	ChangedFields []string
	CreatedAt     time.Time
}

type memoryBackend struct {
	mu sync.RWMutex

	snapshots map[string]Snapshot
	audits    []settingsAuditRecord
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		snapshots: make(map[string]Snapshot),
		audits:    make([]settingsAuditRecord, 0, 32),
	}
}

func (m *memoryBackend) Close() error {
	return nil
}

func (m *memoryBackend) EnsureReady(_ context.Context) error {
	return nil
}

func (m *memoryBackend) GetOrCreateSnapshot(_ context.Context, userID string, now time.Time) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if snapshot, ok := m.snapshots[userID]; ok {
		return snapshot, nil
	}

	snapshot := DefaultSnapshot(userID, now)
	m.snapshots[userID] = snapshot
	return snapshot, nil
}

func (m *memoryBackend) ApplyPatch(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, exists := m.snapshots[params.Before.UserID]
	if !exists {
		current = DefaultSnapshot(params.Before.UserID, params.ChangedAt)
		m.snapshots[params.Before.UserID] = current
	}

	if current.Version != params.Before.Version {
		return Snapshot{}, ErrVersionConflict
	}

	updated := params.After
	m.snapshots[updated.UserID] = updated

	if len(params.ChangedFields) > 0 {
		auditID := params.AuditID
		if auditID == "" {
			auditID = "usa_" + uuid.NewString()
		}
		if params.ChangedAt.IsZero() {
			params.ChangedAt = time.Now().UTC()
		}

		m.audits = append(m.audits, settingsAuditRecord{
			AuditID:       auditID,
			UserID:        updated.UserID,
			ActorUserID:   params.ActorUserID,
			RequestID:     params.RequestID,
			Source:        params.Source,
			Before:        params.Before,
			After:         updated,
			ChangedFields: append([]string(nil), params.ChangedFields...),
			CreatedAt:     params.ChangedAt.UTC(),
		})
	}

	return updated, nil
}
