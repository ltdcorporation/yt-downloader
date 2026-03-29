package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"yt-downloader/backend/internal/igresolver"
	"yt-downloader/backend/internal/xresolver"
	"yt-downloader/backend/internal/youtube"
)

type failWriteResponseWriter struct {
	header  http.Header
	status  int
	writes  int
	lastErr error
}

func (w *failWriteResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failWriteResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *failWriteResponseWriter) Write(_ []byte) (int, error) {
	w.writes++
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.lastErr = errors.New("forced write error")
	return 0, w.lastErr
}

func TestHandleCreateMP3Job_PlatformCoverage(t *testing.T) {
	t.Run("unsupported platform", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://example.com/video.mp4"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsupported platform, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "unsupported platform" {
			t.Fatalf("unexpected error payload: %#v", payload)
		}
	})

	t.Run("x platform success with invalid optional session token", func(t *testing.T) {
		cfg := baseTestConfig()
		xResolver := &fakeXResolver{result: xresolver.ResolveResult{Title: "X Clip", Thumbnail: "https://img.example.com/x-thumb.jpg"}}
		queue := &fakeQueue{}
		store := newFakeJobStore()
		server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, xResolver, &fakeIGResolver{}, &fakeTTResolver{}, queue, store)

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://x.com/user/status/123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer st_invalid")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 for x mp3 enqueue, got %d body=%s", rec.Code, rec.Body.String())
		}
		if len(xResolver.inputs) != 1 || xResolver.inputs[0] != "https://x.com/user/status/123" {
			t.Fatalf("x resolver should receive source URL once, got %#v", xResolver.inputs)
		}
		if len(store.records) != 1 {
			t.Fatalf("expected exactly one queued record, got %d", len(store.records))
		}
		for _, record := range store.records {
			if record.Title != "X Clip" {
				t.Fatalf("expected x title propagated to job record, got %q", record.Title)
			}
			if record.OutputKind != "mp3" {
				t.Fatalf("expected mp3 output kind, got %q", record.OutputKind)
			}
		}
	})

	t.Run("instagram resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		igResolver := &fakeIGResolver{err: errors.New("ig resolve failed")}
		server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, &fakeXResolver{}, igResolver, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.instagram.com/reel/abc/"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for instagram resolver error, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tiktok resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		ttResolver := &fakeTTResolver{err: errors.New("tt resolve failed")}
		server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, &fakeXResolver{}, &fakeIGResolver{}, ttResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.tiktok.com/@user/video/123"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for tiktok resolver error, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleRedirectMP4_AdditionalCoverage(t *testing.T) {
	t.Run("unsupported platform", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://example.com/video.mp4&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unsupported platform, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tiktok resolver error", func(t *testing.T) {
		cfg := baseTestConfig()
		ttResolver := &fakeTTResolver{err: errors.New("tiktok resolve failed")}
		server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, &fakeXResolver{}, &fakeIGResolver{}, ttResolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.tiktok.com/@u/video/1&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for tiktok resolver error, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("x image media success path", func(t *testing.T) {
		imageBody := []byte("image-binary-payload")
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imageBody)
		}))
		defer upstream.Close()

		cfg := baseTestConfig()
		xResolver := &fakeXResolver{result: xresolver.ResolveResult{
			Title: "X image post",
			Medias: []xresolver.Media{
				{ID: "img_1", URL: upstream.URL + "/image.jpg", Type: "image", Quality: "original", Thumbnail: "https://img.example.com/thumb.jpg"},
			},
		}}
		server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, xResolver, &fakeIGResolver{}, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://x.com/user/status/1&format_id=img_1", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for x image download, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("expected image/jpeg content type, got %q", got)
		}
		if !bytes.Equal(rec.Body.Bytes(), imageBody) {
			t.Fatalf("unexpected image response body %q", rec.Body.String())
		}
	})

	t.Run("upstream request creation failure", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: "://bad-url"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for upstream request creation failure, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("upstream fetch failure", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		upstreamURL := upstream.URL
		upstream.Close()

		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: upstreamURL + "/video.mp4"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 for upstream fetch failure, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("upstream non-200 status", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("blocked"))
		}))
		defer upstream.Close()

		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: upstream.URL + "/video.mp4"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 for upstream non-200 status, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "source returned status 403" {
			t.Fatalf("unexpected non-200 upstream error payload: %#v", payload)
		}
	})

	t.Run("copy failure branch via direct handler call", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("stream-data"))
		}))
		defer upstream.Close()

		cfg := baseTestConfig()
		resolver := &fakeResolver{result: youtube.ResolveResult{Formats: []youtube.Format{{ID: "18", Type: "mp4", URL: upstream.URL + "/video.mp4"}}}}
		server := newTestServer(t, cfg, resolver, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.youtube.com/watch?v=abc123&format_id=18", nil)
		writer := &failWriteResponseWriter{}
		server.handleRedirectMP4(writer, req)

		if writer.writes == 0 {
			t.Fatalf("expected write attempts during stream copy, got none")
		}
		if writer.lastErr == nil {
			t.Fatalf("expected forced write error to be captured")
		}
	})
}

func TestDetectPlatform_TableCoverage(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "youtube", url: "https://www.youtube.com/watch?v=abc123", want: "youtube"},
		{name: "x", url: "https://x.com/u/status/1", want: "x"},
		{name: "instagram", url: "https://www.instagram.com/reel/abc/", want: "instagram"},
		{name: "tiktok", url: "https://www.tiktok.com/@u/video/1", want: "tiktok"},
		{name: "invalid", url: "not a url", want: "unknown"},
		{name: "unsupported", url: "https://example.com/video", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := server.detectPlatform(tt.url); got != tt.want {
				t.Fatalf("unexpected platform detection for %s: got=%s want=%s", tt.url, got, tt.want)
			}
		})
	}
}

func TestHandleRedirectMP4_InstagramResolverError(t *testing.T) {
	cfg := baseTestConfig()
	igResolver := &fakeIGResolver{err: errors.New("ig resolve failed")}
	server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, &fakeXResolver{}, igResolver, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://www.instagram.com/reel/abc/&format_id=18", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for instagram resolver error, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleRedirectMP4_XResolverError(t *testing.T) {
	cfg := baseTestConfig()
	xResolver := &fakeXResolver{err: errors.New("x resolve failed")}
	server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, xResolver, &fakeIGResolver{}, &fakeTTResolver{}, &fakeQueue{}, newFakeJobStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/download/mp4?url=https://x.com/u/status/abc&format_id=18", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for x resolver error, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateMP3Job_InstagramSuccessUsesResolverResult(t *testing.T) {
	cfg := baseTestConfig()
	igResolver := &fakeIGResolver{result: igresolver.ResolveResult{Title: "IG Clip", Thumbnail: "https://img.example.com/ig-thumb.jpg"}}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServerWithResolvers(t, cfg, &fakeResolver{}, &fakeXResolver{}, igResolver, &fakeTTResolver{}, queue, store)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/mp3", bytes.NewBufferString(`{"url":"https://www.instagram.com/reel/abc/"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for instagram mp3 enqueue, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(igResolver.inputs) != 1 {
		t.Fatalf("expected instagram resolver to be called once, got %d", len(igResolver.inputs))
	}
	if len(store.records) != 1 {
		t.Fatalf("expected one queued record, got %d", len(store.records))
	}
	for _, record := range store.records {
		if record.OutputKind != "mp3" {
			t.Fatalf("expected mp3 output kind, got %s", record.OutputKind)
		}
		if record.Title != "IG Clip" {
			t.Fatalf("expected IG title propagated, got %s", record.Title)
		}
	}
}
