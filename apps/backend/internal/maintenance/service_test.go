package maintenance

import (
	"context"
	"errors"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	store := NewStore(config.Config{}, nil)
	service := NewService(store)
	if service == nil {
		t.Fatalf("expected non-nil maintenance service")
	}
	return service
}

func TestService_GetReturnsDefaultSnapshot(t *testing.T) {
	svc := newTestService(t)

	snapshot, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version=1, got %d", snapshot.Version)
	}
	if snapshot.Data.EstimatedDowntime == "" || snapshot.Data.PublicMessage == "" {
		t.Fatalf("expected non-empty default maintenance fields")
	}
	if len(snapshot.Data.Services) != 3 {
		t.Fatalf("expected 3 default services, got %d", len(snapshot.Data.Services))
	}
}

func TestService_PatchAndVersionConflict(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	before, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	enabled := true
	message := "Scheduled maintenance in progress"
	status := StatusMaintenance
	serviceEnabled := false

	updated, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: before.Version,
		Patch: Patch{
			Enabled:       &enabled,
			PublicMessage: &message,
			Services: []ServicePatch{
				{Key: ServiceAPIGateway, Status: &status, Enabled: &serviceEnabled},
			},
		},
		ActorUserID: "usr_admin",
		Source:      "admin_web",
	})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if updated.Version != before.Version+1 {
		t.Fatalf("expected version increment, got %d", updated.Version)
	}
	if !updated.Data.Enabled {
		t.Fatalf("expected maintenance enabled")
	}
	if updated.Data.PublicMessage != message {
		t.Fatalf("unexpected public message: %q", updated.Data.PublicMessage)
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("unexpected updated_by_user_id: %q", updated.UpdatedByUserID)
	}

	apiGateway := updated.Data.Services[0]
	if apiGateway.Key != ServiceAPIGateway {
		t.Fatalf("unexpected first service key: %s", apiGateway.Key)
	}
	if apiGateway.Status != StatusMaintenance || apiGateway.Enabled {
		t.Fatalf("unexpected api gateway service state: %+v", apiGateway)
	}

	_, err = svc.Patch(ctx, PatchInput{
		ExpectedVersion: before.Version,
		Patch: Patch{
			Enabled: &enabled,
		},
	})
	if err == nil {
		t.Fatalf("expected stale version conflict")
	}
	var conflictErr *VersionConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected VersionConflictError, got %v", err)
	}
}

func TestService_PatchValidation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	current, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if _, err := svc.Patch(ctx, PatchInput{ExpectedVersion: current.Version, Patch: Patch{}}); err == nil {
		t.Fatalf("expected empty patch validation error")
	}

	invalidStatus := ServiceStatus("broken")
	if _, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: current.Version,
		Patch:           Patch{Services: []ServicePatch{{Key: ServiceAPIGateway, Status: &invalidStatus}}},
	}); err == nil {
		t.Fatalf("expected invalid service status validation error")
	}

	invalidKey := ServiceKey("unknown")
	enabled := true
	if _, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: current.Version,
		Patch:           Patch{Services: []ServicePatch{{Key: invalidKey, Enabled: &enabled}}},
	}); err == nil {
		t.Fatalf("expected invalid service key validation error")
	}

	longMessage := make([]rune, 1001)
	for i := range longMessage {
		longMessage[i] = 'a'
	}
	msg := string(longMessage)
	if _, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: current.Version,
		Patch:           Patch{PublicMessage: &msg},
	}); err == nil {
		t.Fatalf("expected long message validation error")
	}

	if _, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: 0,
		Patch:           Patch{Enabled: &enabled},
	}); err == nil {
		t.Fatalf("expected invalid meta.version validation error")
	}

	if _, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: current.Version,
		Patch: Patch{
			Services: []ServicePatch{{
				Key:     ServiceWorkerNodes,
				Enabled: boolPtr(false),
			}},
		},
		ActorUserID: "usr_admin",
		RequestID:   "req_1",
		Source:      "Admin_Web",
	}); err != nil {
		t.Fatalf("expected valid patch with source normalization, got %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestDefaultSnapshotAndNormalize(t *testing.T) {
	now := time.Now().UTC()
	snapshot := DefaultSnapshot(now)
	if snapshot.Version != 1 {
		t.Fatalf("expected version=1")
	}

	snapshot.Data.EstimatedDowntime = "  "
	normalized, err := normalizeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("normalizeSnapshot failed: %v", err)
	}
	if normalized.Data.EstimatedDowntime == "" {
		t.Fatalf("expected normalized estimated downtime")
	}
}
