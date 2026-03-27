package maintenance

import (
	"context"
	"sync"
	"time"
)

type memoryBackend struct {
	mu       sync.RWMutex
	snapshot Snapshot
	hasValue bool
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{}
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

	if !m.hasValue {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		m.snapshot = DefaultSnapshot(now)
		m.hasValue = true
	}

	return normalizeSnapshot(m.snapshot)
}

func (m *memoryBackend) ApplyPatch(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hasValue {
		base := params.Before
		if base.Version < 1 {
			base = DefaultSnapshot(time.Now().UTC())
		}
		m.snapshot = base
		m.hasValue = true
	}

	if m.snapshot.Version != params.Before.Version {
		return Snapshot{}, ErrVersionConflict
	}

	next := params.After
	next.UpdatedByUserID = params.ActorUserID
	m.snapshot = next
	m.hasValue = true

	return normalizeSnapshot(m.snapshot)
}
