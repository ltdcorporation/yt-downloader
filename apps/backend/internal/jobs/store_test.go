package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeBackend struct {
	records    map[string]Record
	putErr     error
	getErr     error
	listErr    error
	closeCalls int
	lastPut    Record
	lastLimit  int
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{records: make(map[string]Record)}
}

func (f *fakeBackend) Put(_ context.Context, record Record) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.lastPut = record
	f.records[record.ID] = record
	return nil
}

func (f *fakeBackend) Get(_ context.Context, jobID string) (Record, error) {
	if f.getErr != nil {
		return Record{}, f.getErr
	}
	record, ok := f.records[jobID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

func (f *fakeBackend) ListRecent(_ context.Context, limit int) ([]Record, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.lastLimit = limit
	items := make([]Record, 0, len(f.records))
	for _, record := range f.records {
		items = append(items, record)
	}
	return items, nil
}

func (f *fakeBackend) Close() error {
	f.closeCalls++
	return nil
}

func TestStorePut_ValidatesIDAndAppliesTimestamps(t *testing.T) {
	backend := newFakeBackend()
	store := &Store{backend: backend}

	if err := store.Put(context.Background(), Record{}); err == nil {
		t.Fatal("expected error for empty job id")
	}

	record := Record{ID: "job_1", Status: StatusQueued}
	if err := store.Put(context.Background(), record); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}
	if backend.lastPut.CreatedAt.IsZero() || backend.lastPut.UpdatedAt.IsZero() {
		t.Fatalf("expected created/updated timestamps to be set")
	}
}

func TestStoreGet_TrimAndEmptyValidation(t *testing.T) {
	backend := newFakeBackend()
	backend.records["job_1"] = Record{ID: "job_1", Status: StatusQueued}
	store := &Store{backend: backend}

	if _, err := store.Get(context.Background(), "   "); err == nil {
		t.Fatal("expected error for empty/whitespace job id")
	}

	record, err := store.Get(context.Background(), "  job_1  ")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if record.ID != "job_1" {
		t.Fatalf("unexpected record id: %s", record.ID)
	}
}

func TestStoreUpdate_MutatesAndPersists(t *testing.T) {
	backend := newFakeBackend()
	oldTime := time.Now().UTC().Add(-time.Hour)
	backend.records["job_1"] = Record{ID: "job_1", Status: StatusQueued, UpdatedAt: oldTime, CreatedAt: oldTime}
	store := &Store{backend: backend}

	updated, err := store.Update(context.Background(), "job_1", func(r *Record) {
		r.Status = StatusDone
		r.DownloadURL = "https://example.com/file.mp3"
	})
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if updated.Status != StatusDone {
		t.Fatalf("expected status done, got %s", updated.Status)
	}
	if updated.DownloadURL == "" {
		t.Fatalf("expected download URL to be set")
	}
	if !updated.UpdatedAt.After(oldTime) {
		t.Fatalf("expected UpdatedAt to be refreshed")
	}
}

func TestStoreListRecent_AndClose(t *testing.T) {
	backend := newFakeBackend()
	backend.records["job_1"] = Record{ID: "job_1"}
	store := &Store{backend: backend}

	items, err := store.ListRecent(context.Background(), 20)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if backend.lastLimit != 20 {
		t.Fatalf("expected list limit 20, got %d", backend.lastLimit)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if backend.closeCalls != 1 {
		t.Fatalf("expected one close call, got %d", backend.closeCalls)
	}
}

func TestStorePassThroughErrors(t *testing.T) {
	backend := newFakeBackend()
	backend.putErr = errors.New("put failed")
	backend.getErr = errors.New("get failed")
	backend.listErr = errors.New("list failed")
	store := &Store{backend: backend}

	if err := store.Put(context.Background(), Record{ID: "job_x"}); err == nil {
		t.Fatal("expected put error")
	}
	if _, err := store.Get(context.Background(), "job_x"); err == nil {
		t.Fatal("expected get error")
	}
	if _, err := store.ListRecent(context.Background(), 10); err == nil {
		t.Fatal("expected list error")
	}
}

func TestNewStore_SelectsBackendByConfig(t *testing.T) {
	redisStore := NewStore(config.Config{RedisAddr: "127.0.0.1:6382"}, nil)
	if _, ok := redisStore.backend.(*redisBackend); !ok {
		t.Fatalf("expected redis backend when PostgresDSN is empty")
	}
	_ = redisStore.Close()

	pgStore := NewStore(config.Config{PostgresDSN: "postgres://u:p@127.0.0.1:5435/ytd?sslmode=disable"}, nil)
	if _, ok := pgStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend when PostgresDSN is set")
	}
	_ = pgStore.Close()
}
