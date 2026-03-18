package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPostgresBackendIntegration_PutGetListAndFailedErrorLog(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn, 14)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	expires := now.Add(2 * time.Hour).Truncate(time.Microsecond)

	record1 := Record{
		ID:         "job_pg_1",
		Status:     StatusQueued,
		InputURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKind: "mp3",
		OutputKey:  "yt-downloader/prod/mp3/job_pg_1.mp3",
		Title:      "Video One",
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  &expires,
	}
	record2 := Record{
		ID:         "job_pg_2",
		Status:     StatusFailed,
		InputURL:   "https://www.youtube.com/watch?v=def456",
		OutputKind: "mp3",
		OutputKey:  "yt-downloader/prod/mp3/job_pg_2.mp3",
		Title:      "Video Two",
		Error:      "conversion failed",
		CreatedAt:  now.Add(time.Minute),
		UpdatedAt:  now.Add(time.Minute),
	}

	if err := backend.Put(ctx, record1); err != nil {
		t.Fatalf("unexpected put error for record1: %v", err)
	}
	if err := backend.Put(ctx, record2); err != nil {
		t.Fatalf("unexpected put error for record2: %v", err)
	}

	got1, err := backend.Get(ctx, record1.ID)
	if err != nil {
		t.Fatalf("unexpected get error for record1: %v", err)
	}
	if got1.ID != record1.ID || got1.Status != record1.Status || got1.OutputKey != record1.OutputKey {
		t.Fatalf("unexpected record1 from get: %+v", got1)
	}
	if got1.ExpiresAt == nil || !got1.ExpiresAt.Equal(expires) {
		t.Fatalf("expected expires_at=%v, got %+v", expires, got1.ExpiresAt)
	}

	got2, err := backend.Get(ctx, record2.ID)
	if err != nil {
		t.Fatalf("unexpected get error for record2: %v", err)
	}
	if got2.Status != StatusFailed || got2.Error != "conversion failed" {
		t.Fatalf("unexpected failed record2 fields: %+v", got2)
	}

	items, err := backend.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != record2.ID || items[1].ID != record1.ID {
		t.Fatalf("unexpected list order: first=%s second=%s", items[0].ID, items[1].ID)
	}

	_, err = backend.Get(ctx, "job_missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing job, got %v", err)
	}

	var errorRows int
	if err := backend.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_errors WHERE job_id = $1`, record2.ID).Scan(&errorRows); err != nil {
		t.Fatalf("failed to query job_errors count: %v", err)
	}
	if errorRows != 1 {
		t.Fatalf("expected 1 job_error row for failed record, got %d", errorRows)
	}
}

func TestPostgresBackendIntegration_CleanupRetention(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn, 100)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	oldTime := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Microsecond)
	recentTime := time.Now().UTC().Truncate(time.Microsecond)

	oldRecord := Record{
		ID:         "job_old",
		Status:     StatusDone,
		InputURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKind: "mp3",
		OutputKey:  "yt-downloader/prod/mp3/job_old.mp3",
		CreatedAt:  oldTime,
		UpdatedAt:  oldTime,
	}
	recentRecord := Record{
		ID:         "job_recent",
		Status:     StatusDone,
		InputURL:   "https://www.youtube.com/watch?v=def456",
		OutputKind: "mp3",
		OutputKey:  "yt-downloader/prod/mp3/job_recent.mp3",
		CreatedAt:  recentTime,
		UpdatedAt:  recentTime,
	}

	if err := backend.Put(ctx, oldRecord); err != nil {
		t.Fatalf("unexpected put error for old record: %v", err)
	}
	if err := backend.Put(ctx, recentRecord); err != nil {
		t.Fatalf("unexpected put error for recent record: %v", err)
	}

	backend.retentionDays = 1
	backend.lastCleanup = time.Time{}
	if err := backend.cleanupIfDue(ctx); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}

	_, err := backend.Get(ctx, oldRecord.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected old record to be removed by cleanup, got err=%v", err)
	}

	recentGot, err := backend.Get(ctx, recentRecord.ID)
	if err != nil {
		t.Fatalf("expected recent record to remain, got err=%v", err)
	}
	if recentGot.ID != recentRecord.ID {
		t.Fatalf("unexpected recent record after cleanup: %+v", recentGot)
	}
}
