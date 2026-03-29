package settings

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStorePostgresIntegration_GetPatchAuditAndConflict(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	t.Cleanup(func() { _ = backend.Close() })

	ensureAuthUsersTable(t, backend.db)
	seedAuthUser(t, backend.db, "usr_settings_1")
	seedAuthUser(t, backend.db, "usr_admin")

	store := &Store{backend: backend}
	ctx := context.Background()

	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady failed: %v", err)
	}
	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady should be idempotent, got: %v", err)
	}

	now := time.Date(2026, 3, 29, 3, 30, 0, 0, time.UTC)
	snapshot, err := store.GetOrCreateSnapshot(ctx, "usr_settings_1", now)
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version=1, got %d", snapshot.Version)
	}
	if snapshot.Data.Preferences.DefaultQuality != Quality1080p {
		t.Fatalf("unexpected default quality: %s", snapshot.Data.Preferences.DefaultQuality)
	}

	after := snapshot
	after.Data.Preferences.DefaultQuality = Quality720p
	after.Data.Notifications.Email.Summary = true
	after.Version = snapshot.Version + 1
	after.UpdatedAt = now.Add(1 * time.Minute)

	updated, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"preferences.default_quality",
			"notifications.email.summary",
			"preferences.default_quality",
			"   ",
		},
		ActorUserID: "  usr_admin  ",
		RequestID:   "  req-settings-1  ",
		Source:      "WEB",
		ChangedAt:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2, got %d", updated.Version)
	}
	if updated.Data.Preferences.DefaultQuality != Quality720p {
		t.Fatalf("expected updated quality=720p, got %s", updated.Data.Preferences.DefaultQuality)
	}
	if !updated.Data.Notifications.Email.Summary {
		t.Fatalf("expected summary=true after patch")
	}

	var auditCount int
	var source string
	var requestID string
	if err := backend.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), COALESCE(MAX(source), ''), COALESCE(MAX(request_id), '')
		 FROM user_settings_audit
		 WHERE user_id = $1`,
		"usr_settings_1",
	).Scan(&auditCount, &source, &requestID); err != nil {
		t.Fatalf("failed to query settings audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 settings audit row, got %d", auditCount)
	}
	if source != "web" {
		t.Fatalf("expected normalized source=web, got %q", source)
	}
	if strings.TrimSpace(requestID) != "req-settings-1" {
		t.Fatalf("expected normalized request_id, got %q", requestID)
	}

	_, err = store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"preferences.default_quality",
		},
		ActorUserID: "usr_admin",
		Source:      "web",
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict on stale patch, got %v", err)
	}

	beforeNoAudit := updated
	afterNoAudit := updated
	afterNoAudit.Version = beforeNoAudit.Version + 1
	afterNoAudit.UpdatedAt = now.Add(2 * time.Minute)
	afterNoAudit.Data.Preferences.AutoTrimSilence = true

	_, err = store.ApplyPatch(ctx, ApplyPatchParams{
		Before:      beforeNoAudit,
		After:       afterNoAudit,
		ActorUserID: "usr_admin",
		Source:      "web",
	})
	if err != nil {
		t.Fatalf("ApplyPatch without changed_fields failed: %v", err)
	}

	if err := backend.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM user_settings_audit WHERE user_id = $1`,
		"usr_settings_1",
	).Scan(&auditCount); err != nil {
		t.Fatalf("failed to recount settings audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected audit count to stay 1 when changed_fields empty, got %d", auditCount)
	}
}
