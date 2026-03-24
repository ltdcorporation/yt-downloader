package avatar

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
)

type fakeUserStore struct {
	user auth.User

	getErr       error
	updateErr    error
	updateErrors []error

	updateCalls []struct {
		userID    string
		avatarURL string
	}
}

func (f *fakeUserStore) GetUserByID(_ context.Context, userID string) (auth.User, error) {
	if f.getErr != nil {
		return auth.User{}, f.getErr
	}
	if strings.TrimSpace(userID) == "" {
		return auth.User{}, auth.ErrUserNotFound
	}
	return f.user, nil
}

func (f *fakeUserStore) UpdateUserAvatarURL(_ context.Context, userID, avatarURL string, updatedAt time.Time) (auth.User, error) {
	if len(f.updateErrors) > 0 {
		err := f.updateErrors[0]
		f.updateErrors = f.updateErrors[1:]
		if err != nil {
			return auth.User{}, err
		}
	}
	if f.updateErr != nil {
		return auth.User{}, f.updateErr
	}
	if strings.TrimSpace(userID) == "" {
		return auth.User{}, auth.ErrUserNotFound
	}
	if updatedAt.IsZero() {
		return auth.User{}, errors.New("updatedAt must be set")
	}

	f.updateCalls = append(f.updateCalls, struct {
		userID    string
		avatarURL string
	}{
		userID:    userID,
		avatarURL: avatarURL,
	})

	f.user.AvatarURL = strings.TrimSpace(avatarURL)
	f.user.UpdatedAt = updatedAt.UTC()
	return f.user, nil
}

type fakeObjectStore struct {
	uploadErr       error
	deleteErr       error
	deleteErrByKey  map[string]error
	deleteErrByCall []error

	uploads []struct {
		key         string
		contentType string
		payload     []byte
	}
	deletes []string
}

func (f *fakeObjectStore) UploadObject(_ context.Context, key, contentType string, body []byte) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	payload := make([]byte, len(body))
	copy(payload, body)
	f.uploads = append(f.uploads, struct {
		key         string
		contentType string
		payload     []byte
	}{
		key:         key,
		contentType: contentType,
		payload:     payload,
	})
	return nil
}

func (f *fakeObjectStore) DeleteObject(_ context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	if len(f.deleteErrByCall) > 0 {
		err := f.deleteErrByCall[0]
		f.deleteErrByCall = f.deleteErrByCall[1:]
		if err != nil {
			return err
		}
	}
	if f.deleteErrByKey != nil {
		if err, ok := f.deleteErrByKey[key]; ok {
			return err
		}
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

type fakeProcessor struct {
	out []byte
	err error
}

func (f fakeProcessor) Normalize(_ context.Context, _ []byte) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	payload := make([]byte, len(f.out))
	copy(payload, f.out)
	return payload, nil
}

func newAvatarServiceForTest(t *testing.T, userStore *fakeUserStore, objectStore *fakeObjectStore, processor fakeProcessor) *Service {
	t.Helper()

	svc, err := NewService(userStore, objectStore, processor, Options{
		PublicBaseURL:  "https://avatar.indobang.site",
		KeyPrefix:      "avatars",
		MaxUploadBytes: 2 * 1024 * 1024,
		DeleteAttempts: 1,
		DeleteBackoff:  1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new avatar service: %v", err)
	}
	svc.now = func() time.Time {
		return time.Date(2026, 3, 24, 20, 0, 0, 0, time.UTC)
	}
	return svc
}

func TestService_ReplaceAvatar_FirstUpload(t *testing.T) {
	store := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

	updated, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw-image"))
	if err != nil {
		t.Fatalf("ReplaceAvatar failed: %v", err)
	}
	if len(objects.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(objects.uploads))
	}
	if len(objects.deletes) != 0 {
		t.Fatalf("expected no delete call for first avatar, got %d", len(objects.deletes))
	}
	if objects.uploads[0].contentType != "image/webp" {
		t.Fatalf("unexpected upload content type: %s", objects.uploads[0].contentType)
	}
	if updated.AvatarURL == "" || !strings.HasPrefix(updated.AvatarURL, "https://avatar.indobang.site/avatars/usr_1/") {
		t.Fatalf("unexpected avatar URL: %s", updated.AvatarURL)
	}
	if store.user.AvatarURL != updated.AvatarURL {
		t.Fatalf("store should persist new avatar url: got=%s want=%s", store.user.AvatarURL, updated.AvatarURL)
	}
}

func TestService_ReplaceAvatar_DeletesOldManagedAvatar(t *testing.T) {
	store := &fakeUserStore{user: auth.User{
		ID:        "usr_1",
		FullName:  "User",
		Email:     "user@example.com",
		AvatarURL: "https://avatar.indobang.site/avatars/usr_1/old.webp",
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

	updated, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw-image"))
	if err != nil {
		t.Fatalf("ReplaceAvatar failed: %v", err)
	}
	if len(objects.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(objects.uploads))
	}
	if len(objects.deletes) != 1 {
		t.Fatalf("expected one delete for previous avatar, got %d", len(objects.deletes))
	}
	if objects.deletes[0] != "avatars/usr_1/old.webp" {
		t.Fatalf("unexpected deleted key: %s", objects.deletes[0])
	}
	if updated.AvatarURL == "https://avatar.indobang.site/avatars/usr_1/old.webp" {
		t.Fatalf("expected new avatar URL, old still present")
	}
}

func TestService_ReplaceAvatar_DeleteOldFailsRollbackAndCleanup(t *testing.T) {
	oldURL := "https://avatar.indobang.site/avatars/usr_1/old.webp"
	store := &fakeUserStore{user: auth.User{
		ID:        "usr_1",
		FullName:  "User",
		Email:     "user@example.com",
		AvatarURL: oldURL,
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}}
	objects := &fakeObjectStore{
		deleteErrByKey: map[string]error{
			"avatars/usr_1/old.webp": errors.New("r2 delete down"),
		},
	}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

	_, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw-image"))
	if err == nil {
		t.Fatal("expected replace avatar error when old delete fails")
	}
	if !errors.Is(err, ErrDeleteFailed) {
		t.Fatalf("expected ErrDeleteFailed, got %v", err)
	}
	if store.user.AvatarURL != oldURL {
		t.Fatalf("expected avatar URL rollback to old URL, got %s", store.user.AvatarURL)
	}
	if len(store.updateCalls) != 2 {
		t.Fatalf("expected update + rollback calls, got %d", len(store.updateCalls))
	}
	if len(objects.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(objects.uploads))
	}
	if len(objects.deletes) != 2 {
		t.Fatalf("expected delete old + cleanup new, got %d (%v)", len(objects.deletes), objects.deletes)
	}
	if objects.deletes[0] != "avatars/usr_1/old.webp" {
		t.Fatalf("expected first delete old key, got %s", objects.deletes[0])
	}
	if objects.deletes[1] == objects.deletes[0] {
		t.Fatalf("expected second delete to cleanup new key, got duplicate %s", objects.deletes[1])
	}
}

func TestService_RemoveAvatar(t *testing.T) {
	t.Run("managed avatar delete success", func(t *testing.T) {
		store := &fakeUserStore{user: auth.User{
			ID:        "usr_1",
			FullName:  "User",
			Email:     "user@example.com",
			AvatarURL: "https://avatar.indobang.site/avatars/usr_1/current.webp",
			CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		}}
		objects := &fakeObjectStore{}
		svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("ignored")})

		updated, err := svc.RemoveAvatar(context.Background(), "usr_1")
		if err != nil {
			t.Fatalf("RemoveAvatar failed: %v", err)
		}
		if updated.AvatarURL != "" {
			t.Fatalf("expected cleared avatar URL, got %s", updated.AvatarURL)
		}
		if len(objects.deletes) != 1 || objects.deletes[0] != "avatars/usr_1/current.webp" {
			t.Fatalf("unexpected delete calls: %v", objects.deletes)
		}
		if len(store.updateCalls) != 1 || store.updateCalls[0].avatarURL != "" {
			t.Fatalf("expected single clear update call, got %+v", store.updateCalls)
		}
	})

	t.Run("managed avatar delete failure triggers rollback", func(t *testing.T) {
		oldURL := "https://avatar.indobang.site/avatars/usr_1/current.webp"
		store := &fakeUserStore{user: auth.User{
			ID:        "usr_1",
			FullName:  "User",
			Email:     "user@example.com",
			AvatarURL: oldURL,
			CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		}}
		objects := &fakeObjectStore{deleteErrByKey: map[string]error{"avatars/usr_1/current.webp": errors.New("delete failed")}}
		svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("ignored")})

		_, err := svc.RemoveAvatar(context.Background(), "usr_1")
		if err == nil {
			t.Fatal("expected remove avatar failure")
		}
		if !errors.Is(err, ErrDeleteFailed) {
			t.Fatalf("expected ErrDeleteFailed, got %v", err)
		}
		if store.user.AvatarURL != oldURL {
			t.Fatalf("expected rollback to old avatar URL, got %s", store.user.AvatarURL)
		}
		if len(store.updateCalls) != 2 {
			t.Fatalf("expected clear + rollback updates, got %d", len(store.updateCalls))
		}
	})
}

func TestService_ReplaceAvatar_Validation(t *testing.T) {
	store := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Now().UTC()}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", nil); !errors.Is(err, ErrPayloadEmpty) {
		t.Fatalf("expected ErrPayloadEmpty, got %v", err)
	}

	targetTooLarge := make([]byte, svc.MaxUploadBytes()+1)
	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", targetTooLarge); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestNewService_ValidationAndDefaults(t *testing.T) {
	store := &fakeUserStore{}
	objects := &fakeObjectStore{}
	processor := fakeProcessor{out: []byte("ok")}

	if _, err := NewService(nil, objects, processor, Options{PublicBaseURL: "https://avatar.indobang.site"}); !errors.Is(err, ErrServiceNotReady) {
		t.Fatalf("expected ErrServiceNotReady for nil store, got %v", err)
	}
	if _, err := NewService(store, nil, processor, Options{PublicBaseURL: "https://avatar.indobang.site"}); !errors.Is(err, ErrServiceNotReady) {
		t.Fatalf("expected ErrServiceNotReady for nil object store, got %v", err)
	}
	if _, err := NewService(store, objects, nil, Options{PublicBaseURL: "https://avatar.indobang.site"}); !errors.Is(err, ErrServiceNotReady) {
		t.Fatalf("expected ErrServiceNotReady for nil processor, got %v", err)
	}
	if _, err := NewService(store, objects, processor, Options{}); !errors.Is(err, ErrInvalidAvatarURL) {
		t.Fatalf("expected ErrInvalidAvatarURL for missing base URL, got %v", err)
	}
	if _, err := NewService(store, objects, processor, Options{PublicBaseURL: "://bad"}); !errors.Is(err, ErrInvalidAvatarURL) {
		t.Fatalf("expected ErrInvalidAvatarURL for bad URL parse, got %v", err)
	}
	if _, err := NewService(store, objects, processor, Options{PublicBaseURL: "https:///missing-host"}); !errors.Is(err, ErrInvalidAvatarURL) {
		t.Fatalf("expected ErrInvalidAvatarURL for missing host, got %v", err)
	}

	svc, err := NewService(store, objects, processor, Options{PublicBaseURL: "https://avatar.indobang.site/base/"})
	if err != nil {
		t.Fatalf("expected valid service, got err: %v", err)
	}
	if svc.keyPrefix != "avatars" {
		t.Fatalf("expected default keyPrefix avatars, got %q", svc.keyPrefix)
	}
	if svc.maxUploadBytes != DefaultMaxUploadBytes {
		t.Fatalf("expected default max upload bytes, got %d", svc.maxUploadBytes)
	}
	if svc.deleteAttempts != defaultDeleteAttempts {
		t.Fatalf("expected default delete attempts, got %d", svc.deleteAttempts)
	}
	if svc.deleteBackoff != defaultDeleteBackoff {
		t.Fatalf("expected default delete backoff, got %s", svc.deleteBackoff)
	}
}

func TestService_MaxUploadBytesFallback(t *testing.T) {
	var nilSvc *Service
	if got := nilSvc.MaxUploadBytes(); got != DefaultMaxUploadBytes {
		t.Fatalf("expected default max upload bytes for nil service, got %d", got)
	}

	svc := &Service{maxUploadBytes: 0}
	if got := svc.MaxUploadBytes(); got != DefaultMaxUploadBytes {
		t.Fatalf("expected default max upload bytes for zero value, got %d", got)
	}
}

func TestService_ReplaceAvatar_ErrorPaths(t *testing.T) {
	var nilSvc *Service
	if _, err := nilSvc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); !errors.Is(err, ErrServiceNotReady) {
		t.Fatalf("expected ErrServiceNotReady for nil service, got %v", err)
	}

	store := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Now().UTC()}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

	if _, err := svc.ReplaceAvatar(context.Background(), "", []byte("raw")); err == nil {
		t.Fatalf("expected validation error for empty user id")
	}

	failingProcessor := newAvatarServiceForTest(t, store, objects, fakeProcessor{err: ErrInvalidImage})
	if _, err := failingProcessor.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected ErrInvalidImage from processor, got %v", err)
	}

	failingStore := &fakeUserStore{getErr: auth.ErrUserNotFound}
	svc = newAvatarServiceForTest(t, failingStore, objects, fakeProcessor{out: []byte("webp-bytes")})
	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("expected auth.ErrUserNotFound from get user, got %v", err)
	}

	failingUpload := &fakeObjectStore{uploadErr: errors.New("upload down")}
	svc = newAvatarServiceForTest(t, store, failingUpload, fakeProcessor{out: []byte("webp-bytes")})
	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); err == nil || !strings.Contains(err.Error(), "upload down") {
		t.Fatalf("expected upload error propagation, got %v", err)
	}

	failingUpdate := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Now().UTC()}, updateErr: auth.ErrUserNotFound}
	svc = newAvatarServiceForTest(t, failingUpdate, &fakeObjectStore{}, fakeProcessor{out: []byte("webp-bytes")})
	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("expected update error propagation, got %v", err)
	}

	failingCleanupObjects := &fakeObjectStore{deleteErr: errors.New("cleanup failed")}
	svc = newAvatarServiceForTest(t, failingUpdate, failingCleanupObjects, fakeProcessor{out: []byte("webp-bytes")})
	if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("expected cleanup failure in error, got %v", err)
	}
}

func TestService_ReplaceAvatar_RollbackFailureAndUnmanagedOldAvatar(t *testing.T) {
	t.Run("rollback failure returns ErrRollbackFailed", func(t *testing.T) {
		store := &fakeUserStore{user: auth.User{
			ID:        "usr_1",
			FullName:  "User",
			Email:     "user@example.com",
			AvatarURL: "https://avatar.indobang.site/avatars/usr_1/old.webp",
			CreatedAt: time.Now().UTC(),
		}, updateErrors: []error{nil, errors.New("rollback write failed")}}
		objects := &fakeObjectStore{deleteErrByKey: map[string]error{"avatars/usr_1/old.webp": errors.New("delete old failed")}}
		svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

		_, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw"))
		if !errors.Is(err, ErrRollbackFailed) {
			t.Fatalf("expected ErrRollbackFailed, got %v", err)
		}
	})

	t.Run("unmanaged old avatar is ignored for delete", func(t *testing.T) {
		store := &fakeUserStore{user: auth.User{
			ID:        "usr_1",
			FullName:  "User",
			Email:     "user@example.com",
			AvatarURL: "https://external.cdn.example/avatar.webp",
			CreatedAt: time.Now().UTC(),
		}}
		objects := &fakeObjectStore{}
		svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("webp-bytes")})

		if _, err := svc.ReplaceAvatar(context.Background(), "usr_1", []byte("raw")); err != nil {
			t.Fatalf("ReplaceAvatar failed: %v", err)
		}
		if len(objects.deletes) != 0 {
			t.Fatalf("expected no delete for unmanaged old avatar, got %v", objects.deletes)
		}
	})
}

func TestService_RemoveAvatar_EdgePaths(t *testing.T) {
	var nilSvc *Service
	if _, err := nilSvc.RemoveAvatar(context.Background(), "usr_1"); !errors.Is(err, ErrServiceNotReady) {
		t.Fatalf("expected ErrServiceNotReady for nil service, got %v", err)
	}

	store := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Now().UTC()}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("ignored")})

	if _, err := svc.RemoveAvatar(context.Background(), ""); err == nil {
		t.Fatalf("expected validation error for empty user id")
	}

	store.getErr = auth.ErrUserNotFound
	if _, err := svc.RemoveAvatar(context.Background(), "usr_1"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("expected auth.ErrUserNotFound, got %v", err)
	}
	store.getErr = nil

	updated, err := svc.RemoveAvatar(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("unexpected error when avatar empty: %v", err)
	}
	if updated.AvatarURL != "" {
		t.Fatalf("expected avatar to remain empty, got %q", updated.AvatarURL)
	}

	store.user.AvatarURL = "https://cdn.example/avatar.webp"
	updated, err = svc.RemoveAvatar(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("unexpected error for unmanaged avatar remove: %v", err)
	}
	if updated.AvatarURL != "" {
		t.Fatalf("expected avatar cleared for unmanaged source, got %q", updated.AvatarURL)
	}
	if len(objects.deletes) != 0 {
		t.Fatalf("expected no object delete for unmanaged source, got %v", objects.deletes)
	}

	store.user.AvatarURL = "https://avatar.indobang.site/avatars/usr_1/current.webp"
	store.updateErr = errors.New("update failed")
	if _, err := svc.RemoveAvatar(context.Background(), "usr_1"); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}
	store.updateErr = nil

	store.user.AvatarURL = "https://avatar.indobang.site/avatars/usr_1/current.webp"
	store.updateErrors = []error{nil, errors.New("rollback failed")}
	objects.deleteErrByKey = map[string]error{"avatars/usr_1/current.webp": errors.New("delete failed")}
	if _, err := svc.RemoveAvatar(context.Background(), "usr_1"); !errors.Is(err, ErrRollbackFailed) {
		t.Fatalf("expected ErrRollbackFailed when remove rollback fails, got %v", err)
	}
}

func TestService_DeleteObjectStrictAndHelpers(t *testing.T) {
	store := &fakeUserStore{user: auth.User{ID: "usr_1", FullName: "User", Email: "user@example.com", CreatedAt: time.Now().UTC()}}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("ok")})

	if err := svc.deleteObjectStrict(context.Background(), ""); !errors.Is(err, ErrDeleteFailed) {
		t.Fatalf("expected ErrDeleteFailed for empty key, got %v", err)
	}

	svc.deleteAttempts = 2
	svc.deleteBackoff = time.Millisecond
	objects.deleteErrByCall = []error{errors.New("transient"), nil}
	if err := svc.deleteObjectStrict(context.Background(), "avatars/usr_1/retry.webp"); err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}

	svc.deleteAttempts = 2
	svc.deleteBackoff = 50 * time.Millisecond
	objects.deleteErrByCall = []error{errors.New("always fail")}
	objects.deleteErr = errors.New("always fail")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.deleteObjectStrict(ctx, "avatars/usr_1/canceled.webp"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestService_HelperFunctionsCoverage(t *testing.T) {
	svc := &Service{
		publicBaseURL: &url.URL{Scheme: "https", Host: "avatar.indobang.site", Path: "/"},
		keyPrefix:     "avatars",
	}

	if got := sanitizeKeyPart("  Usr !@# 01  "); got == "" {
		t.Fatalf("expected sanitized key part, got empty")
	}
	if got := sanitizeKeyPart("   "); got != "unknown" {
		t.Fatalf("expected unknown fallback, got %q", got)
	}

	svc.publicBaseURL = &url.URL{Scheme: "https", Host: "avatar.indobang.site", Path: "/cdn"}
	built := svc.buildObjectURL("avatars/usr_1/a.webp")
	if built != "https://avatar.indobang.site/cdn/avatars/usr_1/a.webp" {
		t.Fatalf("unexpected built URL with base path: %s", built)
	}

	if key, ok := svc.extractManagedObjectKey("https://avatar.indobang.site/cdn/avatars/usr_1/a.webp"); !ok || key != "avatars/usr_1/a.webp" {
		t.Fatalf("expected managed key extraction success, got key=%q ok=%v", key, ok)
	}
	if _, ok := svc.extractManagedObjectKey("://bad"); ok {
		t.Fatalf("expected invalid URL to be unmanaged")
	}
	if _, ok := svc.extractManagedObjectKey("https://other.example/cdn/avatars/usr_1/a.webp"); ok {
		t.Fatalf("expected host mismatch to be unmanaged")
	}
	if _, ok := svc.extractManagedObjectKey("https://avatar.indobang.site/cdn"); ok {
		t.Fatalf("expected base path only to be unmanaged")
	}
	if _, ok := svc.extractManagedObjectKey("https://avatar.indobang.site/cdn/not-prefix/usr_1/a.webp"); ok {
		t.Fatalf("expected prefix mismatch to be unmanaged")
	}
}

func TestService_GenerateObjectKey_RandomFailure(t *testing.T) {
	store := &fakeUserStore{}
	objects := &fakeObjectStore{}
	svc := newAvatarServiceForTest(t, store, objects, fakeProcessor{out: []byte("ok")})

	prev := readRandomBytes
	readRandomBytes = func(_ []byte) (int, error) {
		return 0, errors.New("random source failed")
	}
	t.Cleanup(func() {
		readRandomBytes = prev
	})

	if _, err := svc.generateObjectKey("usr_1"); err == nil || !strings.Contains(err.Error(), "random source failed") {
		t.Fatalf("expected random source failure, got %v", err)
	}
}
