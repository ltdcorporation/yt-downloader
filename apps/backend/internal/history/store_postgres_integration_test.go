package history

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPostgresBackendIntegration_ItemLifecycle(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_pg_1",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=abc123",
		SourceURLHash: "hash_abc123",
		Title:         "Video One",
		ThumbnailURL:  "https://img.example.com/1.jpg",
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}
	if item.ID != "his_pg_1" {
		t.Fatalf("unexpected item id: %s", item.ID)
	}

	upsertedAgain, err := backend.UpsertItem(ctx, Item{
		ID:            "his_pg_2",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=abc123",
		SourceURLHash: "hash_abc123",
		Title:         "Video One Updated",
		AttemptCount:  1,
		CreatedAt:     now.Add(time.Minute),
		UpdatedAt:     now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected second upsert error: %v", err)
	}
	if upsertedAgain.ID != item.ID {
		t.Fatalf("expected same item id on hash conflict, got %s", upsertedAgain.ID)
	}
	if upsertedAgain.AttemptCount != 2 {
		t.Fatalf("expected attempt_count 2, got %d", upsertedAgain.AttemptCount)
	}

	got, err := backend.GetItemByID(ctx, "user_1", item.ID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.Title != "Video One Updated" {
		t.Fatalf("expected updated title, got %q", got.Title)
	}

	if err := backend.SoftDeleteItem(ctx, "user_1", item.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected soft delete error: %v", err)
	}
	_, err = backend.GetItemByID(ctx, "user_1", item.ID)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound after delete, got %v", err)
	}
}

func TestPostgresBackendIntegration_AttemptLifecycle(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_pg_1",
		UserID:        "user_1",
		Platform:      PlatformTikTok,
		SourceURL:     "https://www.tiktok.com/@foo/video/1",
		SourceURLHash: "hash_tiktok_1",
		Title:         "TikTok One",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	size := int64(123456)
	attempt := Attempt{
		ID:            "hat_pg_1",
		HistoryItemID: item.ID,
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		FormatID:      "audio-best",
		QualityLabel:  "128kbps",
		SizeBytes:     &size,
		JobID:         "job_pg_1",
		OutputKey:     "yt-downloader/prod/mp3/job_pg_1.mp3",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := backend.CreateAttempt(ctx, attempt); err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	gotByID, err := backend.GetAttemptByID(ctx, "user_1", "hat_pg_1")
	if err != nil {
		t.Fatalf("unexpected get by id error: %v", err)
	}
	if gotByID.JobID != "job_pg_1" {
		t.Fatalf("expected job_id job_pg_1, got %s", gotByID.JobID)
	}

	gotByJobID, err := backend.GetAttemptByJobID(ctx, "job_pg_1")
	if err != nil {
		t.Fatalf("unexpected get by job id error: %v", err)
	}
	if gotByJobID.ID != "hat_pg_1" {
		t.Fatalf("expected attempt id hat_pg_1, got %s", gotByJobID.ID)
	}

	updated := gotByID
	updated.Status = StatusDone
	downloadURL := "https://r2.example.com/job_pg_1.mp3"
	expiresAt := now.Add(2 * time.Hour)
	completedAt := now.Add(3 * time.Minute)
	updated.DownloadURL = downloadURL
	updated.ExpiresAt = &expiresAt
	updated.CompletedAt = &completedAt
	updated.UpdatedAt = now.Add(3 * time.Minute)

	if err := backend.UpdateAttempt(ctx, updated); err != nil {
		t.Fatalf("unexpected update attempt error: %v", err)
	}

	afterUpdate, err := backend.GetAttemptByID(ctx, "user_1", "hat_pg_1")
	if err != nil {
		t.Fatalf("unexpected get after update error: %v", err)
	}
	if afterUpdate.Status != StatusDone {
		t.Fatalf("expected status done, got %s", afterUpdate.Status)
	}
	if afterUpdate.DownloadURL != downloadURL {
		t.Fatalf("expected download URL %q, got %q", downloadURL, afterUpdate.DownloadURL)
	}
	if afterUpdate.ExpiresAt == nil || !afterUpdate.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expires_at %v, got %+v", expiresAt, afterUpdate.ExpiresAt)
	}

	successAt := now.Add(5 * time.Minute)
	if err := backend.MarkItemSuccess(ctx, "user_1", item.ID, successAt); err != nil {
		t.Fatalf("unexpected mark item success error: %v", err)
	}
	itemAfterSuccess, err := backend.GetItemByID(ctx, "user_1", item.ID)
	if err != nil {
		t.Fatalf("unexpected get item after success error: %v", err)
	}
	if itemAfterSuccess.LastSuccessAt == nil || !itemAfterSuccess.LastSuccessAt.Equal(successAt) {
		t.Fatalf("expected last_success_at %v, got %+v", successAt, itemAfterSuccess.LastSuccessAt)
	}

	duplicate := attempt
	duplicate.ID = "hat_pg_2"
	if err := backend.CreateAttempt(ctx, duplicate); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate job_id, got %v", err)
	}
}

func TestPostgresBackendIntegration_AttemptRequiresMatchingItemUser(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx, cancel := integrationContext(t)
	defer cancel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_pg_user1",
		UserID:        "user_1",
		Platform:      PlatformInstagram,
		SourceURL:     "https://www.instagram.com/p/abc",
		SourceURLHash: "hash_instagram_abc",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	err = backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_pg_mismatch",
		HistoryItemID: item.ID,
		UserID:        "user_2",
		RequestKind:   RequestKindMP4,
		Status:        StatusProcessing,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for mismatched user/item link, got %v", err)
	}
}
