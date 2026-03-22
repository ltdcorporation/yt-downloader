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

func TestMemoryBackend_UpsertUpdateAndDeleteBranches(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_branch_1",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=branch",
		SourceURLHash: "hash_branch_1",
		Title:         "branch-a",
	})
	if err != nil {
		t.Fatalf("unexpected first upsert error: %v", err)
	}
	if item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		t.Fatalf("expected auto timestamps on first upsert")
	}

	lastAttemptAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	lastSuccessAt := lastAttemptAt.Add(time.Minute)
	updated, err := backend.UpsertItem(ctx, Item{
		ID:            "his_branch_2",
		UserID:        "user_1",
		Platform:      PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=branch",
		SourceURLHash: "hash_branch_1",
		Title:         "branch-b",
		ThumbnailURL:  "https://img.example.com/branch.jpg",
		LastAttemptAt: &lastAttemptAt,
		LastSuccessAt: &lastSuccessAt,
		AttemptCount:  3,
		UpdatedAt:     lastSuccessAt,
	})
	if err != nil {
		t.Fatalf("unexpected second upsert error: %v", err)
	}
	if updated.ID != "his_branch_1" {
		t.Fatalf("expected merged item id his_branch_1, got %s", updated.ID)
	}
	if updated.ThumbnailURL == "" || updated.LastAttemptAt == nil || updated.LastSuccessAt == nil {
		t.Fatalf("expected thumbnail and last attempt/success fields to be updated: %+v", updated)
	}

	if err := backend.SoftDeleteItem(ctx, "user_1", "his_branch_1", time.Now().UTC()); err != nil {
		t.Fatalf("unexpected soft delete error: %v", err)
	}
	if err := backend.SoftDeleteItem(ctx, "user_1", "his_branch_1", time.Now().UTC()); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound on second soft delete, got %v", err)
	}
}

func TestMemoryBackend_AttemptMappingAndListBranches(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err := backend.UpsertItem(ctx, Item{ID: "his_a", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=a", SourceURLHash: "hash_a", LastAttemptAt: ptrTimeValue(now), AttemptCount: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert item A: %v", err)
	}
	_, err = backend.UpsertItem(ctx, Item{ID: "his_b", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=b", SourceURLHash: "hash_b", LastAttemptAt: ptrTimeValue(now), AttemptCount: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert item B: %v", err)
	}
	_, err = backend.UpsertItem(ctx, Item{ID: "his_other_user", UserID: "user_2", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=o", SourceURLHash: "hash_o", LastAttemptAt: ptrTimeValue(now), AttemptCount: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert item other user: %v", err)
	}
	_, err = backend.UpsertItem(ctx, Item{ID: "his_deleted", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=del", SourceURLHash: "hash_del", LastAttemptAt: ptrTimeValue(now), AttemptCount: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert item deleted: %v", err)
	}
	if err := backend.SoftDeleteItem(ctx, "user_1", "his_deleted", now.Add(time.Second)); err != nil {
		t.Fatalf("unexpected soft delete of branch deleted item: %v", err)
	}

	if _, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_a"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for item with no attempts, got %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{ID: "hat_a", HistoryItemID: "his_a", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusDone, JobID: "job_a", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("unexpected create attempt A: %v", err)
	}
	if err := backend.CreateAttempt(ctx, Attempt{ID: "hat_b", HistoryItemID: "his_b", UserID: "user_1", RequestKind: RequestKindMP4, Status: StatusQueued, JobID: "job_b", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("unexpected create attempt B: %v", err)
	}

	updatedB := Attempt{ID: "hat_b", HistoryItemID: "his_b", UserID: "user_1", RequestKind: RequestKindMP4, Status: StatusProcessing, JobID: "job_b_new", CreatedAt: now, UpdatedAt: now.Add(time.Second)}
	if err := backend.UpdateAttempt(ctx, updatedB); err != nil {
		t.Fatalf("unexpected update attempt B: %v", err)
	}
	if _, err := backend.GetAttemptByJobID(ctx, "job_b"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected old job mapping removed after update, got %v", err)
	}
	if _, err := backend.GetAttemptByJobID(ctx, "job_b_new"); err != nil {
		t.Fatalf("expected new job mapping active, got %v", err)
	}

	// job id conflict on update branch
	conflictUpdate := updatedB
	conflictUpdate.JobID = "job_a"
	if err := backend.UpdateAttempt(ctx, conflictUpdate); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on update to duplicate job id, got %v", err)
	}

	// stale mapping branch: mapped id exists in index but missing in attempts map
	backend.attemptIDByJobID["stale_job"] = "missing_attempt"
	if _, err := backend.GetAttemptByJobID(ctx, "stale_job"); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound for stale job mapping, got %v", err)
	}

	// latest attempt for item without attempts
	if _, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_deleted"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for deleted item latest attempt, got %v", err)
	}
	if _, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_other_user"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for foreign user item latest attempt, got %v", err)
	}

	page, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 0, Query: "nomatch", Status: StatusDone})
	if err != nil {
		t.Fatalf("unexpected list with query/status mismatch error: %v", err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("expected empty list for mismatch query/status, got %+v", page.Entries)
	}

	// force limit > max branch and tie-sort + cursor branch
	cursor := &ListCursor{SortAt: now.Add(-time.Second), ItemID: "zzz"}
	page, err = backend.ListItems(ctx, "user_1", ListFilter{Limit: MaxListLimit + 1, Cursor: cursor})
	if err != nil {
		t.Fatalf("unexpected list with cursor error: %v", err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("expected cursor to skip newer records, got %+v", page.Entries)
	}
}

func TestMemoryBackend_GetStatsSkipBranches(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	active, err := backend.UpsertItem(ctx, Item{ID: "his_stats_active", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=s1", SourceURLHash: "hash_s1", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert active item: %v", err)
	}
	_, err = backend.UpsertItem(ctx, Item{ID: "his_stats_other", UserID: "user_2", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=s2", SourceURLHash: "hash_s2", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert other item: %v", err)
	}
	deleted, err := backend.UpsertItem(ctx, Item{ID: "his_stats_deleted", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=s3", SourceURLHash: "hash_s3", CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("unexpected upsert deleted item: %v", err)
	}
	if err := backend.SoftDeleteItem(ctx, "user_1", deleted.ID, now.Add(time.Second)); err != nil {
		t.Fatalf("unexpected soft delete stats item: %v", err)
	}

	if err := backend.CreateAttempt(ctx, Attempt{ID: "hat_stats_done", HistoryItemID: active.ID, UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusDone, SizeBytes: ptrInt64(55), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("unexpected create done attempt: %v", err)
	}
	if err := backend.CreateAttempt(ctx, Attempt{ID: "hat_stats_other_user", HistoryItemID: active.ID, UserID: "user_2", RequestKind: RequestKindMP3, Status: StatusDone, SizeBytes: ptrInt64(999), CreatedAt: now, UpdatedAt: now}); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for mismatched user/item on create attempt, got %v", err)
	}
	// inject attempts to hit skip branches in stats aggregation
	backend.attemptsByID["hat_orphan"] = Attempt{ID: "hat_orphan", HistoryItemID: "missing_item", UserID: "user_1", Status: StatusDone, CreatedAt: now}
	backend.attemptsByID["hat_other_user"] = Attempt{ID: "hat_other_user", HistoryItemID: active.ID, UserID: "user_2", Status: StatusDone, CreatedAt: now}

	stats, err := backend.GetStats(ctx, "user_1")
	if err != nil {
		t.Fatalf("unexpected stats error: %v", err)
	}
	if stats.TotalItems != 1 {
		t.Fatalf("expected 1 active item, got %+v", stats)
	}
	if stats.TotalAttempts != 1 || stats.SuccessCount != 1 || stats.TotalBytesDownloaded != 55 {
		t.Fatalf("unexpected stats aggregates: %+v", stats)
	}
}
