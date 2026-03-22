package history

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewPostgresBackend_CloseAndEnsureReady(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	if backend == nil {
		t.Fatalf("expected postgres backend instance")
	}

	ctx, cancel := integrationContext(t)
	defer cancel()
	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("unexpected ensure ready error: %v", err)
	}
	// second call should hit schemaReady fast path
	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("unexpected ensure ready second call error: %v", err)
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestPostgresBackendIntegration_NotFoundAndEmptyStatsPaths(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx, cancel := integrationContext(t)
	defer cancel()

	if _, err := backend.GetItemByID(ctx, "user_1", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for missing item, got %v", err)
	}
	if err := backend.SoftDeleteItem(ctx, "user_1", "missing", time.Now().UTC()); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for missing soft delete, got %v", err)
	}
	if err := backend.MarkItemSuccess(ctx, "user_1", "missing", time.Now().UTC()); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for missing mark success, got %v", err)
	}

	if _, err := backend.GetAttemptByID(ctx, "user_1", "missing"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for missing attempt by id, got %v", err)
	}
	if _, err := backend.GetAttemptByJobID(ctx, "missing_job"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for missing attempt by job id, got %v", err)
	}
	if _, err := backend.GetLatestAttemptByItem(ctx, "user_1", "missing_item"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for latest attempt by missing item, got %v", err)
	}
	if err := backend.UpdateAttempt(ctx, Attempt{ID: "missing", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued, UpdatedAt: time.Now().UTC()}); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for update missing attempt, got %v", err)
	}

	page, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error on empty db: %v", err)
	}
	if len(page.Entries) != 0 || page.HasMore || page.NextCursor != nil {
		t.Fatalf("unexpected empty list page: %+v", page)
	}

	stats, err := backend.GetStats(ctx, "user_1")
	if err != nil {
		t.Fatalf("unexpected empty stats error: %v", err)
	}
	if stats.TotalItems != 0 || stats.TotalAttempts != 0 || stats.SuccessCount != 0 || stats.FailedCount != 0 || stats.TotalBytesDownloaded != 0 || stats.ThisMonthAttempts != 0 {
		t.Fatalf("unexpected empty stats values: %+v", stats)
	}
}

func TestPostgresBackendIntegration_ListItemWithoutAttempt(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_no_attempt",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=no-attempt",
		SourceURLHash: "hash_no_attempt",
		Title:         "No Attempt Yet",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	page, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(page.Entries))
	}
	entry := page.Entries[0]
	if entry.Item.ID != item.ID {
		t.Fatalf("unexpected item id in page: %+v", entry.Item)
	}
	if entry.LatestAttempt != nil {
		t.Fatalf("expected latest attempt to be nil for item without attempts, got %+v", entry.LatestAttempt)
	}
}

func TestMapPostgresWriteError(t *testing.T) {
	if err := mapPostgresWriteError(nil); err != nil {
		t.Fatalf("expected nil when input error nil, got %v", err)
	}

	if err := mapPostgresWriteError(errors.New("plain")); err != nil {
		t.Fatalf("expected nil for non-pg error, got %v", err)
	}

	if err := mapPostgresWriteError(&pgconn.PgError{Code: "23505"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for unique violation, got %v", err)
	}

	if err := mapPostgresWriteError(&pgconn.PgError{Code: "23503", ConstraintName: "fk_history_attempt_item_user"}); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for fk_history_attempt_item_user, got %v", err)
	}

	if err := mapPostgresWriteError(&pgconn.PgError{Code: "23503", ConstraintName: "other_constraint"}); err != nil {
		t.Fatalf("expected nil for unrelated fk violation, got %v", err)
	}
}

func TestNullIfEmpty(t *testing.T) {
	if got := nullIfEmpty("   "); got != nil {
		t.Fatalf("expected nil for empty string, got %#v", got)
	}
	if got := nullIfEmpty("abc"); got != "abc" {
		t.Fatalf("expected passthrough string, got %#v", got)
	}
}
