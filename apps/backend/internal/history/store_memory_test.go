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
