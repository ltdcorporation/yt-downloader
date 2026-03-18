package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisBackendIntegration_PutGetListAndLimit(t *testing.T) {
	backend := newIntegrationRedisBackend(t, 72*time.Hour)
	ctx := context.Background()

	now := time.Now().UTC()
	record1 := Record{
		ID:         "job_r1",
		Status:     StatusQueued,
		InputURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKind: "mp3",
		OutputKey:  "yt-downloader/prod/mp3/job_r1.mp3",
		Title:      "Video One",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	record2 := Record{
		ID:          "job_r2",
		Status:      StatusDone,
		InputURL:    "https://www.youtube.com/watch?v=def456",
		OutputKind:  "mp3",
		OutputKey:   "yt-downloader/prod/mp3/job_r2.mp3",
		Title:       "Video Two",
		DownloadURL: "https://example.com/job_r2.mp3",
		CreatedAt:   now.Add(time.Minute),
		UpdatedAt:   now.Add(time.Minute),
	}

	if err := backend.Put(ctx, record1); err != nil {
		t.Fatalf("unexpected put error for record1: %v", err)
	}
	if err := backend.Put(ctx, record2); err != nil {
		t.Fatalf("unexpected put error for record2: %v", err)
	}

	got, err := backend.Get(ctx, record1.ID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.ID != record1.ID || got.Status != record1.Status || got.OutputKey != record1.OutputKey {
		t.Fatalf("unexpected record from get: %+v", got)
	}

	items, err := backend.ListRecent(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 item with limit=1, got %d", len(items))
	}
	if items[0].ID != record2.ID {
		t.Fatalf("expected latest record first, got %s", items[0].ID)
	}

	items, err = backend.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != record2.ID || items[1].ID != record1.ID {
		t.Fatalf("unexpected ordering from ListRecent: %+v", items)
	}

	_, err = backend.Get(ctx, "job_missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing record, got %v", err)
	}
}

func TestRedisBackendIntegration_ListRecentSkipsDanglingIndexEntries(t *testing.T) {
	backend := newIntegrationRedisBackend(t, 72*time.Hour)
	ctx := context.Background()

	now := time.Now().UTC()
	valid := Record{
		ID:         "job_valid",
		Status:     StatusQueued,
		InputURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKind: "mp3",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := backend.Put(ctx, valid); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	if err := backend.client.ZAdd(ctx, backend.jobsIndexKey(), redis.Z{
		Score:  float64(now.Add(time.Minute).Unix()),
		Member: "job_dangling",
	}).Err(); err != nil {
		t.Fatalf("failed to insert dangling index member: %v", err)
	}

	items, err := backend.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only 1 valid item (dangling skipped), got %d", len(items))
	}
	if items[0].ID != valid.ID {
		t.Fatalf("unexpected listed item id: %s", items[0].ID)
	}
}

func TestRedisBackendIntegration_PutPrunesExpiredIndexEntries(t *testing.T) {
	backend := newIntegrationRedisBackend(t, 24*time.Hour)
	ctx := context.Background()

	oldID := "job_old"
	oldUnix := time.Now().Add(-72 * time.Hour).Unix()
	if err := backend.client.ZAdd(ctx, backend.jobsIndexKey(), redis.Z{
		Score:  float64(oldUnix),
		Member: oldID,
	}).Err(); err != nil {
		t.Fatalf("failed to seed old index entry: %v", err)
	}

	recent := Record{
		ID:         "job_recent",
		Status:     StatusQueued,
		InputURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKind: "mp3",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := backend.Put(ctx, recent); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	ids, err := backend.client.ZRange(ctx, backend.jobsIndexKey(), 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to read jobs index: %v", err)
	}

	for _, id := range ids {
		if id == oldID {
			t.Fatalf("expected old index entry to be pruned, but found %s", oldID)
		}
	}
}
