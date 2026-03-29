package adminsettings

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeAdminSettingsStoreBackend struct {
	closeFn     func() error
	ensureReady func(context.Context) error
	getSnapshot func(context.Context, time.Time) (Snapshot, error)
	applyPatch  func(context.Context, ApplyPatchParams) (Snapshot, error)
}

func (f *fakeAdminSettingsStoreBackend) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeAdminSettingsStoreBackend) EnsureReady(ctx context.Context) error {
	if f.ensureReady != nil {
		return f.ensureReady(ctx)
	}
	return nil
}

func (f *fakeAdminSettingsStoreBackend) GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error) {
	if f.getSnapshot != nil {
		return f.getSnapshot(ctx, now)
	}
	return DefaultSnapshot(now), nil
}

func (f *fakeAdminSettingsStoreBackend) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if f.applyPatch != nil {
		return f.applyPatch(ctx, params)
	}
	return params.After, nil
}

func TestAdminSettingsValidationError_ErrorBranches(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid admin settings input" {
		t.Fatalf("unexpected default validation message: %q", got)
	}
	if got := (&ValidationError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("unexpected custom validation message: %q", got)
	}
}

func TestAdminSettingsStore_NewStoreSelectsBackend(t *testing.T) {
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

func TestAdminSettingsStore_Guards(t *testing.T) {
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

func TestAdminSettingsStore_GetOrCreateSnapshot_Normalization(t *testing.T) {
	ctx := context.Background()
	capturedNow := time.Time{}
	store := &Store{backend: &fakeAdminSettingsStoreBackend{
		getSnapshot: func(_ context.Context, now time.Time) (Snapshot, error) {
			capturedNow = now
			snapshot := DefaultSnapshot(now)
			snapshot.UpdatedByUserID = "  usr_admin  "
			return snapshot, nil
		},
	}}

	snapshot, err := store.GetOrCreateSnapshot(ctx, time.Time{})
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if capturedNow.IsZero() {
		t.Fatalf("expected non-zero now when caller passes zero time")
	}
	if snapshot.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id trimmed, got %q", snapshot.UpdatedByUserID)
	}
}

func TestAdminSettingsStore_ApplyPatch_ValidationAndNormalization(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 6, 15, 0, 0, time.UTC)
	before := DefaultSnapshot(now)
	after := before
	after.Version = before.Version + 1
	after.UpdatedAt = now.Add(time.Minute)
	after.CreatedAt = time.Time{}

	store := &Store{backend: &fakeAdminSettingsStoreBackend{}}

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
	store = &Store{backend: &fakeAdminSettingsStoreBackend{
		applyPatch: func(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
			captured = params
			return params.After, nil
		},
	}}

	result, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before:        before,
		After:         after,
		ChangedFields: []string{" preferences.default_quality ", "", "preferences.default_quality", "notifications.email.summary"},
		ActorUserID:   "  usr_admin  ",
		RequestID:     "  req-admin-settings  ",
		Source:        "   ",
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if captured.ActorUserID != "usr_admin" {
		t.Fatalf("expected trimmed actor user id, got %q", captured.ActorUserID)
	}
	if captured.Source != "admin_web" {
		t.Fatalf("expected default source=admin_web, got %q", captured.Source)
	}
	if captured.RequestID != "req-admin-settings" {
		t.Fatalf("expected trimmed request id, got %q", captured.RequestID)
	}
	if captured.ChangedAt.IsZero() {
		t.Fatalf("expected auto-filled changed_at")
	}
	wantChanged := []string{"preferences.default_quality", "notifications.email.summary"}
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
	store = &Store{backend: &fakeAdminSettingsStoreBackend{applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
		return Snapshot{}, expectedErr
	}}}
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: after})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected backend error propagation, got %v", err)
	}
}
