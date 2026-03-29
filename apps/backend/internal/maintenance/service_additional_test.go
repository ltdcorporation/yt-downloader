package maintenance

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMaintenanceService_GuardsAndNoopPatch(t *testing.T) {
	ctx := context.Background()

	var nilSvc *Service
	if _, err := nilSvc.Get(ctx); err == nil {
		t.Fatalf("expected nil service Get error")
	}
	if _, err := nilSvc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: Patch{}}); err == nil {
		t.Fatalf("expected nil service Patch error")
	}

	now := time.Date(2026, 3, 29, 8, 10, 0, 0, time.UTC)
	applyCalled := false
	store := &Store{backend: &fakeMaintenanceStoreBackend{
		getSnapshot: func(_ context.Context, ts time.Time) (Snapshot, error) {
			return DefaultSnapshot(ts), nil
		},
		applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
			applyCalled = true
			return Snapshot{}, nil
		},
	}}
	svc := NewService(store)
	svc.now = func() time.Time { return now }

	// patch with same value -> no effective changes -> no store.ApplyPatch call
	enabled := false
	result, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: Patch{Enabled: &enabled}})
	if err != nil {
		t.Fatalf("noop patch should return current snapshot, got err=%v", err)
	}
	if result.Version != 1 {
		t.Fatalf("expected noop patch to keep version=1, got %d", result.Version)
	}
	if applyCalled {
		t.Fatalf("expected store.ApplyPatch not called for noop patch")
	}
}

func TestMaintenanceService_VersionConflictFallbackBranches(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 8, 20, 0, 0, time.UTC)

	enabled := true
	patch := Patch{Enabled: &enabled}

	t.Run("reload latest snapshot succeeds", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeMaintenanceStoreBackend{
			getSnapshot: func(_ context.Context, ts time.Time) (Snapshot, error) {
				getCalls++
				snapshot := DefaultSnapshot(ts)
				if getCalls > 1 {
					snapshot.Version = 7
				}
				return snapshot, nil
			},
			applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
				return Snapshot{}, ErrVersionConflict
			},
		}}
		svc := NewService(store)
		svc.now = func() time.Time { return now }

		_, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: patch})
		if err == nil {
			t.Fatalf("expected version conflict error")
		}
		var conflictErr *VersionConflictError
		if !errors.As(err, &conflictErr) {
			t.Fatalf("expected VersionConflictError, got %T %v", err, err)
		}
		if conflictErr.CurrentVersion != 7 {
			t.Fatalf("expected latest current version=7, got %d", conflictErr.CurrentVersion)
		}
	})

	t.Run("reload latest snapshot fails", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeMaintenanceStoreBackend{
			getSnapshot: func(_ context.Context, ts time.Time) (Snapshot, error) {
				getCalls++
				if getCalls > 1 {
					return Snapshot{}, errors.New("latest unavailable")
				}
				return DefaultSnapshot(ts), nil
			},
			applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
				return Snapshot{}, ErrVersionConflict
			},
		}}
		svc := NewService(store)
		svc.now = func() time.Time { return now }

		_, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: patch})
		if err == nil {
			t.Fatalf("expected version conflict error")
		}
		var conflictErr *VersionConflictError
		if !errors.As(err, &conflictErr) {
			t.Fatalf("expected VersionConflictError, got %T %v", err, err)
		}
		if conflictErr.CurrentVersion != 0 {
			t.Fatalf("expected fallback current version=0, got %d", conflictErr.CurrentVersion)
		}
	})
}
