package maintenance

import (
	"context"
	"testing"
	"time"
)

func TestMemoryBackend_GetOrCreateAndApplyPatch(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	snapshot, err := backend.GetOrCreateSnapshot(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version 1, got %d", snapshot.Version)
	}

	next := snapshot
	next.Version = snapshot.Version + 1
	next.Data.Enabled = true
	next.UpdatedAt = time.Now().UTC()

	updated, err := backend.ApplyPatch(ctx, ApplyPatchParams{
		Before:      snapshot,
		After:       next,
		ActorUserID: "usr_admin",
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if !updated.Data.Enabled {
		t.Fatalf("expected updated enabled=true")
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id to be forwarded")
	}

	_, err = backend.ApplyPatch(ctx, ApplyPatchParams{Before: snapshot, After: next})
	if err == nil {
		t.Fatalf("expected version conflict when using stale version")
	}
	if err != ErrVersionConflict {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}
