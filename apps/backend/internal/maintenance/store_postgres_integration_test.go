package maintenance

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

	now := time.Date(2026, 3, 29, 4, 0, 0, 0, time.UTC)
	snapshot, err := store.GetOrCreateSnapshot(ctx, now)
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version=1, got %d", snapshot.Version)
	}
	if len(snapshot.Data.Services) != 3 {
		t.Fatalf("expected 3 default services, got %d", len(snapshot.Data.Services))
	}

	after := snapshot
	after.Data.Enabled = true
	after.Data.PublicMessage = "Scheduled maintenance window in progress"
	after.Data.EstimatedDowntime = "30 minutes"
	after.Version = snapshot.Version + 1
	after.UpdatedAt = now.Add(1 * time.Minute)

	for i := range after.Data.Services {
		if after.Data.Services[i].Key == ServiceAPIGateway {
			after.Data.Services[i].Status = StatusMaintenance
			after.Data.Services[i].Enabled = false
		}
	}

	updated, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"enabled",
			"services.api_gateway.status",
			"services.api_gateway.enabled",
			"enabled",
		},
		ActorUserID: " usr_admin ",
		RequestID:   " req-maint-1 ",
		Source:      "ADMIN_WEB",
		ChangedAt:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2, got %d", updated.Version)
	}
	if !updated.Data.Enabled {
		t.Fatalf("expected maintenance enabled=true after patch")
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id=usr_admin, got %q", updated.UpdatedByUserID)
	}

	var apiGateway ServiceOverride
	for _, service := range updated.Data.Services {
		if service.Key == ServiceAPIGateway {
			apiGateway = service
			break
		}
	}
	if apiGateway.Key == "" {
		t.Fatalf("expected api_gateway service in updated snapshot")
	}
	if apiGateway.Status != StatusMaintenance || apiGateway.Enabled {
		t.Fatalf("unexpected api_gateway state: %+v", apiGateway)
	}

	reloaded, err := store.GetOrCreateSnapshot(ctx, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("reloading snapshot failed: %v", err)
	}
	if !reloaded.Data.Enabled {
		t.Fatalf("expected reloaded snapshot to persist enabled=true")
	}
	if len(reloaded.Data.Services) != 3 {
		t.Fatalf("expected 3 services after reload, got %d", len(reloaded.Data.Services))
	}

	var auditCount int
	var source string
	var requestID string
	if err := backend.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*), COALESCE(MAX(source), ''), COALESCE(MAX(request_id), '') FROM maintenance_audit`,
	).Scan(&auditCount, &source, &requestID); err != nil {
		t.Fatalf("failed to query maintenance audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 maintenance audit row, got %d", auditCount)
	}
	if source != "admin_web" {
		t.Fatalf("expected normalized source=admin_web, got %q", source)
	}
	if strings.TrimSpace(requestID) != "req-maint-1" {
		t.Fatalf("expected normalized request_id, got %q", requestID)
	}

	_, err = store.ApplyPatch(ctx, ApplyPatchParams{
		Before: snapshot,
		After:  after,
		ChangedFields: []string{
			"enabled",
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
	afterNoAudit.UpdatedAt = now.Add(3 * time.Minute)
	afterNoAudit.Data.PublicMessage = "Maintenance almost done"

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

	if err := backend.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_audit`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to recount maintenance audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected audit count to stay 1 when changed_fields empty, got %d", auditCount)
	}
}
