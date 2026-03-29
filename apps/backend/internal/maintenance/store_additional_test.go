package maintenance

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeMaintenanceStoreBackend struct {
	closeFn     func() error
	ensureReady func(context.Context) error
	getSnapshot func(context.Context, time.Time) (Snapshot, error)
	applyPatch  func(context.Context, ApplyPatchParams) (Snapshot, error)
}

func (f *fakeMaintenanceStoreBackend) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeMaintenanceStoreBackend) EnsureReady(ctx context.Context) error {
	if f.ensureReady != nil {
		return f.ensureReady(ctx)
	}
	return nil
}

func (f *fakeMaintenanceStoreBackend) GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error) {
	if f.getSnapshot != nil {
		return f.getSnapshot(ctx, now)
	}
	return DefaultSnapshot(now), nil
}

func (f *fakeMaintenanceStoreBackend) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if f.applyPatch != nil {
		return f.applyPatch(ctx, params)
	}
	return params.After, nil
}

func TestMaintenanceValidationError_ErrorBranches(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid maintenance input" {
		t.Fatalf("unexpected default validation message: %q", got)
	}
	if got := (&ValidationError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("unexpected custom validation message: %q", got)
	}
}

func TestMaintenanceStore_NewStoreSelectsBackend(t *testing.T) {
	memoryStore := NewStore(config.Config{}, nil)
	if memoryStore == nil {
		t.Fatalf("expected non-nil memory store")
	}
	if _, ok := memoryStore.backend.(*memoryBackend); !ok {
		t.Fatalf("expected memory backend when POSTGRES_DSN empty")
	}

	pgStore := NewStore(config.Config{PostgresDSN: "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"}, nil)
	if pgStore == nil {
		t.Fatalf("expected non-nil postgres store")
	}
	if _, ok := pgStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend when POSTGRES_DSN is set")
	}
	_ = pgStore.Close()
}

func TestMaintenanceStore_Guards(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil store Close should not error, got %v", err)
	}

	s := &Store{}
	ctx := context.Background()
	if err := s.EnsureReady(ctx); err == nil {
		t.Fatalf("expected EnsureReady error on uninitialized store")
	}
	if _, err := s.GetOrCreateSnapshot(ctx, time.Now()); err == nil {
		t.Fatalf("expected GetOrCreateSnapshot error on uninitialized store")
	}
	if _, err := s.ApplyPatch(ctx, ApplyPatchParams{}); err == nil {
		t.Fatalf("expected ApplyPatch error on uninitialized store")
	}
}

func TestMaintenanceStore_ApplyPatch_ValidationAndNormalization(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 6, 30, 0, 0, time.UTC)
	before := DefaultSnapshot(now)
	after := before
	after.Version = before.Version + 1
	after.UpdatedAt = now.Add(time.Minute)
	after.CreatedAt = time.Time{}

	store := &Store{backend: &fakeMaintenanceStoreBackend{}}

	invalidVersion := after
	invalidVersion.Version = before.Version
	_, err := store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: invalidVersion})
	if err == nil {
		t.Fatalf("expected version increment validation error")
	}

	missingUpdated := after
	missingUpdated.UpdatedAt = time.Time{}
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: missingUpdated})
	if err == nil {
		t.Fatalf("expected missing updated_at validation error")
	}

	zeroCreatedBefore := before
	zeroCreatedBefore.CreatedAt = time.Time{}
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: zeroCreatedBefore, After: after})
	if err == nil {
		t.Fatalf("expected missing created_at validation error when both before/after created_at empty")
	}

	captured := ApplyPatchParams{}
	store = &Store{backend: &fakeMaintenanceStoreBackend{
		applyPatch: func(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
			captured = params
			return params.After, nil
		},
	}}

	result, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before:        before,
		After:         after,
		ChangedFields: []string{" services.api_gateway.status ", "", "services.api_gateway.status", "enabled"},
		ActorUserID:   "  usr_admin  ",
		RequestID:     "  req-maint-1  ",
		Source:        "   ",
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if captured.ActorUserID != "usr_admin" {
		t.Fatalf("expected trimmed actor user id, got %q", captured.ActorUserID)
	}
	if captured.Source != "web" {
		t.Fatalf("expected default source=web, got %q", captured.Source)
	}
	if captured.RequestID != "req-maint-1" {
		t.Fatalf("expected trimmed request id, got %q", captured.RequestID)
	}
	if captured.ChangedAt.IsZero() {
		t.Fatalf("expected auto-filled changed_at")
	}
	// maintenance store sorts changed fields alphabetically
	wantChanged := []string{"enabled", "services.api_gateway.status"}
	if !reflect.DeepEqual(captured.ChangedFields, wantChanged) {
		t.Fatalf("unexpected normalized changed fields got=%#v want=%#v", captured.ChangedFields, wantChanged)
	}
	if captured.After.CreatedAt.IsZero() || !captured.After.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("expected created_at propagated from before, got %s", captured.After.CreatedAt)
	}
	if result.Version != 2 {
		t.Fatalf("unexpected result version: %d", result.Version)
	}

	expectedErr := errors.New("backend failed")
	store = &Store{backend: &fakeMaintenanceStoreBackend{applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
		return Snapshot{}, expectedErr
	}}}
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: after})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected backend error propagation, got %v", err)
	}
}

func TestMaintenanceNormalizeDataAndHelpers(t *testing.T) {
	data, err := normalizeData(Data{
		EstimatedDowntime: "   ",
		PublicMessage:     "   ",
		Services: []ServiceOverride{
			{Key: " API_GATEWAY ", Name: "", Status: " ", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("normalizeData failed: %v", err)
	}
	if data.EstimatedDowntime != defaultEstimatedDowntime {
		t.Fatalf("expected default estimated downtime, got %q", data.EstimatedDowntime)
	}
	if data.PublicMessage != defaultPublicMessage {
		t.Fatalf("expected default public message, got %q", data.PublicMessage)
	}
	if len(data.Services) != 3 {
		t.Fatalf("expected default service list size 3, got %d", len(data.Services))
	}
	if data.Services[0].Name == "" {
		t.Fatalf("expected normalized service name for first service")
	}

	tooLongDowntime := make([]rune, 121)
	for i := range tooLongDowntime {
		tooLongDowntime[i] = 'a'
	}
	_, err = normalizeData(Data{EstimatedDowntime: string(tooLongDowntime), PublicMessage: "ok", Services: defaultServiceOverrides()})
	if err == nil {
		t.Fatalf("expected estimated_downtime length validation error")
	}

	_, err = normalizeServiceOverride(ServiceOverride{Key: "unknown", Status: StatusActive, Enabled: true})
	if err == nil {
		t.Fatalf("expected unsupported service key validation error")
	}

	_, err = normalizeServiceOverride(ServiceOverride{Key: ServiceAPIGateway, Status: "broken", Enabled: true})
	if err == nil {
		t.Fatalf("expected unsupported service status validation error")
	}

	if serviceName(ServiceKey("custom")) != "custom" {
		t.Fatalf("expected unknown serviceName to return raw key")
	}
	if !IsValidServiceKey(ServiceAPIGateway) || IsValidServiceKey(ServiceKey("x")) {
		t.Fatalf("unexpected service key validation behavior")
	}
	if !IsValidServiceStatus(StatusScaling) || IsValidServiceStatus(ServiceStatus("x")) {
		t.Fatalf("unexpected service status validation behavior")
	}
}
