package adminsettings

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBackend_GetOrCreateAndApplyPatch(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Date(2026, 3, 23, 8, 0, 0, 0, time.UTC)

	snapshot, err := backend.GetOrCreateSnapshot(ctx, now)
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version=1, got %d", snapshot.Version)
	}

	next := snapshot
	next.Data.Preferences.DefaultQuality = Quality720p
	next.Version = snapshot.Version + 1
	next.UpdatedAt = now.Add(time.Minute)

	updated, err := backend.ApplyPatch(ctx, ApplyPatchParams{
		Before:        snapshot,
		After:         next,
		ChangedFields: []string{"preferences.default_quality"},
		ActorUserID:   "usr_admin",
		Source:        "admin_web",
		ChangedAt:     now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2 after apply patch, got %d", updated.Version)
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id=usr_admin, got %s", updated.UpdatedByUserID)
	}

	_, err = backend.ApplyPatch(ctx, ApplyPatchParams{
		Before:        snapshot,
		After:         next,
		ChangedFields: []string{"preferences.default_quality"},
		ActorUserID:   "usr_admin",
		Source:        "admin_web",
		ChangedAt:     now.Add(2 * time.Minute),
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	if len(backend.audits) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(backend.audits))
	}
	if backend.audits[0].AuditID == "" {
		t.Fatalf("expected non-empty audit id")
	}
}
