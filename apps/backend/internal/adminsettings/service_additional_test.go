package adminsettings

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdminSettingsService_GuardsAndNoopPatch(t *testing.T) {
	ctx := context.Background()

	var nilSvc *Service
	if _, err := nilSvc.Get(ctx); err == nil {
		t.Fatalf("expected nil service Get error")
	}
	if _, err := nilSvc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: Patch{}}); err == nil {
		t.Fatalf("expected nil service Patch error")
	}

	now := time.Date(2026, 3, 29, 7, 45, 0, 0, time.UTC)
	applyCalled := false
	store := &Store{backend: &fakeAdminSettingsStoreBackend{
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

	if _, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 0, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: ptrAdminQuality(Quality1080p)}}}); err == nil {
		t.Fatalf("expected meta.version validation error")
	}

	result, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: 1,
		Patch: Patch{
			Preferences: &PreferencesPatch{DefaultQuality: ptrAdminQuality(Quality1080p)},
		},
	})
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

func TestAdminSettingsService_Patch_DefaultMetadataAndConflictFallback(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 8, 0, 0, 0, time.UTC)

	t.Run("defaults source and forwards metadata", func(t *testing.T) {
		captured := ApplyPatchParams{}
		store := &Store{backend: &fakeAdminSettingsStoreBackend{
			getSnapshot: func(_ context.Context, ts time.Time) (Snapshot, error) {
				return DefaultSnapshot(ts), nil
			},
			applyPatch: func(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
				captured = params
				return params.After, nil
			},
		}}

		svc := NewService(store)
		svc.now = func() time.Time { return now }
		quality := Quality720p
		updated, err := svc.Patch(ctx, PatchInput{
			ExpectedVersion: 1,
			Patch:           Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}},
			ActorUserID:     "  usr_admin  ",
			RequestID:       "  req-admin-settings-service  ",
		})
		if err != nil {
			t.Fatalf("Patch failed: %v", err)
		}
		if updated.Version != 2 {
			t.Fatalf("expected version increment to 2, got %d", updated.Version)
		}
		if captured.ActorUserID != "usr_admin" {
			t.Fatalf("expected trimmed actor user id, got %q", captured.ActorUserID)
		}
		if captured.Source != "admin_web" {
			t.Fatalf("expected default source=admin_web, got %q", captured.Source)
		}
		if captured.RequestID != "req-admin-settings-service" {
			t.Fatalf("expected trimmed request id, got %q", captured.RequestID)
		}
		if captured.AuditID == "" {
			t.Fatalf("expected generated audit id")
		}
	})

	t.Run("version conflict reports latest version when reload succeeds", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeAdminSettingsStoreBackend{
			getSnapshot: func(_ context.Context, ts time.Time) (Snapshot, error) {
				getCalls++
				snapshot := DefaultSnapshot(ts)
				if getCalls > 1 {
					snapshot.Version = 11
				}
				return snapshot, nil
			},
			applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
				return Snapshot{}, ErrVersionConflict
			},
		}}
		svc := NewService(store)
		svc.now = func() time.Time { return now }

		quality := Quality720p
		_, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}}})
		if err == nil {
			t.Fatalf("expected version conflict error")
		}
		var conflictErr *VersionConflictError
		if !errors.As(err, &conflictErr) {
			t.Fatalf("expected VersionConflictError, got %T %v", err, err)
		}
		if conflictErr.CurrentVersion != 11 {
			t.Fatalf("expected latest current version=11, got %d", conflictErr.CurrentVersion)
		}
	})

	t.Run("version conflict fallback when latest reload fails", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeAdminSettingsStoreBackend{
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

		quality := Quality720p
		_, err := svc.Patch(ctx, PatchInput{ExpectedVersion: 1, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}}})
		if err == nil {
			t.Fatalf("expected version conflict error")
		}
		var conflictErr *VersionConflictError
		if !errors.As(err, &conflictErr) {
			t.Fatalf("expected VersionConflictError, got %T %v", err, err)
		}
		if conflictErr.CurrentVersion != 0 {
			t.Fatalf("expected fallback current version=0 when latest reload fails, got %d", conflictErr.CurrentVersion)
		}
	})
}

func ptrAdminQuality(v Quality) *Quality { return &v }
