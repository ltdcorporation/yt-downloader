package history

import (
	"context"
	"errors"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeBackend struct {
	readyErr error

	upsertItemFn      func(ctx context.Context, item Item) (Item, error)
	getItemByIDFn     func(ctx context.Context, userID, itemID string) (Item, error)
	softDeleteItemFn  func(ctx context.Context, userID, itemID string, deletedAt time.Time) error
	markItemSuccessFn func(ctx context.Context, userID, itemID string, succeededAt time.Time) error
	createAttemptFn   func(ctx context.Context, attempt Attempt) error
	updateAttemptFn   func(ctx context.Context, attempt Attempt) error
	getAttemptByIDFn  func(ctx context.Context, userID, attemptID string) (Attempt, error)
	getByJobIDFn      func(ctx context.Context, jobID string) (Attempt, error)

	closeCalls int

	lastUpsertItem Item
	lastAttempt    Attempt
}

func (f *fakeBackend) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeBackend) EnsureReady(_ context.Context) error {
	return f.readyErr
}

func (f *fakeBackend) UpsertItem(ctx context.Context, item Item) (Item, error) {
	if f.upsertItemFn != nil {
		return f.upsertItemFn(ctx, item)
	}
	f.lastUpsertItem = item
	return item, nil
}

func (f *fakeBackend) GetItemByID(ctx context.Context, userID, itemID string) (Item, error) {
	if f.getItemByIDFn != nil {
		return f.getItemByIDFn(ctx, userID, itemID)
	}
	return Item{}, ErrItemNotFound
}

func (f *fakeBackend) SoftDeleteItem(ctx context.Context, userID, itemID string, deletedAt time.Time) error {
	if f.softDeleteItemFn != nil {
		return f.softDeleteItemFn(ctx, userID, itemID, deletedAt)
	}
	return nil
}

func (f *fakeBackend) MarkItemSuccess(ctx context.Context, userID, itemID string, succeededAt time.Time) error {
	if f.markItemSuccessFn != nil {
		return f.markItemSuccessFn(ctx, userID, itemID, succeededAt)
	}
	return nil
}

func (f *fakeBackend) CreateAttempt(ctx context.Context, attempt Attempt) error {
	if f.createAttemptFn != nil {
		return f.createAttemptFn(ctx, attempt)
	}
	f.lastAttempt = attempt
	return nil
}

func (f *fakeBackend) UpdateAttempt(ctx context.Context, attempt Attempt) error {
	if f.updateAttemptFn != nil {
		return f.updateAttemptFn(ctx, attempt)
	}
	f.lastAttempt = attempt
	return nil
}

func (f *fakeBackend) GetAttemptByID(ctx context.Context, userID, attemptID string) (Attempt, error) {
	if f.getAttemptByIDFn != nil {
		return f.getAttemptByIDFn(ctx, userID, attemptID)
	}
	return Attempt{}, ErrAttemptNotFound
}

func (f *fakeBackend) GetAttemptByJobID(ctx context.Context, jobID string) (Attempt, error) {
	if f.getByJobIDFn != nil {
		return f.getByJobIDFn(ctx, jobID)
	}
	return Attempt{}, ErrAttemptNotFound
}

func TestStoreUpsertItem_NormalizesAndHashesURL(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	item, err := store.UpsertItem(context.Background(), Item{
		ID:        "his_1",
		UserID:    "user_1",
		Platform:  PlatformYouTube,
		SourceURL: "HTTPS://WWW.YOUTUBE.COM/watch?v=abc&utm=1",
		Title:     "  Example Title  ",
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}
	if item.SourceURLHash == "" {
		t.Fatalf("expected source_url_hash to be computed")
	}
	if backend.lastUpsertItem.SourceURLHash == "" {
		t.Fatalf("backend did not receive source_url_hash")
	}
	if backend.lastUpsertItem.Title != "Example Title" {
		t.Fatalf("expected trimmed title, got %q", backend.lastUpsertItem.Title)
	}
}

func TestStoreUpsertItem_ValidatesInputs(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	_, err := store.UpsertItem(context.Background(), Item{})
	if err == nil {
		t.Fatal("expected validation error for empty item")
	}

	_, err = store.UpsertItem(context.Background(), Item{
		ID:        "his_1",
		UserID:    "user_1",
		Platform:  Platform("unknown"),
		SourceURL: "https://example.com",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid platform")
	}
}

func TestStoreMarkItemSuccess(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	var gotUserID, gotItemID string
	var gotAt time.Time
	backend.markItemSuccessFn = func(_ context.Context, userID, itemID string, succeededAt time.Time) error {
		gotUserID = userID
		gotItemID = itemID
		gotAt = succeededAt
		return nil
	}

	if err := store.MarkItemSuccess(context.Background(), " user_1 ", " his_1 ", time.Time{}); err != nil {
		t.Fatalf("unexpected mark success error: %v", err)
	}
	if gotUserID != "user_1" || gotItemID != "his_1" {
		t.Fatalf("expected trimmed identifiers, got user=%q item=%q", gotUserID, gotItemID)
	}
	if gotAt.IsZero() {
		t.Fatalf("expected succeededAt to be set")
	}
}

func TestStoreCreateAttemptAndUpdate(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	created, err := store.CreateAttempt(context.Background(), Attempt{
		ID:            "hat_1",
		HistoryItemID: "his_1",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		JobID:         "job_1",
	})
	if err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set")
	}

	backend.getAttemptByIDFn = func(_ context.Context, userID, attemptID string) (Attempt, error) {
		if userID != "user_1" || attemptID != "hat_1" {
			t.Fatalf("unexpected lookup user=%s attempt=%s", userID, attemptID)
		}
		return created, nil
	}

	updated, err := store.UpdateAttempt(context.Background(), "user_1", "hat_1", func(a *Attempt) {
		a.Status = StatusDone
		a.DownloadURL = " https://example.com/file.mp3 "
	})
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if updated.Status != StatusDone {
		t.Fatalf("expected status done, got %s", updated.Status)
	}
	if backend.lastAttempt.DownloadURL != "https://example.com/file.mp3" {
		t.Fatalf("expected trimmed download URL in backend, got %q", backend.lastAttempt.DownloadURL)
	}
}

func TestStorePassThroughErrors(t *testing.T) {
	expectedErr := errors.New("boom")
	backend := &fakeBackend{
		upsertItemFn: func(_ context.Context, item Item) (Item, error) {
			return Item{}, expectedErr
		},
		createAttemptFn: func(_ context.Context, attempt Attempt) error {
			return expectedErr
		},
		getAttemptByIDFn: func(_ context.Context, userID, attemptID string) (Attempt, error) {
			return Attempt{}, expectedErr
		},
	}
	store := &Store{backend: backend}

	if _, err := store.UpsertItem(context.Background(), Item{
		ID:        "his_1",
		UserID:    "user_1",
		Platform:  PlatformYouTube,
		SourceURL: "https://youtube.com/watch?v=abc",
	}); !errors.Is(err, expectedErr) {
		t.Fatalf("expected upsert error passthrough, got %v", err)
	}

	if _, err := store.CreateAttempt(context.Background(), Attempt{
		ID:            "hat_1",
		HistoryItemID: "his_1",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
	}); !errors.Is(err, expectedErr) {
		t.Fatalf("expected create attempt error passthrough, got %v", err)
	}

	if _, err := store.UpdateAttempt(context.Background(), "user_1", "hat_1", nil); !errors.Is(err, expectedErr) {
		t.Fatalf("expected update attempt error passthrough, got %v", err)
	}
}

func TestStoreEnsureReadyAndClose(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	if err := store.EnsureReady(context.Background()); err != nil {
		t.Fatalf("unexpected ensure ready error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if backend.closeCalls != 1 {
		t.Fatalf("expected close called once, got %d", backend.closeCalls)
	}
}

func TestNewStore_SelectsBackendByConfig(t *testing.T) {
	memoryStore := NewStore(config.Config{}, nil)
	if _, ok := memoryStore.backend.(*memoryBackend); !ok {
		t.Fatalf("expected memory backend when PostgresDSN is empty")
	}
	_ = memoryStore.Close()

	postgresStore := NewStore(config.Config{PostgresDSN: "postgres://u:p@127.0.0.1:5435/ytd?sslmode=disable"}, nil)
	if _, ok := postgresStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend when PostgresDSN is set")
	}
	_ = postgresStore.Close()
}
