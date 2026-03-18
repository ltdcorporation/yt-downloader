package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/jobs"
	queuepkg "yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/youtube"
)

type fakeResolver struct {
	result youtube.ResolveResult
	err    error
}

func (f *fakeResolver) Resolve(_ context.Context, _ string) (youtube.ResolveResult, error) {
	if f.err != nil {
		return youtube.ResolveResult{}, f.err
	}
	return f.result, nil
}

type fakeQueue struct {
	err         error
	enqueueTask *asynq.Task
	enqueueOpts []asynq.Option
}

func (f *fakeQueue) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.enqueueTask = task
	f.enqueueOpts = append([]asynq.Option{}, opts...)
	if f.err != nil {
		return nil, f.err
	}
	return &asynq.TaskInfo{ID: "fake-task"}, nil
}

func (f *fakeQueue) Close() error { return nil }

type fakeJobStore struct {
	records map[string]jobs.Record
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{records: make(map[string]jobs.Record)}
}

func (f *fakeJobStore) Close() error { return nil }

func (f *fakeJobStore) Put(_ context.Context, record jobs.Record) error {
	f.records[record.ID] = record
	return nil
}

func (f *fakeJobStore) Get(_ context.Context, jobID string) (jobs.Record, error) {
	record, ok := f.records[jobID]
	if !ok {
		return jobs.Record{}, jobs.ErrNotFound
	}
	return record, nil
}

func (f *fakeJobStore) Update(_ context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error) {
	record, ok := f.records[jobID]
	if !ok {
		return jobs.Record{}, jobs.ErrNotFound
	}
	if mutate != nil {
		mutate(&record)
	}
	f.records[jobID] = record
	return record, nil
}

func (f *fakeJobStore) ListRecent(_ context.Context, _ int) ([]jobs.Record, error) {
	items := make([]jobs.Record, 0, len(f.records))
	for _, record := range f.records {
		items = append(items, record)
	}
	return items, nil
}

func baseTestConfig() config.Config {
	return config.Config{
		RateLimitRPS:       0,
		MP3Bitrate:         192,
		R2KeyPrefix:        "yt-downloader/prod",
		AdminBasicAuthUser: "admin",
		AdminBasicAuthPass: "secret",
	}
}

func newTestServer(t *testing.T, cfg config.Config, resolver youtubeResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	logger := log.New(io.Discard, "", 0)
	return newServerWithDeps(cfg, logger, resolver, queue, store)
}

func TestBuildMP3OutputKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		jobID  string
		want   string
	}{
		{
			name:   "default without prefix",
			prefix: "",
			jobID:  "job_abc",
			want:   "mp3/job_abc.mp3",
		},
		{
			name:   "with clean prefix",
			prefix: "yt-downloader/prod",
			jobID:  "job_abc",
			want:   "yt-downloader/prod/mp3/job_abc.mp3",
		},
		{
			name:   "trim slash and space",
			prefix: " /yt-downloader/prod/ ",
			jobID:  " job_abc ",
			want:   "yt-downloader/prod/mp3/job_abc.mp3",
		},
		{
			name:   "fallback unknown job id",
			prefix: "yt-downloader/prod",
			jobID:  "",
			want:   "yt-downloader/prod/mp3/unknown.mp3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildMP3OutputKey(tc.prefix, tc.jobID)
			if got != tc.want {
				t.Fatalf("unexpected output key, got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestHandleCreateMP3Job_QueuesTaskAndPersistsQueuedRecord(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Test Video"}}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	body := bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	jobID, _ := response["job_id"].(string)
	if jobID == "" {
		t.Fatalf("missing job_id in response: %s", rec.Body.String())
	}
	if response["status"] != jobs.StatusQueued {
		t.Fatalf("unexpected status in response: %#v", response["status"])
	}

	record, ok := store.records[jobID]
	if !ok {
		t.Fatalf("job record not persisted, id=%s", jobID)
	}
	if record.Status != jobs.StatusQueued {
		t.Fatalf("unexpected record status: %s", record.Status)
	}
	if record.OutputKey != "yt-downloader/prod/mp3/"+jobID+".mp3" {
		t.Fatalf("unexpected output key: %s", record.OutputKey)
	}
	if record.Title != "Test Video" {
		t.Fatalf("unexpected record title: %s", record.Title)
	}

	if queue.enqueueTask == nil {
		t.Fatal("expected enqueue to be called")
	}
	if queue.enqueueTask.Type() != queuepkg.TaskConvertMP3 {
		t.Fatalf("unexpected task type: %s", queue.enqueueTask.Type())
	}

	var payload queuepkg.ConvertMP3Payload
	if err := json.Unmarshal(queue.enqueueTask.Payload(), &payload); err != nil {
		t.Fatalf("failed to decode queue payload: %v", err)
	}
	if payload.JobID != jobID {
		t.Fatalf("payload job id mismatch, got=%s want=%s", payload.JobID, jobID)
	}
	if payload.OutputKey != record.OutputKey {
		t.Fatalf("payload output key mismatch, got=%s want=%s", payload.OutputKey, record.OutputKey)
	}
	if payload.BitrateKbps != cfg.MP3Bitrate {
		t.Fatalf("payload bitrate mismatch, got=%d want=%d", payload.BitrateKbps, cfg.MP3Bitrate)
	}
}

func TestHandleCreateMP3Job_QueueFailureMarksJobFailed(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Test Video"}}
	queue := &fakeQueue{err: errors.New("enqueue failed")}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	body := bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}

	if len(store.records) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(store.records))
	}

	for _, record := range store.records {
		if record.Status != jobs.StatusFailed {
			t.Fatalf("expected failed status after enqueue error, got %s", record.Status)
		}
		if record.Error != "failed to enqueue job" {
			t.Fatalf("unexpected error message, got %q", record.Error)
		}
	}
}

func TestHandleRedirectMP4_ReturnsFoundForSelectedFormat(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{result: youtube.ResolveResult{
		Formats: []youtube.Format{
			{ID: "18", Type: "mp4", URL: "https://cdn.example/video-18.mp4"},
			{ID: "mp3-128", Type: "mp3"},
		},
	}}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://cdn.example/video-18.mp4" {
		t.Fatalf("unexpected redirect location: %s", got)
	}
}
