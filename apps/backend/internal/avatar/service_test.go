package avatar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
)

type fakeUserStore struct {
	user auth.User

	getErr    error
	updateErr error

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
	uploadErr      error
	deleteErrByKey map[string]error

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
	if f.deleteErrByKey != nil {
		if err, ok := f.deleteErrByKey[key]; ok {
			return err
		}
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
