package history

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

func TestStoreValidationAndGuardClauses(t *testing.T) {
	var zeroStore Store

	if err := zeroStore.EnsureReady(context.Background()); err == nil {
		t.Fatalf("expected ensure ready error on zero store")
	}
	if _, err := zeroStore.UpsertItem(context.Background(), Item{}); err == nil {
		t.Fatalf("expected upsert error on zero store")
	}
	if _, err := zeroStore.GetItemByID(context.Background(), "u", "i"); err == nil {
		t.Fatalf("expected get item error on zero store")
	}
	if err := zeroStore.SoftDeleteItem(context.Background(), "u", "i", time.Time{}); err == nil {
		t.Fatalf("expected soft delete error on zero store")
	}
	if err := zeroStore.MarkItemSuccess(context.Background(), "u", "i", time.Time{}); err == nil {
		t.Fatalf("expected mark success error on zero store")
	}
	if _, err := zeroStore.CreateAttempt(context.Background(), Attempt{}); err == nil {
		t.Fatalf("expected create attempt error on zero store")
	}
	if _, err := zeroStore.GetAttemptByID(context.Background(), "u", "a"); err == nil {
		t.Fatalf("expected get attempt by id error on zero store")
	}
	if _, err := zeroStore.GetAttemptByJobID(context.Background(), "j"); err == nil {
		t.Fatalf("expected get attempt by job id error on zero store")
	}
	if _, err := zeroStore.GetLatestAttemptByItem(context.Background(), "u", "i"); err == nil {
		t.Fatalf("expected get latest attempt error on zero store")
	}
	if _, err := zeroStore.ListItems(context.Background(), "u", ListFilter{}); err == nil {
		t.Fatalf("expected list items error on zero store")
	}
	if _, err := zeroStore.GetStats(context.Background(), "u"); err == nil {
		t.Fatalf("expected get stats error on zero store")
	}
	if _, err := zeroStore.UpdateAttempt(context.Background(), "u", "a", nil); err == nil {
		t.Fatalf("expected update attempt error on zero store")
	}
}

func TestStoreGetAndDeleteWrappers(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	backend.getItemByIDFn = func(_ context.Context, userID, itemID string) (Item, error) {
		if userID != "user_1" || itemID != "his_1" {
			t.Fatalf("unexpected get item args user=%q item=%q", userID, itemID)
		}
		return Item{ID: itemID, UserID: userID}, nil
	}

	item, err := store.GetItemByID(context.Background(), " user_1 ", " his_1 ")
	if err != nil {
		t.Fatalf("unexpected get item wrapper error: %v", err)
	}
	if item.ID != "his_1" {
		t.Fatalf("unexpected item from get wrapper: %+v", item)
	}

	var softDeleteAt time.Time
	backend.softDeleteItemFn = func(_ context.Context, userID, itemID string, deletedAt time.Time) error {
		if userID != "user_1" || itemID != "his_1" {
			t.Fatalf("unexpected soft delete args user=%q item=%q", userID, itemID)
		}
		softDeleteAt = deletedAt
		return nil
	}

	if err := store.SoftDeleteItem(context.Background(), " user_1 ", " his_1 ", time.Time{}); err != nil {
		t.Fatalf("unexpected soft delete wrapper error: %v", err)
	}
	if softDeleteAt.IsZero() {
		t.Fatalf("expected soft delete timestamp to be auto-filled")
	}

	backend.getByJobIDFn = func(_ context.Context, jobID string) (Attempt, error) {
		if jobID != "job_1" {
			t.Fatalf("unexpected job id: %q", jobID)
		}
		return Attempt{ID: "hat_1"}, nil
	}

	attempt, err := store.GetAttemptByJobID(context.Background(), " job_1 ")
	if err != nil {
		t.Fatalf("unexpected get by job id error: %v", err)
	}
	if attempt.ID != "hat_1" {
		t.Fatalf("unexpected attempt from job lookup: %+v", attempt)
	}
}

func TestStoreInputValidationDetails(t *testing.T) {
	store := &Store{backend: &fakeBackend{}}

	if _, err := store.GetItemByID(context.Background(), "", "x"); err == nil {
		t.Fatalf("expected validation error for missing user in get item")
	}
	if err := store.SoftDeleteItem(context.Background(), "u", "", time.Time{}); err == nil {
		t.Fatalf("expected validation error for missing item in soft delete")
	}
	if err := store.MarkItemSuccess(context.Background(), "", "i", time.Time{}); err == nil {
		t.Fatalf("expected validation error for missing user in mark success")
	}
	if _, err := store.GetAttemptByID(context.Background(), "", "a"); err == nil {
		t.Fatalf("expected validation error for missing user in get attempt")
	}
	if _, err := store.GetAttemptByJobID(context.Background(), ""); err == nil {
		t.Fatalf("expected validation error for empty job id")
	}
	if _, err := store.GetLatestAttemptByItem(context.Background(), "u", ""); err == nil {
		t.Fatalf("expected validation error for missing item in latest attempt")
	}
	if _, err := store.ListItems(context.Background(), "", ListFilter{}); err == nil {
		t.Fatalf("expected validation error for empty user in list")
	}
	if _, err := store.GetStats(context.Background(), ""); err == nil {
		t.Fatalf("expected validation error for empty user in stats")
	}
}

func TestStoreCloseNoopAndItemSortAt(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("expected nil store close to be noop, got %v", err)
	}

	store := &Store{}
	if err := store.Close(); err != nil {
		t.Fatalf("expected store close without backend to be noop, got %v", err)
	}

	now := time.Now().UTC()
	if got := itemSortAt(Item{CreatedAt: now}); !got.Equal(now) {
		t.Fatalf("expected fallback sort_at from created_at, got %v", got)
	}
	later := now.Add(time.Minute)
	if got := itemSortAt(Item{CreatedAt: now, LastAttemptAt: &later}); !got.Equal(later) {
		t.Fatalf("expected sort_at from last_attempt_at, got %v", got)
	}
}

func TestNormalizeSourceURLAndValidationHelpers(t *testing.T) {
	raw := " HTTPS://WWW.YOUTUBE.COM/watch?v=abc&z=9&a=1#frag "
	normalized := normalizeSourceURL(raw)
	if normalized != "https://www.youtube.com/watch?a=1&v=abc&z=9" {
		t.Fatalf("unexpected normalized url: %s", normalized)
	}

	if normalized := normalizeSourceURL("::bad-url::"); normalized != "::bad-url::" {
		t.Fatalf("expected invalid url to remain as-is, got %q", normalized)
	}

	if !isValidRequestKind(RequestKindImage) {
		t.Fatalf("expected image request kind to be valid")
	}
	if isValidRequestKind(RequestKind("archive")) {
		t.Fatalf("expected custom request kind to be invalid")
	}

	hashA := hashSourceURL("https://example.com/watch?v=1")
	hashB := hashSourceURL("https://example.com/watch?v=1")
	hashC := hashSourceURL("https://example.com/watch?v=2")
	if hashA != hashB {
		t.Fatalf("expected deterministic hash, got %s vs %s", hashA, hashB)
	}
	if hashA == hashC {
		t.Fatalf("expected different hashes for different URLs")
	}
}

func TestStorePassThroughForStatsAndLatestErrors(t *testing.T) {
	expectedErr := errors.New("backend fail")
	backend := &fakeBackend{
		getStatsFn: func(context.Context, string) (Stats, error) {
			return Stats{}, expectedErr
		},
		getLatestAttemptByItemFn: func(context.Context, string, string) (Attempt, error) {
			return Attempt{}, expectedErr
		},
	}
	store := &Store{backend: backend}

	if _, err := store.GetStats(context.Background(), "user_1"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected stats passthrough error, got %v", err)
	}
	if _, err := store.GetLatestAttemptByItem(context.Background(), "user_1", "his_1"); !errors.Is(err, expectedErr) {
		t.Fatalf("expected latest attempt passthrough error, got %v", err)
	}
}

func TestStoreValidationBranchCoverage(t *testing.T) {
	store := &Store{backend: &fakeBackend{}}

	if _, err := store.UpsertItem(context.Background(), Item{UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=a"}); err == nil {
		t.Fatalf("expected upsert validation error for missing item id")
	}
	if _, err := store.UpsertItem(context.Background(), Item{ID: "his_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=a"}); err == nil {
		t.Fatalf("expected upsert validation error for missing user id")
	}
	if _, err := store.UpsertItem(context.Background(), Item{ID: "his_1", UserID: "user_1", Platform: PlatformYouTube}); err == nil {
		t.Fatalf("expected upsert validation error for missing source url")
	}

	invalidCreateCases := []Attempt{
		{HistoryItemID: "his_1", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued},
		{ID: "hat_1", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued},
		{ID: "hat_1", HistoryItemID: "his_1", RequestKind: RequestKindMP3, Status: StatusQueued},
		{ID: "hat_1", HistoryItemID: "his_1", UserID: "user_1", RequestKind: RequestKind("archive"), Status: StatusQueued},
		{ID: "hat_1", HistoryItemID: "his_1", UserID: "user_1", RequestKind: RequestKindMP3, Status: AttemptStatus("invalid")},
	}
	for index, attempt := range invalidCreateCases {
		if _, err := store.CreateAttempt(context.Background(), attempt); err == nil {
			t.Fatalf("expected create attempt validation error for case %d", index)
		}
	}
}

func TestNewStoreWithLoggerBranches(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	memoryStore := NewStore(config.Config{}, logger)
	if memoryStore == nil {
		t.Fatalf("expected memory store instance")
	}
	if _, ok := memoryStore.backend.(*memoryBackend); !ok {
		t.Fatalf("expected memory backend with empty postgres dsn")
	}
	_ = memoryStore.Close()

	postgresStore := NewStore(config.Config{PostgresDSN: "postgres://u:p@127.0.0.1:5435/ytd?sslmode=disable"}, logger)
	if postgresStore == nil {
		t.Fatalf("expected postgres store instance")
	}
	if _, ok := postgresStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend with postgres dsn")
	}
	_ = postgresStore.Close()
}

func TestStoreListAndUpdateMutationBranches(t *testing.T) {
	backend := &fakeBackend{}
	store := &Store{backend: backend}

	backend.listItemsFn = func(_ context.Context, userID string, filter ListFilter) (ListPage, error) {
		if filter.Limit != MaxListLimit {
			t.Fatalf("expected limit clamped to MaxListLimit, got %d", filter.Limit)
		}
		if filter.Cursor == nil {
			t.Fatalf("expected cursor to be present")
		}
		if filter.Cursor.SortAt.Location() != time.UTC {
			t.Fatalf("expected cursor sortAt in UTC, got %v", filter.Cursor.SortAt.Location())
		}
		return ListPage{}, nil
	}

	localTime := time.Date(2026, 3, 22, 18, 0, 0, 0, time.FixedZone("UTC+7", 7*3600))
	if _, err := store.ListItems(context.Background(), "user_1", ListFilter{Limit: MaxListLimit + 100, Cursor: &ListCursor{SortAt: localTime, ItemID: "his_1"}}); err != nil {
		t.Fatalf("unexpected list items error: %v", err)
	}

	baseAttempt := Attempt{ID: "hat_1", UserID: "user_1", HistoryItemID: "his_1", RequestKind: RequestKindMP3, Status: StatusQueued}
	backend.getAttemptByIDFn = func(context.Context, string, string) (Attempt, error) { return baseAttempt, nil }

	if _, err := store.UpdateAttempt(context.Background(), "user_1", "hat_1", func(a *Attempt) {
		a.RequestKind = RequestKind("archive")
	}); err == nil {
		t.Fatalf("expected update attempt error for invalid request kind mutation")
	}

	if _, err := store.UpdateAttempt(context.Background(), "user_1", "hat_1", func(a *Attempt) {
		a.RequestKind = RequestKindMP3
		a.Status = AttemptStatus("invalid")
	}); err == nil {
		t.Fatalf("expected update attempt error for invalid status mutation")
	}

	backend.updateAttemptFn = func(context.Context, Attempt) error { return errors.New("update failed") }
	if _, err := store.UpdateAttempt(context.Background(), "user_1", "hat_1", nil); err == nil {
		t.Fatalf("expected update attempt passthrough error")
	}
}
