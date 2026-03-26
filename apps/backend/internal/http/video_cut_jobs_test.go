package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yt-downloader/backend/internal/heatmap"
	"yt-downloader/backend/internal/history"
	queuepkg "yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/youtube"
)

func TestHandleCreateVideoCutJob_FeatureDisabled(t *testing.T) {
	cfg := baseTestConfig()
	cfg.HeatmapTrimEnabled = false

	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc","format_id":"22","cut_mode":"manual","manual":{"start_sec":10,"end_sec":30}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONMap(t, rec.Body.Bytes())
	if payload["code"] != "video_cut_disabled" {
		t.Fatalf("expected code video_cut_disabled, got %+v", payload)
	}
}

func TestHandleCreateVideoCutJob_ValidationErrors(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{
		result: youtube.ResolveResult{
			Title:           "Example",
			DurationSeconds: 120,
			Formats: []youtube.Format{{
				ID:      "22",
				Quality: "720p",
				Type:    "mp4",
				URL:     "https://cdn.example/video.mp4",
			}},
		},
	}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	t.Run("unsupported platform", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", bytes.NewBufferString(`{"url":"https://x.com/i/status/1","format_id":"22","cut_mode":"manual","manual":{"start_sec":10,"end_sec":30}}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "video_cut_platform_not_supported" {
			t.Fatalf("expected platform-not-supported code, got %+v", payload)
		}
	})

	t.Run("format unavailable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc","format_id":"18","cut_mode":"manual","manual":{"start_sec":10,"end_sec":30}}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "format_not_available" {
			t.Fatalf("expected format_not_available code, got %+v", payload)
		}
	})

	t.Run("invalid manual range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc","format_id":"22","cut_mode":"manual","manual":{"start_sec":80,"end_sec":30}}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "invalid_trim_range" {
			t.Fatalf("expected invalid_trim_range code, got %+v", payload)
		}
	})

	t.Run("heatmap unavailable", func(t *testing.T) {
		resolver.result.HeatmapMeta = heatmap.Meta{Available: false, Bins: 0, AlgorithmVersion: heatmap.AlgorithmVersion}
		resolver.result.KeyMoments = nil

		req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc","format_id":"22","cut_mode":"heatmap"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "invalid_trim_range" {
			t.Fatalf("expected invalid_trim_range code, got %+v", payload)
		}
	})
}

func TestHandleCreateVideoCutJob_SuccessQueuesTaskAndHistory(t *testing.T) {
	cfg := baseTestConfig()
	cfg.VideoCutMaxDurationSec = 180

	resolver := &fakeResolver{
		result: youtube.ResolveResult{
			Title:           "Video Cut Example",
			Thumbnail:       "https://img.example.com/thumb.jpg",
			DurationSeconds: 120,
			Formats: []youtube.Format{
				{ID: "22", Quality: "720p", Type: "mp4", URL: "https://cdn.example.com/22.mp4"},
				{ID: "mp3-128", Quality: "audio", Type: "mp3"},
			},
			HeatmapMeta: heatmap.Meta{Available: true, Bins: 10, AlgorithmVersion: heatmap.AlgorithmVersion},
			KeyMoments:  []int{60},
		},
	}
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, resolver, queue, store)

	token, userID := registerUserAndGetToken(t, server)
	if strings.TrimSpace(token) == "" || strings.TrimSpace(userID) == "" {
		t.Fatalf("expected authenticated user")
	}

	body := bytes.NewBufferString(`{"url":"https://www.youtube.com/watch?v=abc123","format_id":"22","cut_mode":"manual","manual":{"start_sec":12,"end_sec":48}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/video-cut", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	responsePayload := decodeJSONMap(t, rec.Body.Bytes())
	jobID, _ := responsePayload["job_id"].(string)
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("unexpected job_id: %+v", responsePayload)
	}

	record, ok := store.records[jobID]
	if !ok {
		t.Fatalf("expected job record %s to be stored", jobID)
	}
	if record.Status != "queued" {
		t.Fatalf("expected queued record status, got %s", record.Status)
	}
	if record.OutputKind != "video_cut" {
		t.Fatalf("expected output_kind video_cut, got %s", record.OutputKind)
	}
	if !strings.Contains(record.OutputKey, "video-cut/") {
		t.Fatalf("expected video-cut output key, got %s", record.OutputKey)
	}

	if queue.enqueueTask == nil {
		t.Fatalf("expected queue task to be enqueued")
	}
	if queue.enqueueTask.Type() != queuepkg.TaskVideoCut {
		t.Fatalf("expected task type %s, got %s", queuepkg.TaskVideoCut, queue.enqueueTask.Type())
	}

	var queuePayload queuepkg.VideoCutPayload
	if err := json.Unmarshal(queue.enqueueTask.Payload(), &queuePayload); err != nil {
		t.Fatalf("decode queue payload: %v", err)
	}
	if queuePayload.JobID != jobID {
		t.Fatalf("unexpected queue payload job id, got=%s want=%s", queuePayload.JobID, jobID)
	}
	if queuePayload.CutMode != queuepkg.VideoCutModeManual {
		t.Fatalf("expected manual cut mode, got %s", queuePayload.CutMode)
	}
	if queuePayload.ManualStartSec != 12 || queuePayload.ManualEndSec != 48 {
		t.Fatalf("unexpected manual range in payload: %+v", queuePayload)
	}
	if queuePayload.FormatID != "22" {
		t.Fatalf("unexpected format id in queue payload: %s", queuePayload.FormatID)
	}

	attempt, err := server.historyStore.GetAttemptByJobID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("expected history attempt by job id, got err=%v", err)
	}
	if attempt.UserID != userID {
		t.Fatalf("unexpected history user id, got=%s want=%s", attempt.UserID, userID)
	}
	if attempt.RequestKind != history.RequestKindMP4 {
		t.Fatalf("expected request kind mp4, got %s", attempt.RequestKind)
	}
	if attempt.FormatID != "22" {
		t.Fatalf("unexpected history format id: %s", attempt.FormatID)
	}
	if !strings.Contains(strings.ToLower(attempt.QualityLabel), "manual") {
		t.Fatalf("expected quality label to include manual mode, got %s", attempt.QualityLabel)
	}
}

func TestComputeVideoCutPlan_HeatmapWindowDerivation(t *testing.T) {
	cfg := baseTestConfig()
	cfg.VideoCutMaxDurationSec = 120
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	plan, err := server.computeVideoCutPlan(
		createVideoCutJobRequest{
			URL:      "https://www.youtube.com/watch?v=abc",
			FormatID: "22",
			CutMode:  "heatmap",
			Heatmap: &videoCutHeatmapRequest{
				TargetSec: 90,
				WindowSec: 40,
			},
		},
		youtube.ResolveResult{
			DurationSeconds: 100,
			HeatmapMeta:     heatmap.Meta{Available: true, Bins: 10, AlgorithmVersion: heatmap.AlgorithmVersion},
			KeyMoments:      []int{75},
		},
	)
	if err != nil {
		t.Fatalf("expected plan success, got err=%v", err)
	}
	if plan.Mode != videoCutModeHeatmap {
		t.Fatalf("unexpected mode: %s", plan.Mode)
	}
	if plan.ManualStartSec >= plan.ManualEndSec {
		t.Fatalf("expected valid manual range from heatmap, got %+v", plan)
	}
	if plan.ManualEndSec > 100 {
		t.Fatalf("expected derived end <= duration, got %+v", plan)
	}
}
