package adminsettings

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type settingsAuditRecord struct {
	AuditID       string
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

	snapshot Snapshot
	hasValue bool
	audits   []settingsAuditRecord
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		audits: make([]settingsAuditRecord, 0, 32),
	}
}

func (m *memoryBackend) Close() error {
	return nil
}

func (m *memoryBackend) EnsureReady(_ context.Context) error {
	return nil
}

func (m *memoryBackend) GetOrCreateSnapshot(_ context.Context, now time.Time) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasValue {
		return m.snapshot, nil
	}

	snapshot := DefaultSnapshot(now)
	m.snapshot = snapshot
	m.hasValue = true
	return snapshot, nil
}

func (m *memoryBackend) ApplyPatch(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hasValue {
		m.snapshot = DefaultSnapshot(params.ChangedAt)
		m.hasValue = true
	}

	if m.snapshot.Version != params.Before.Version {
		return Snapshot{}, ErrVersionConflict
	}

	updated := params.After
	updated.UpdatedByUserID = params.ActorUserID
	m.snapshot = updated

	if len(params.ChangedFields) > 0 {
		auditID := params.AuditID
		if auditID == "" {
			auditID = "asa_" + uuid.NewString()
		}
		if params.ChangedAt.IsZero() {
			params.ChangedAt = time.Now().UTC()
		}

		m.audits = append(m.audits, settingsAuditRecord{
			AuditID:       auditID,
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
