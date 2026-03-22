package http

import (
	"context"
	"testing"
	"time"

	"yt-downloader/backend/internal/history"
)

func TestCreateHistoryAttempt_Branches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	_, userID := registerUserAndGetToken(t, server)

	if attempt, ok := (*Server)(nil).createHistoryAttempt(context.Background(), historyAttemptCreateParams{}); ok || attempt != nil {
		t.Fatalf("expected nil/false for nil server")
	}

	if attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{UserID: "   "}); ok || attempt != nil {
		t.Fatalf("expected nil/false for blank user id")
	}

	backupStore := server.historyStore
	server.historyStore = &history.Store{} // EnsureReady will fail
	if attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{UserID: userID, Platform: "youtube", SourceURL: "https://youtube.com/watch?v=1", RequestKind: history.RequestKindMP3, Status: history.StatusQueued}); ok || attempt != nil {
		t.Fatalf("expected nil/false when ensure ready fails")
	}
	server.historyStore = backupStore

	if attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{UserID: userID, Platform: "unknown", SourceURL: "https://example.com", RequestKind: history.RequestKindMP3, Status: history.StatusQueued}); ok || attempt != nil {
		t.Fatalf("expected nil/false for unsupported platform")
	}

	if attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{UserID: userID, Platform: "youtube", SourceURL: "", RequestKind: history.RequestKindMP3, Status: history.StatusQueued}); ok || attempt != nil {
		t.Fatalf("expected nil/false when item upsert validation fails")
	}

	if attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{UserID: userID, Platform: "youtube", SourceURL: "https://youtube.com/watch?v=invalid-attempt", RequestKind: history.RequestKind("archive"), Status: history.StatusQueued}); ok || attempt != nil {
		t.Fatalf("expected nil/false when attempt create validation fails")
	}

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	attempt, ok := server.createHistoryAttempt(context.Background(), historyAttemptCreateParams{
		UserID:       userID,
		Platform:     "youtube",
		SourceURL:    "https://youtube.com/watch?v=created",
		Title:        "Created Item",
		ThumbnailURL: "https://img.example.com/created.jpg",
		RequestKind:  history.RequestKindMP3,
		Status:       history.StatusQueued,
		FormatID:     "audio",
		QualityLabel: "192kbps",
		JobID:        "job_created",
		OutputKey:    "yt-downloader/prod/mp3/job_created.mp3",
		DownloadURL:  "https://signed.example.com/job_created.mp3",
		ExpiresAt:    &expiresAt,
	})
	if !ok || attempt == nil {
		t.Fatalf("expected created history attempt")
	}
	if attempt.RequestKind != history.RequestKindMP3 || attempt.Status != history.StatusQueued {
		t.Fatalf("unexpected created attempt: %+v", attempt)
	}
}

func TestMarkHistoryAttemptHelpers_Branches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	_, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_mark_helpers",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://youtube.com/watch?v=mark-helpers",
		SourceURLHash: "hash_mark_helpers",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item upsert error: %v", err)
	}

	attempt, err := server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_mark_helpers",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusProcessing,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected attempt create error: %v", err)
	}

	size := int64(777)
	server.markHistoryAttemptDone(&attempt, &size)
	updated, err := server.historyStore.GetAttemptByID(ctx, userID, attempt.ID)
	if err != nil {
		t.Fatalf("unexpected read after done mark: %v", err)
	}
	if updated.Status != history.StatusDone {
		t.Fatalf("expected done status, got %s", updated.Status)
	}
	if updated.SizeBytes == nil || *updated.SizeBytes != size {
		t.Fatalf("expected size bytes %d, got %+v", size, updated.SizeBytes)
	}

	if err := server.historyStore.SoftDeleteItem(ctx, userID, item.ID, time.Now().UTC()); err != nil {
		t.Fatalf("unexpected soft delete item error: %v", err)
	}
	// MarkItemSuccess will fail on deleted item path, but helper should return safely.
	server.markHistoryAttemptDone(&updated, nil)

	server.markHistoryAttemptFailed(&updated, "failed_code", context.DeadlineExceeded)
	failed, err := server.historyStore.GetAttemptByID(ctx, userID, attempt.ID)
	if err != nil {
		t.Fatalf("unexpected read after failed mark: %v", err)
	}
	if failed.Status != history.StatusFailed {
		t.Fatalf("expected failed status, got %s", failed.Status)
	}
	if failed.ErrorCode != "failed_code" {
		t.Fatalf("unexpected error code after failed mark: %s", failed.ErrorCode)
	}

	// Non-existing attempt branch should be tolerated.
	server.markHistoryAttemptFailed(&history.Attempt{ID: "missing", UserID: userID}, "failed_code", context.Canceled)
	server.markHistoryAttemptDone(&history.Attempt{ID: "missing", UserID: userID}, nil)
}
