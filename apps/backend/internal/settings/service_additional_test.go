package settings

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSettingsService_GuardsAndNoopPatch(t *testing.T) {
	ctx := context.Background()

	var nilSvc *Service
	if _, err := nilSvc.Get(ctx, "usr_1"); err == nil {
		t.Fatalf("expected nil service Get error")
	}
	if _, err := nilSvc.Patch(ctx, PatchInput{UserID: "usr_1", ExpectedVersion: 1, Patch: Patch{}}); err == nil {
		t.Fatalf("expected nil service Patch error")
	}

	now := time.Date(2026, 3, 29, 7, 15, 0, 0, time.UTC)
	applyCalled := false
	store := &Store{backend: &fakeSettingsStoreBackend{
		getSnapshot: func(_ context.Context, userID string, ts time.Time) (Snapshot, error) {
			return DefaultSnapshot(userID, ts), nil
		},
		applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
			applyCalled = true
			return Snapshot{}, nil
		},
	}}
	svc := NewService(store)
	svc.now = func() time.Time { return now }

	if _, err := svc.Patch(ctx, PatchInput{UserID: "   ", ExpectedVersion: 1, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: ptrQuality(Quality1080p)}}}); err == nil {
		t.Fatalf("expected user_id validation error")
	}
	if _, err := svc.Patch(ctx, PatchInput{UserID: "usr_1", ExpectedVersion: 0, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: ptrQuality(Quality1080p)}}}); err == nil {
		t.Fatalf("expected meta.version validation error")
	}

	result, err := svc.Patch(ctx, PatchInput{
		UserID:          "usr_1",
		ExpectedVersion: 1,
		Patch: Patch{
			Preferences: &PreferencesPatch{DefaultQuality: ptrQuality(Quality1080p)},
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

func TestSettingsService_Patch_DefaultMetadataAndConflictFallback(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 7, 30, 0, 0, time.UTC)

	t.Run("defaults actor/source and forwards metadata", func(t *testing.T) {
		captured := ApplyPatchParams{}
		store := &Store{backend: &fakeSettingsStoreBackend{
			getSnapshot: func(_ context.Context, userID string, ts time.Time) (Snapshot, error) {
				return DefaultSnapshot(userID, ts), nil
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
			UserID:          " usr_1 ",
			ExpectedVersion: 1,
			Patch:           Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}},
			RequestID:       " req-settings-service ",
		})
		if err != nil {
			t.Fatalf("Patch failed: %v", err)
		}
		if updated.Version != 2 {
			t.Fatalf("expected version increment to 2, got %d", updated.Version)
		}
		if captured.ActorUserID != "usr_1" {
			t.Fatalf("expected default actor user id=user_id, got %q", captured.ActorUserID)
		}
		if captured.Source != "web" {
			t.Fatalf("expected default source=web, got %q", captured.Source)
		}
		if captured.RequestID != "req-settings-service" {
			t.Fatalf("expected trimmed request id, got %q", captured.RequestID)
		}
		if captured.AuditID == "" {
			t.Fatalf("expected generated audit id")
		}
	})

	t.Run("version conflict reports latest version when reload succeeds", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeSettingsStoreBackend{
			getSnapshot: func(_ context.Context, userID string, ts time.Time) (Snapshot, error) {
				getCalls++
				snapshot := DefaultSnapshot(userID, ts)
				if getCalls > 1 {
					snapshot.Version = 9
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
		_, err := svc.Patch(ctx, PatchInput{UserID: "usr_1", ExpectedVersion: 1, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}}})
		if err == nil {
			t.Fatalf("expected version conflict error")
		}
		var conflictErr *VersionConflictError
		if !errors.As(err, &conflictErr) {
			t.Fatalf("expected VersionConflictError, got %T %v", err, err)
		}
		if conflictErr.CurrentVersion != 9 {
			t.Fatalf("expected latest current version=9, got %d", conflictErr.CurrentVersion)
		}
	})

	t.Run("version conflict fallback when latest reload fails", func(t *testing.T) {
		getCalls := 0
		store := &Store{backend: &fakeSettingsStoreBackend{
			getSnapshot: func(_ context.Context, userID string, ts time.Time) (Snapshot, error) {
				getCalls++
				if getCalls > 1 {
					return Snapshot{}, errors.New("latest unavailable")
				}
				return DefaultSnapshot(userID, ts), nil
			},
			applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
				return Snapshot{}, ErrVersionConflict
			},
		}}
		svc := NewService(store)
		svc.now = func() time.Time { return now }

		quality := Quality720p
		_, err := svc.Patch(ctx, PatchInput{UserID: "usr_1", ExpectedVersion: 1, Patch: Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}}})
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

func ptrQuality(v Quality) *Quality { return &v }
