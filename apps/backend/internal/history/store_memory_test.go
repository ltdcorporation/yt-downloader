package history

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBackend_UpsertGetSoftDelete(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_1",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=abc",
		SourceURLHash: "hash_1",
		Title:         "title-1",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	if item.ID != "his_1" {
		t.Fatalf("expected item id his_1, got %s", item.ID)
	}

	updated, err := backend.UpsertItem(ctx, Item{
		ID:            "his_2",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=abc",
		SourceURLHash: "hash_1",
		Title:         "title-2",
		UpdatedAt:     now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected update by hash error: %v", err)
	}
	if updated.ID != "his_1" {
		t.Fatalf("expected upsert conflict to keep original item id, got %s", updated.ID)
	}
	if updated.Title != "title-2" {
		t.Fatalf("expected title update, got %s", updated.Title)
	}

	if err := backend.SoftDeleteItem(ctx, "user_1", "his_1", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected soft-delete error: %v", err)
	}

	_, err = backend.GetItemByID(ctx, "user_1", "his_1")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound after soft delete, got %v", err)
	}
}

func TestMemoryBackend_ListAndStats(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item1, err := backend.UpsertItem(ctx, Item{
		ID:            "his_1",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=abc",
		SourceURLHash: "hash_1",
		Title:         "Alpha Video",
		LastAttemptAt: ptrTimeValue(now.Add(-time.Minute)),
		AttemptCount:  1,
		CreatedAt:     now.Add(-2 * time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected upsert item1 error: %v", err)
	}

	item2, err := backend.UpsertItem(ctx, Item{
		ID:            "his_2",
		UserID:        "user_1",
		Platform:      PlatformTikTok,
		SourceURL:     "https://tiktok.com/@user/video/1",
		SourceURLHash: "hash_2",
		Title:         "Beta Clip",
		LastAttemptAt: ptrTimeValue(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item2 error: %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_1",
		HistoryItemID: item1.ID,
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusDone,
		SizeBytes:     ptrInt64(123),
		CreatedAt:     now.Add(-time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("unexpected create attempt for item1: %v", err)
	}
	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_2",
		HistoryItemID: item2.ID,
		UserID:        "user_1",
		RequestKind:   RequestKindMP4,
		Status:        StatusFailed,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("unexpected create attempt for item2: %v", err)
	}

	page, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected first page with 1 entry, got %d", len(page.Entries))
	}
	if page.Entries[0].Item.ID != item2.ID {
		t.Fatalf("expected newest item first, got %s", page.Entries[0].Item.ID)
	}
	if !page.HasMore || page.NextCursor == nil {
		t.Fatalf("expected has_more cursor on first page")
	}

	nextPage, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("unexpected second page error: %v", err)
	}
	if len(nextPage.Entries) != 1 || nextPage.Entries[0].Item.ID != item1.ID {
		t.Fatalf("unexpected second page entries: %+v", nextPage.Entries)
	}

	filtered, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10, Platform: PlatformYouTube, Query: "alpha", Status: StatusDone})
	if err != nil {
		t.Fatalf("unexpected filtered list error: %v", err)
	}
	if len(filtered.Entries) != 1 || filtered.Entries[0].Item.ID != item1.ID {
		t.Fatalf("unexpected filtered entries: %+v", filtered.Entries)
	}

	stats, err := backend.GetStats(ctx, "user_1")
	if err != nil {
		t.Fatalf("unexpected stats error: %v", err)
	}
	if stats.TotalItems != 2 || stats.TotalAttempts != 2 {
		t.Fatalf("unexpected stats counts: %+v", stats)
	}
	if stats.SuccessCount != 1 || stats.FailedCount != 1 {
		t.Fatalf("unexpected status aggregates: %+v", stats)
	}
	if stats.TotalBytesDownloaded != 123 {
		t.Fatalf("unexpected total bytes downloaded: %+v", stats)
	}
}

func TestMemoryBackend_AttemptLifecycle(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := backend.UpsertItem(ctx, Item{
		ID:            "his_1",
		UserID:        "user_1",
		Platform:      PlatformTikTok,
		SourceURL:     "https://www.tiktok.com/@a/video/1",
		SourceURLHash: "hash_1",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	if err := backend.MarkItemSuccess(ctx, "user_1", "his_1", now.Add(30*time.Second)); err != nil {
		t.Fatalf("unexpected mark item success error: %v", err)
	}
	itemAfterSuccess, err := backend.GetItemByID(ctx, "user_1", "his_1")
	if err != nil {
		t.Fatalf("unexpected get item after success error: %v", err)
	}
	if itemAfterSuccess.LastSuccessAt == nil {
		t.Fatalf("expected last_success_at to be set")
	}

	attempt := Attempt{
		ID:            "hat_1",
		HistoryItemID: "his_1",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		JobID:         "job_1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := backend.CreateAttempt(ctx, attempt); err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	gotByID, err := backend.GetAttemptByID(ctx, "user_1", "hat_1")
	if err != nil {
		t.Fatalf("unexpected get by id error: %v", err)
	}
	if gotByID.JobID != "job_1" {
		t.Fatalf("expected job_1, got %s", gotByID.JobID)
	}

	gotByJobID, err := backend.GetAttemptByJobID(ctx, "job_1")
	if err != nil {
		t.Fatalf("unexpected get by job id error: %v", err)
	}
	if gotByJobID.ID != "hat_1" {
		t.Fatalf("expected hat_1, got %s", gotByJobID.ID)
	}

	gotByID.Status = StatusDone
	url := "https://example.com/file.mp3"
	gotByID.DownloadURL = url
	gotByID.UpdatedAt = now.Add(time.Minute)
	if err := backend.UpdateAttempt(ctx, gotByID); err != nil {
		t.Fatalf("unexpected update attempt error: %v", err)
	}

	updated, err := backend.GetAttemptByID(ctx, "user_1", "hat_1")
	if err != nil {
		t.Fatalf("unexpected get updated attempt error: %v", err)
	}
	if updated.Status != StatusDone {
		t.Fatalf("expected status done, got %s", updated.Status)
	}
	if updated.DownloadURL != url {
		t.Fatalf("expected updated download url, got %q", updated.DownloadURL)
	}
}

func ptrTimeValue(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}

func ptrInt64(value int64) *int64 {
	v := value
	return &v
}
