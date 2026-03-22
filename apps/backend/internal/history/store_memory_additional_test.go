package history

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBackend_EnsureReadyAndNotFoundGuards(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("unexpected ensure ready error: %v", err)
	}

	if _, err := backend.GetAttemptByJobID(ctx, "missing"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound, got %v", err)
	}
	if _, err := backend.GetLatestAttemptByItem(ctx, "user_1", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
	if err := backend.MarkItemSuccess(ctx, "user_1", "missing", time.Now().UTC()); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for missing item mark success, got %v", err)
	}
}

func TestMemoryBackend_AttemptConflictsAndUpdateGuards(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := backend.UpsertItem(ctx, Item{
		ID:            "his_conflict",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=conflict",
		SourceURLHash: "hash_conflict",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	baseAttempt := Attempt{
		ID:            "hat_conflict_1",
		HistoryItemID: "his_conflict",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		JobID:         "job_conflict",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := backend.CreateAttempt(ctx, baseAttempt); err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	if err := backend.CreateAttempt(ctx, baseAttempt); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate attempt id, got %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_conflict_2",
		HistoryItemID: "his_conflict",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		JobID:         "job_conflict",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate job id, got %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_conflict_3",
		HistoryItemID: "missing_item",
		UserID:        "user_1",
		RequestKind:   RequestKindMP3,
		Status:        StatusQueued,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for missing item, got %v", err)
	}

	if err := backend.UpdateAttempt(ctx, Attempt{ID: "unknown", UserID: "user_1"}); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for missing attempt update, got %v", err)
	}

	if err := backend.UpdateAttempt(ctx, Attempt{ID: baseAttempt.ID, UserID: "user_2"}); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for wrong user update, got %v", err)
	}

	if _, err := backend.GetAttemptByID(ctx, "user_2", baseAttempt.ID); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for wrong user read, got %v", err)
	}
}

func TestMemoryBackend_GetLatestAttemptByItemAndCopyBranches(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := backend.UpsertItem(ctx, Item{
		ID:            "his_latest",
		UserID:        "user_1",
		Platform:      PlatformInstagram,
		SourceURL:     "https://instagram.com/p/latest",
		SourceURLHash: "hash_latest",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	firstSize := int64(42)
	expires := now.Add(10 * time.Minute)
	completed := now.Add(time.Minute)
	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_latest_1",
		HistoryItemID: "his_latest",
		UserID:        "user_1",
		RequestKind:   RequestKindImage,
		Status:        StatusDone,
		SizeBytes:     &firstSize,
		ExpiresAt:     &expires,
		CompletedAt:   &completed,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("unexpected create first attempt error: %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_latest_2",
		HistoryItemID: "his_latest",
		UserID:        "user_1",
		RequestKind:   RequestKindMP4,
		Status:        StatusProcessing,
		CreatedAt:     now.Add(2 * time.Minute),
		UpdatedAt:     now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("unexpected create second attempt error: %v", err)
	}

	latest, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_latest")
	if err != nil {
		t.Fatalf("unexpected latest attempt read error: %v", err)
	}
	if latest.ID != "hat_latest_2" {
		t.Fatalf("expected latest attempt hat_latest_2, got %s", latest.ID)
	}

	firstRead, err := backend.GetAttemptByID(ctx, "user_1", "hat_latest_1")
	if err != nil {
		t.Fatalf("unexpected first attempt read error: %v", err)
	}
	if firstRead.SizeBytes == nil || *firstRead.SizeBytes != firstSize {
		t.Fatalf("expected copied size bytes %d, got %+v", firstSize, firstRead.SizeBytes)
	}
	if firstRead.ExpiresAt == nil || !firstRead.ExpiresAt.Equal(expires) {
		t.Fatalf("expected copied expires_at %v, got %+v", expires, firstRead.ExpiresAt)
	}
	if firstRead.CompletedAt == nil || !firstRead.CompletedAt.Equal(completed) {
		t.Fatalf("expected copied completed_at %v, got %+v", completed, firstRead.CompletedAt)
	}
}

func TestMemoryBackend_ItemSortFallbackViaList(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := backend.UpsertItem(ctx, Item{
		ID:            "his_no_last_attempt",
		UserID:        "user_1",
		Platform:      PlatformX,
		SourceURL:     "https://x.com/status/1",
		SourceURLHash: "hash_x_1",
		Title:         "No Last Attempt",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	page, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(page.Entries))
	}
	if page.Entries[0].Item.ID != "his_no_last_attempt" {
		t.Fatalf("unexpected item in fallback sort list: %+v", page.Entries[0].Item)
	}
}
