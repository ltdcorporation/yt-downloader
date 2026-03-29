package settings

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeSettingsStoreBackend struct {
	closeFn     func() error
	ensureReady func(context.Context) error
	getSnapshot func(context.Context, string, time.Time) (Snapshot, error)
	applyPatch  func(context.Context, ApplyPatchParams) (Snapshot, error)
}

func (f *fakeSettingsStoreBackend) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *fakeSettingsStoreBackend) EnsureReady(ctx context.Context) error {
	if f.ensureReady != nil {
		return f.ensureReady(ctx)
	}
	return nil
}

func (f *fakeSettingsStoreBackend) GetOrCreateSnapshot(ctx context.Context, userID string, now time.Time) (Snapshot, error) {
	if f.getSnapshot != nil {
		return f.getSnapshot(ctx, userID, now)
	}
	return DefaultSnapshot(userID, now), nil
}

func (f *fakeSettingsStoreBackend) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if f.applyPatch != nil {
		return f.applyPatch(ctx, params)
	}
	return params.After, nil
}

func TestSettingsValidationError_ErrorBranches(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid settings input" {
		t.Fatalf("unexpected default validation message: %q", got)
	}
	if got := (&ValidationError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("unexpected custom validation message: %q", got)
	}
}

func TestSettingsStore_NewStoreSelectsBackend(t *testing.T) {
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

func TestSettingsStore_GuardsAndGetValidation(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil store Close should not error, got %v", err)
	}

	s := &Store{}
	ctx := context.Background()
	if err := s.EnsureReady(ctx); err == nil {
		t.Fatalf("expected EnsureReady error on uninitialized store")
	}
	if _, err := s.GetOrCreateSnapshot(ctx, "usr_1", time.Now()); err == nil {
		t.Fatalf("expected GetOrCreateSnapshot error on uninitialized store")
	}
	if _, err := s.ApplyPatch(ctx, ApplyPatchParams{}); err == nil {
		t.Fatalf("expected ApplyPatch error on uninitialized store")
	}

	s = &Store{backend: &fakeSettingsStoreBackend{}}
	if _, err := s.GetOrCreateSnapshot(ctx, "   ", time.Now()); err == nil {
		t.Fatalf("expected user_id validation error")
	}
}

func TestSettingsStore_GetOrCreateSnapshot_Normalization(t *testing.T) {
	ctx := context.Background()

	capturedUserID := ""
	capturedNow := time.Time{}
	store := &Store{backend: &fakeSettingsStoreBackend{
		getSnapshot: func(_ context.Context, userID string, ts time.Time) (Snapshot, error) {
			capturedUserID = userID
			capturedNow = ts
			snapshot := DefaultSnapshot(userID, ts)
			snapshot.UserID = "  " + snapshot.UserID + "  "
			return snapshot, nil
		},
	}}

	snapshot, err := store.GetOrCreateSnapshot(ctx, "  usr_1  ", time.Time{})
	if err != nil {
		t.Fatalf("GetOrCreateSnapshot failed: %v", err)
	}
	if capturedUserID != "usr_1" {
		t.Fatalf("expected trimmed user id sent to backend, got %q", capturedUserID)
	}
	if capturedNow.IsZero() {
		t.Fatalf("expected non-zero now when caller passes zero time")
	}
	if snapshot.UserID != "usr_1" {
		t.Fatalf("expected normalized snapshot user id, got %q", snapshot.UserID)
	}
}

func TestSettingsStore_ApplyPatch_ValidationAndNormalization(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 29, 6, 0, 0, 0, time.UTC)
	before := DefaultSnapshot("usr_1", now)
	after := before
	after.Version = before.Version + 1
	after.UpdatedAt = now.Add(time.Minute)
	after.CreatedAt = time.Time{}

	store := &Store{backend: &fakeSettingsStoreBackend{}}

	_, err := store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: Snapshot{UserID: "usr_2", Data: before.Data, Version: 2, CreatedAt: now, UpdatedAt: now}})
	if err == nil {
		t.Fatalf("expected user_id mismatch validation error")
	}

	invalidVersion := after
	invalidVersion.Version = before.Version
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: invalidVersion})
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
	store = &Store{backend: &fakeSettingsStoreBackend{
		applyPatch: func(_ context.Context, params ApplyPatchParams) (Snapshot, error) {
			captured = params
			return params.After, nil
		},
	}}

	result, err := store.ApplyPatch(ctx, ApplyPatchParams{
		Before:        before,
		After:         after,
		ChangedFields: []string{" preferences.default_quality ", "", "preferences.default_quality", "notifications.email.summary"},
		ActorUserID:   "   ",
		RequestID:     "  req-settings-1  ",
		Source:        "   ",
	})
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if captured.ActorUserID != "usr_1" {
		t.Fatalf("expected fallback actor user id to before.user_id, got %q", captured.ActorUserID)
	}
	if captured.Source != "web" {
		t.Fatalf("expected default source=web, got %q", captured.Source)
	}
	if captured.RequestID != "req-settings-1" {
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
	if result.UserID != "usr_1" || result.Version != 2 {
		t.Fatalf("unexpected normalized result: %+v", result)
	}

	expectedErr := errors.New("backend failed")
	store = &Store{backend: &fakeSettingsStoreBackend{applyPatch: func(context.Context, ApplyPatchParams) (Snapshot, error) {
		return Snapshot{}, expectedErr
	}}}
	_, err = store.ApplyPatch(ctx, ApplyPatchParams{Before: before, After: after})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected backend error propagation, got %v", err)
	}
}
