package adminsettings

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
	seedAuthUser(t, backend.db, "usr_admin")

	store := &Store{backend: backend}
	ctx := context.Background()

	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady failed: %v", err)
	}
	if err := store.EnsureReady(ctx); err != nil {
		t.Fatalf("EnsureReady should be idempotent, got: %v", err)
	}

	now := time.Date(2026, 3, 29, 3, 45, 0, 0, time.UTC)
	snapshot, err := store.GetOrCreateSnapshot(ctx, now)
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
	after.Data.Preferences.DefaultQuality = Quality4K
	after.Data.Notifications.Email.Processing = false
	after.Version = snapshot.Version + 1
	after.UpdatedAt = now.Add(1 * time.Minute)

	updated, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"preferences.default_quality",
			"notifications.email.processing",
			"preferences.default_quality",
		},
		ActorUserID: " usr_admin ",
		RequestID:   " req-admin-settings ",
		Source:      "ADMIN_WEB",
		ChangedAt:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2, got %d", updated.Version)
	}
	if updated.Data.Preferences.DefaultQuality != Quality4K {
		t.Fatalf("expected quality=4k, got %s", updated.Data.Preferences.DefaultQuality)
	}
	if updated.Data.Notifications.Email.Processing {
		t.Fatalf("expected processing=false after patch")
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id=usr_admin, got %q", updated.UpdatedByUserID)
	}

	var auditCount int
	var source string
	var requestID string
	if err := backend.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), COALESCE(MAX(source), ''), COALESCE(MAX(request_id), '')
		 FROM admin_system_settings_audit`,
	).Scan(&auditCount, &source, &requestID); err != nil {
		t.Fatalf("failed to query admin settings audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 admin settings audit row, got %d", auditCount)
	}
	if source != "admin_web" {
		t.Fatalf("expected normalized source=admin_web, got %q", source)
	}
	if strings.TrimSpace(requestID) != "req-admin-settings" {
		t.Fatalf("expected normalized request_id, got %q", requestID)
	}

	_, err = store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"preferences.default_quality",
		},
		ActorUserID: "usr_admin",
		Source:      "admin_web",
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict on stale patch, got %v", err)
	}

	beforeNoAudit := updated
	afterNoAudit := updated
	afterNoAudit.Version = beforeNoAudit.Version + 1
	afterNoAudit.UpdatedAt = now.Add(2 * time.Minute)
	afterNoAudit.Data.Preferences.AutoTrimSilence = true

	updatedNoAudit, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before: beforeNoAudit,
		After:  afterNoAudit,
		Source: "admin_web",
	})
	if err != nil {
		t.Fatalf("ApplyPatch without changed_fields failed: %v", err)
	}
	if updatedNoAudit.UpdatedByUserID != "" {
		t.Fatalf("expected updated_by_user_id empty when actor omitted, got %q", updatedNoAudit.UpdatedByUserID)
	}

	if err := backend.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM admin_system_settings_audit`,
	).Scan(&auditCount); err != nil {
		t.Fatalf("failed to recount admin settings audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected audit count to stay 1 when changed_fields empty, got %d", auditCount)
	}
}
