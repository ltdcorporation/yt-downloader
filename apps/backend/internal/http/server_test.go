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
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/igresolver"
	"yt-downloader/backend/internal/jobs"
	queuepkg "yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/ttresolver"
	"yt-downloader/backend/internal/xresolver"
	"yt-downloader/backend/internal/youtube"
)

type fakeResolver struct {
	result youtube.ResolveResult
	err    error
	inputs []string
}

func (f *fakeResolver) Resolve(_ context.Context, rawURL string) (youtube.ResolveResult, error) {
	f.inputs = append(f.inputs, rawURL)
	if f.err != nil {
		return youtube.ResolveResult{}, f.err
	}
	return f.result, nil
}

type fakeXResolver struct {
	result xresolver.ResolveResult
	err    error
	inputs []string
}

func (f *fakeXResolver) Resolve(_ context.Context, rawURL string) (xresolver.ResolveResult, error) {
	f.inputs = append(f.inputs, rawURL)
	if f.err != nil {
		return xresolver.ResolveResult{}, f.err
	}
	return f.result, nil
}

type fakeIGResolver struct {
	result igresolver.ResolveResult
	err    error
	inputs []string
}

func (f *fakeIGResolver) Resolve(_ context.Context, rawURL string) (igresolver.ResolveResult, error) {
	f.inputs = append(f.inputs, rawURL)
	if f.err != nil {
		return igresolver.ResolveResult{}, f.err
	}
	return f.result, nil
}

type fakeTTResolver struct {
	result ttresolver.ResolveResult
	err    error
	inputs []string
}

func (f *fakeTTResolver) Resolve(_ context.Context, rawURL string) (ttresolver.ResolveResult, error) {
	f.inputs = append(f.inputs, rawURL)
	if f.err != nil {
		return ttresolver.ResolveResult{}, f.err
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
	records       map[string]jobs.Record
	putErr        error
	getErr        error
	updateErr     error
	listErr       error
	listItems     []jobs.Record
	lastListLimit int
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{records: make(map[string]jobs.Record)}
}

func (f *fakeJobStore) Close() error { return nil }

func (f *fakeJobStore) Put(_ context.Context, record jobs.Record) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.records[record.ID] = record
	return nil
}

func (f *fakeJobStore) Get(_ context.Context, jobID string) (jobs.Record, error) {
	if f.getErr != nil {
		return jobs.Record{}, f.getErr
	}
	record, ok := f.records[jobID]
	if !ok {
		return jobs.Record{}, jobs.ErrNotFound
	}
	return record, nil
}

func (f *fakeJobStore) Update(_ context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error) {
	if f.updateErr != nil {
		return jobs.Record{}, f.updateErr
	}
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

func (f *fakeJobStore) ListRecent(_ context.Context, limit int) ([]jobs.Record, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.lastListLimit = limit
	if f.listItems != nil {
		items := make([]jobs.Record, len(f.listItems))
		copy(items, f.listItems)
		return items, nil
	}

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
		CORSAllowedOrigins: "http://allowed.local",
	}
}

func newTestServer(t *testing.T, cfg config.Config, resolver youtubeResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	return newTestServerWithResolvers(t, cfg, resolver, &fakeXResolver{}, &fakeIGResolver{}, &fakeTTResolver{}, queue, store)
}

func newTestServerWithXResolver(t *testing.T, cfg config.Config, resolver youtubeResolver, xResolver xMediaResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	return newTestServerWithResolvers(t, cfg, resolver, xResolver, &fakeIGResolver{}, &fakeTTResolver{}, queue, store)
}

func newTestServerWithIGResolver(t *testing.T, cfg config.Config, resolver youtubeResolver, igResolver igMediaResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	return newTestServerWithResolvers(t, cfg, resolver, &fakeXResolver{}, igResolver, &fakeTTResolver{}, queue, store)
}

func newTestServerWithTTResolver(t *testing.T, cfg config.Config, resolver youtubeResolver, ttResolver ttMediaResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	return newTestServerWithResolvers(t, cfg, resolver, &fakeXResolver{}, &fakeIGResolver{}, ttResolver, queue, store)
}

func newTestServerWithResolvers(t *testing.T, cfg config.Config, resolver youtubeResolver, xResolver xMediaResolver, igResolver igMediaResolver, ttResolver ttMediaResolver, queue taskQueue, store jobStore) *Server {
	t.Helper()
	logger := log.New(io.Discard, "", 0)
	return newServerWithDeps(cfg, logger, resolver, xResolver, igResolver, ttResolver, queue, store)
}

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode JSON: %v body=%s", err, string(body))
	}
	return payload
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

func TestParseAllowedOrigins(t *testing.T) {
	origins := parseAllowedOrigins(" http://a.local , http://b.local ")
	if _, ok := origins["http://a.local"]; !ok {
		t.Fatalf("expected http://a.local to be allowed")
	}
	if _, ok := origins["http://b.local"]; !ok {
		t.Fatalf("expected http://b.local to be allowed")
	}

	fallback := parseAllowedOrigins("")
	if _, ok := fallback["http://127.0.0.1:3000"]; !ok {
		t.Fatalf("expected localhost fallback origin")
	}
}

func TestGetClientIPPriority(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	req.Header.Set("X-Real-IP", "3.3.3.3")

	if got := getClientIP(req); got != "1.1.1.1" {
		t.Fatalf("expected X-Forwarded-For first IP, got %s", got)
	}

	req.Header.Del("X-Forwarded-For")
	if got := getClientIP(req); got != "3.3.3.3" {
		t.Fatalf("expected X-Real-IP, got %s", got)
	}

	req.Header.Del("X-Real-IP")
	if got := getClientIP(req); got != "9.9.9.9" {
		t.Fatalf("expected RemoteAddr host, got %s", got)
	}
}

func TestHandleHealthz(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	payload := decodeJSONMap(t, rec.Body.Bytes())
	if payload["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", payload["ok"])
	}
	if payload["service"] != "api" {
		t.Fatalf("expected service=api, got %#v", payload["service"])
	}
}

func TestHandleResolveYouTube(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/youtube/resolve", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{err: errors.New("bad url")}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/youtube/resolve", bytes.NewBufferString(`{"url":"https://x"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "bad url" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Video OK"}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/youtube/resolve", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["title"] != "Video OK" {
			t.Fatalf("unexpected response payload: %#v", payload)
		}
		if len(resolver.inputs) != 1 || resolver.inputs[0] != "https://www.youtube.com/watch?v=abc123" {
			t.Fatalf("resolver should receive input URL, got %#v", resolver.inputs)
		}
	})
}

func TestHandleResolveX(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithXResolver(t, cfg, &fakeResolver{}, &fakeXResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/x/resolve", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty url", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithXResolver(t, cfg, &fakeResolver{}, &fakeXResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/x/resolve", bytes.NewBufferString(`{"url":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "url is required" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		xResolver := &fakeXResolver{err: errors.New("x bad url")}
		server := newTestServerWithXResolver(t, cfg, &fakeResolver{}, xResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/x/resolve", bytes.NewBufferString(`{"url":"https://x.com/x/status/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "x bad url" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if _, ok := payload["code"]; ok {
			t.Fatalf("generic resolver error should not include code, got %#v", payload)
		}
	})

	t.Run("resolver typed error includes code", func(t *testing.T) {
		cfg := baseTestConfig()
		xResolver := &fakeXResolver{err: &xresolver.ResolveError{Code: xresolver.ErrCodeXHLSOnlyNotSupported, Message: "X video is HLS-only and not supported yet"}}
		server := newTestServerWithXResolver(t, cfg, &fakeResolver{}, xResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/x/resolve", bytes.NewBufferString(`{"url":"https://x.com/x/status/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "X video is HLS-only and not supported yet" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if payload["code"] != xresolver.ErrCodeXHLSOnlyNotSupported {
			t.Fatalf("expected code %q, got %#v", xresolver.ErrCodeXHLSOnlyNotSupported, payload["code"])
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := baseTestConfig()
		xResolver := &fakeXResolver{result: xresolver.ResolveResult{Title: "X Video", CookieProfile: "acc-main"}}
		server := newTestServerWithXResolver(t, cfg, &fakeResolver{}, xResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/x/resolve", bytes.NewBufferString(`{"url":"https://x.com/x/status/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["title"] != "X Video" {
			t.Fatalf("unexpected response payload: %#v", payload)
		}
		if payload["cookie_profile"] != "acc-main" {
			t.Fatalf("expected cookie_profile acc-main, got %#v", payload["cookie_profile"])
		}
		if len(xResolver.inputs) != 1 || xResolver.inputs[0] != "https://x.com/x/status/1" {
			t.Fatalf("x resolver should receive input URL, got %#v", xResolver.inputs)
		}
	})
}

func TestHandleResolveInstagram(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithIGResolver(t, cfg, &fakeResolver{}, &fakeIGResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/instagram/resolve", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty url", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithIGResolver(t, cfg, &fakeResolver{}, &fakeIGResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/instagram/resolve", bytes.NewBufferString(`{"url":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "url is required" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		igResolver := &fakeIGResolver{err: errors.New("ig bad url")}
		server := newTestServerWithIGResolver(t, cfg, &fakeResolver{}, igResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/instagram/resolve", bytes.NewBufferString(`{"url":"https://instagram.com/reel/ABC123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "ig bad url" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if _, ok := payload["code"]; ok {
			t.Fatalf("generic resolver error should not include code, got %#v", payload)
		}
	})

	t.Run("resolver typed error includes code", func(t *testing.T) {
		cfg := baseTestConfig()
		igResolver := &fakeIGResolver{err: &igresolver.ResolveError{Code: igresolver.ErrCodeIGHLSOnlyNotSupported, Message: "Instagram video is HLS-only and not supported yet"}}
		server := newTestServerWithIGResolver(t, cfg, &fakeResolver{}, igResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/instagram/resolve", bytes.NewBufferString(`{"url":"https://instagram.com/reel/ABC123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "Instagram video is HLS-only and not supported yet" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if payload["code"] != igresolver.ErrCodeIGHLSOnlyNotSupported {
			t.Fatalf("expected code %q, got %#v", igresolver.ErrCodeIGHLSOnlyNotSupported, payload["code"])
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := baseTestConfig()
		igResolver := &fakeIGResolver{result: igresolver.ResolveResult{Title: "IG Video", CookieProfile: "ig-main"}}
		server := newTestServerWithIGResolver(t, cfg, &fakeResolver{}, igResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/ig/resolve", bytes.NewBufferString(`{"url":"https://instagram.com/reel/ABC123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["title"] != "IG Video" {
			t.Fatalf("unexpected response payload: %#v", payload)
		}
		if payload["cookie_profile"] != "ig-main" {
			t.Fatalf("expected cookie_profile ig-main, got %#v", payload["cookie_profile"])
		}
		if len(igResolver.inputs) != 1 || igResolver.inputs[0] != "https://instagram.com/reel/ABC123" {
			t.Fatalf("ig resolver should receive input URL, got %#v", igResolver.inputs)
		}
	})
}

func TestHandleResolveTikTok(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithTTResolver(t, cfg, &fakeResolver{}, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/tiktok/resolve", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty url", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServerWithTTResolver(t, cfg, &fakeResolver{}, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/tiktok/resolve", bytes.NewBufferString(`{"url":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "url is required" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		ttResolver := &fakeTTResolver{err: errors.New("tt bad url")}
		server := newTestServerWithTTResolver(t, cfg, &fakeResolver{}, ttResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/tiktok/resolve", bytes.NewBufferString(`{"url":"https://www.tiktok.com/@user/video/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "tt bad url" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if _, ok := payload["code"]; ok {
			t.Fatalf("generic resolver error should not include code, got %#v", payload)
		}
	})

	t.Run("resolver typed error includes code", func(t *testing.T) {
		cfg := baseTestConfig()
		ttResolver := &fakeTTResolver{err: &ttresolver.ResolveError{Code: ttresolver.ErrCodeTTHLSOnlyNotSupported, Message: "TikTok video is HLS-only and not supported yet"}}
		server := newTestServerWithTTResolver(t, cfg, &fakeResolver{}, ttResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/tiktok/resolve", bytes.NewBufferString(`{"url":"https://www.tiktok.com/@user/video/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "TikTok video is HLS-only and not supported yet" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
		if payload["code"] != ttresolver.ErrCodeTTHLSOnlyNotSupported {
			t.Fatalf("expected code %q, got %#v", ttresolver.ErrCodeTTHLSOnlyNotSupported, payload["code"])
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := baseTestConfig()
		ttResolver := &fakeTTResolver{result: ttresolver.ResolveResult{Title: "TT Video", CookieProfile: "tt-main"}}
		server := newTestServerWithTTResolver(t, cfg, &fakeResolver{}, ttResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/tt/resolve", bytes.NewBufferString(`{"url":"https://www.tiktok.com/@user/video/1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["title"] != "TT Video" {
			t.Fatalf("unexpected response payload: %#v", payload)
		}
		if payload["cookie_profile"] != "tt-main" {
			t.Fatalf("expected cookie_profile tt-main, got %#v", payload["cookie_profile"])
		}
		if len(ttResolver.inputs) != 1 || ttResolver.inputs[0] != "https://www.tiktok.com/@user/video/1" {
			t.Fatalf("tt resolver should receive input URL, got %#v", ttResolver.inputs)
		}
	})
}

func TestHandleCreateMP3Job(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty url", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"   "}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{err: errors.New("resolve failed")}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("queue task and persist queued record", func(t *testing.T) {
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

		response := decodeJSONMap(t, rec.Body.Bytes())
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
	})

	t.Run("curl input carries headers and user-agent", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Test Video"}}
		queue := &fakeQueue{}
		store := newFakeJobStore()
		server := newTestServer(t, cfg, resolver, queue, store)

		input := `curl "https://www.youtube.com/watch?v=abc123" -H "Referer: https://www.youtube.com/" -A "Mozilla/5.0 Test"`
		bodyMap := map[string]string{"url": input}
		bodyBytes, _ := json.Marshal(bodyMap)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
		}

		var payload queuepkg.ConvertMP3Payload
		if err := json.Unmarshal(queue.enqueueTask.Payload(), &payload); err != nil {
			t.Fatalf("failed to decode queue payload: %v", err)
		}
		if payload.SourceURL != "https://www.youtube.com/watch?v=abc123" {
			t.Fatalf("unexpected source URL: %s", payload.SourceURL)
		}
		if payload.Headers["Referer"] != "https://www.youtube.com/" {
			t.Fatalf("expected referer header in payload, got %#v", payload.Headers)
		}
		if payload.UserAgent != "Mozilla/5.0 Test" {
			t.Fatalf("unexpected user-agent: %s", payload.UserAgent)
		}
	})

	t.Run("put failure", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Test Video"}}
		queue := &fakeQueue{}
		store := newFakeJobStore()
		store.putErr = errors.New("db write failed")
		server := newTestServer(t, cfg, resolver, queue, store)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("queue failure marks job failed", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Test Video"}}
		queue := &fakeQueue{err: errors.New("enqueue failed")}
		store := newFakeJobStore()
		server := newTestServer(t, cfg, resolver, queue, store)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123"}`))
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
	})
}

func TestHandleGetJob(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job_missing", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("internal store error", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		store.getErr = errors.New("db down")
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job_any", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		record := jobs.Record{ID: "job_ok", Status: jobs.StatusDone, OutputKind: "mp3"}
		store.records[record.ID] = record
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/v1/jobs/job_ok", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["id"] != "job_ok" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	})
}

func TestHandleRedirectMP4(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing format_id", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{err: errors.New("resolve failed")}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("selected format unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: ""}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("format not available", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "22", Type: "mp4", URL: "https://cdn.example/22.mp4"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		upstreamBody := []byte("fake-mp4-bytes")
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(upstreamBody)
		}))
		defer upstream.Close()

		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Title: "Video 18", Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: upstream.URL + "/video-18.mp4"}, {ID: "mp3-128", Type: "mp3"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "video/mp4" {
			t.Fatalf("unexpected content type: %s", got)
		}
		if !strings.Contains(rec.Header().Get("Content-Disposition"), `attachment;`) {
			t.Fatalf("expected attachment content-disposition, got %q", rec.Header().Get("Content-Disposition"))
		}
		if body := rec.Body.Bytes(); string(body) != string(upstreamBody) {
			t.Fatalf("unexpected proxied body: %q", string(body))
		}
	})
}

func TestHandleAdminJobs(t *testing.T) {
	t.Run("unauthorized without credentials", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("expected WWW-Authenticate header")
		}
	})

	t.Run("authorized and limit parsed", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		store.listItems = []jobs.Record{{ID: "job_1", Status: jobs.StatusDone, CreatedAt: time.Now().UTC()}}
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/admin/jobs?limit=77", nil)
		req.SetBasicAuth("admin", "secret")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if store.lastListLimit != 77 {
			t.Fatalf("expected limit 77, got %d", store.lastListLimit)
		}

		payload := decodeJSONMap(t, rec.Body.Bytes())
		items, ok := payload["items"].([]any)
		if !ok {
			t.Fatalf("expected items array, got %#v", payload["items"])
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
	})

	t.Run("invalid limit fallback", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		store.listItems = []jobs.Record{}
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/admin/jobs?limit=999", nil)
		req.SetBasicAuth("admin", "secret")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if store.lastListLimit != 30 {
			t.Fatalf("expected default limit 30, got %d", store.lastListLimit)
		}
	})

	t.Run("store error", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		store.listErr = errors.New("db list failed")
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.SetBasicAuth("admin", "secret")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	t.Run("allowed preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/v1/youtube/resolve", nil)
		req.Header.Set("Origin", "http://allowed.local")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.local" {
			t.Fatalf("unexpected allow origin header: %q", got)
		}
	})

	t.Run("disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://blocked.local")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no allow origin header, got %q", got)
		}
	})
}

func TestRateLimitMiddleware_EnforcesLimit(t *testing.T) {
	cfg := baseTestConfig()
	cfg.RateLimitRPS = 1
	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req1.RemoteAddr = "1.2.3.4:1000"
	rec1 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req2.RemoteAddr = "1.2.3.4:1001"
	rec2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request 429, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestNewIPRateLimiter_DisabledWhenLimitNonPositive(t *testing.T) {
	if limiter := newIPRateLimiter(0, 1); limiter != nil {
		t.Fatalf("expected nil limiter when limit <= 0")
	}
}

func TestNewServerAndClose(t *testing.T) {
	cfg := baseTestConfig()
	cfg.RedisAddr = "127.0.0.1:6382"

	server := NewServer(cfg, log.New(io.Discard, "", 0), &fakeResolver{})
	if server == nil {
		t.Fatal("expected non-nil server")
	}

	server.Close()
}
