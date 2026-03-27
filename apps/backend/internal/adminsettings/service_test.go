package adminsettings

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_GetAndPatch(t *testing.T) {
	store := &Store{backend: newMemoryBackend()}
	svc := NewService(store)
	now := time.Date(2026, 3, 23, 8, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	snapshot, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("expected default version=1, got %d", snapshot.Version)
	}
	if snapshot.Data.Preferences.DefaultQuality != Quality1080p {
		t.Fatalf("unexpected default quality: %s", snapshot.Data.Preferences.DefaultQuality)
	}

	quality := Quality720p
	autoTrim := true
	summary := true

	updated, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: 1,
		Patch: Patch{
			Preferences: &PreferencesPatch{
				DefaultQuality:  &quality,
				AutoTrimSilence: &autoTrim,
			},
			Notifications: &NotificationsPatch{
				Email: &EmailNotificationsPatch{Summary: &summary},
			},
		},
		ActorUserID: "usr_admin",
		RequestID:   "req-test",
		Source:      "admin_web",
	})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version=2 after patch, got %d", updated.Version)
	}
	if updated.Data.Preferences.DefaultQuality != quality {
		t.Fatalf("unexpected patched quality: %s", updated.Data.Preferences.DefaultQuality)
	}
	if !updated.Data.Preferences.AutoTrimSilence {
		t.Fatalf("expected auto_trim_silence=true after patch")
	}
	if !updated.Data.Notifications.Email.Summary {
		t.Fatalf("expected notifications.email.summary=true after patch")
	}
	if updated.UpdatedByUserID != "usr_admin" {
		t.Fatalf("expected updated_by_user_id=usr_admin, got %s", updated.UpdatedByUserID)
	}

	_, err = svc.Patch(ctx, PatchInput{
		ExpectedVersion: 1,
		Patch:           Patch{Preferences: &PreferencesPatch{DefaultQuality: &quality}},
	})
	if err == nil {
		t.Fatalf("expected version conflict")
	}
	var versionErr *VersionConflictError
	if !errors.As(err, &versionErr) {
		t.Fatalf("expected VersionConflictError, got %T %v", err, err)
	}
	if versionErr.CurrentVersion != 2 {
		t.Fatalf("unexpected current version in conflict: %d", versionErr.CurrentVersion)
	}
}

func TestService_PatchValidation(t *testing.T) {
	store := &Store{backend: newMemoryBackend()}
	svc := NewService(store)
	ctx := context.Background()

	_, err := svc.Patch(ctx, PatchInput{
		ExpectedVersion: 1,
		Patch:           Patch{},
	})
	if err == nil {
		t.Fatalf("expected validation error for empty patch")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T %v", err, err)
	}

	invalidQuality := Quality("8k")
	_, err = svc.Patch(ctx, PatchInput{
		ExpectedVersion: 1,
		Patch: Patch{Preferences: &PreferencesPatch{
			DefaultQuality: &invalidQuality,
		}},
	})
	if err == nil {
		t.Fatalf("expected validation error for invalid quality")
	}
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T %v", err, err)
	}
}
