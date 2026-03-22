package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/history"
)

func TestHandleHistory_AuthRequired(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONMap(t, rec.Body.Bytes())
	if payload["code"] != "invalid_session" {
		t.Fatalf("expected invalid_session code, got %#v", payload["code"])
	}
}

func TestHandleHistory_ListStatsDeleteFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	now := time.Now().UTC().Truncate(time.Microsecond)
	item, err := server.historyStore.UpsertItem(context.Background(), history.Item{
		ID:            "his_test_flow_1",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=abc123",
		SourceURLHash: "hash_history_flow_1",
		Title:         "History Flow Video",
		ThumbnailURL:  "https://img.example.com/flow.jpg",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	size := int64(2048)
	_, err = server.historyStore.CreateAttempt(context.Background(), history.Attempt{
		ID:            "hat_test_flow_1",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP3,
		Status:        history.StatusDone,
		QualityLabel:  "192kbps",
		SizeBytes:     &size,
		DownloadURL:   "https://signed.example.com/audio.mp3",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/history?limit=10", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for history list, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	listPayload := decodeJSONMap(t, listRec.Body.Bytes())
	items, ok := listPayload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 history item, got %#v", listPayload["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected history item object, got %#v", items[0])
	}
	if first["id"] != item.ID {
		t.Fatalf("unexpected history item id: %#v", first["id"])
	}
	latestAttempt, ok := first["latest_attempt"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest_attempt object, got %#v", first["latest_attempt"])
	}
	if latestAttempt["status"] != string(history.StatusDone) {
		t.Fatalf("unexpected latest attempt status: %#v", latestAttempt["status"])
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/v1/history/stats", nil)
	statsReq.Header.Set("Authorization", "Bearer "+token)
	statsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(statsRec, statsReq)

	if statsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for history stats, got %d body=%s", statsRec.Code, statsRec.Body.String())
	}
	statsPayload := decodeJSONMap(t, statsRec.Body.Bytes())
	if statsPayload["total_items"] != float64(1) {
		t.Fatalf("unexpected total_items: %#v", statsPayload["total_items"])
	}
	if statsPayload["total_attempts"] != float64(1) {
		t.Fatalf("unexpected total_attempts: %#v", statsPayload["total_attempts"])
	}
	if statsPayload["success_count"] != float64(1) {
		t.Fatalf("unexpected success_count: %#v", statsPayload["success_count"])
	}
	if statsPayload["total_bytes_downloaded"] != float64(size) {
		t.Fatalf("unexpected total_bytes_downloaded: %#v", statsPayload["total_bytes_downloaded"])
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/history/"+item.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for history delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	listAfterDeleteReq.Header.Set("Authorization", "Bearer "+token)
	listAfterDeleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)

	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for history list after delete, got %d body=%s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}
	listAfterDeletePayload := decodeJSONMap(t, listAfterDeleteRec.Body.Bytes())
	itemsAfterDelete, ok := listAfterDeletePayload["items"].([]any)
	if !ok {
		t.Fatalf("expected items array after delete, got %#v", listAfterDeletePayload["items"])
	}
	if len(itemsAfterDelete) != 0 {
		t.Fatalf("expected no items after delete, got %#v", itemsAfterDelete)
	}
}

func TestHandleHistoryList_CursorPagination(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item1, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_cursor_1",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=first",
		SourceURLHash: "hash_cursor_1",
		Title:         "Cursor First",
		LastAttemptAt: ptrTimeTest(now.Add(-time.Minute)),
		AttemptCount:  1,
		CreatedAt:     now.Add(-time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected item1 upsert error: %v", err)
	}
	item2, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_cursor_2",
		UserID:        userID,
		Platform:      history.PlatformTikTok,
		SourceURL:     "https://www.tiktok.com/@u/video/2",
		SourceURLHash: "hash_cursor_2",
		Title:         "Cursor Second",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item2 upsert error: %v", err)
	}
	_, _ = server.historyStore.CreateAttempt(ctx, history.Attempt{ID: "hat_cursor_1", HistoryItemID: item1.ID, UserID: userID, RequestKind: history.RequestKindMP3, Status: history.StatusDone, CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)})
	_, _ = server.historyStore.CreateAttempt(ctx, history.Attempt{ID: "hat_cursor_2", HistoryItemID: item2.ID, UserID: userID, RequestKind: history.RequestKindMP4, Status: history.StatusDone, CreatedAt: now, UpdatedAt: now})

	firstReq := httptest.NewRequest(http.MethodGet, "/v1/history?limit=1", nil)
	firstReq.Header.Set("Authorization", "Bearer "+token)
	firstRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected 200 first cursor page, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}
	firstPayload := decodeJSONMap(t, firstRec.Body.Bytes())
	firstItems := firstPayload["items"].([]any)
	firstItem := firstItems[0].(map[string]any)
	if firstItem["id"] != item2.ID {
		t.Fatalf("expected newest item first, got %#v", firstItem["id"])
	}
	pageObj := firstPayload["page"].(map[string]any)
	nextCursor, _ := pageObj["next_cursor"].(string)
	if strings.TrimSpace(nextCursor) == "" {
		t.Fatalf("expected non-empty next_cursor on first page")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v1/history?limit=10&cursor="+url.QueryEscape(nextCursor), nil)
	secondReq.Header.Set("Authorization", "Bearer "+token)
	secondRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected 200 second cursor page, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
	secondPayload := decodeJSONMap(t, secondRec.Body.Bytes())
	secondItems := secondPayload["items"].([]any)
	if len(secondItems) != 1 {
		t.Fatalf("expected one item in second page, got %#v", secondItems)
	}
	secondItem := secondItems[0].(map[string]any)
	if secondItem["id"] != item1.ID {
		t.Fatalf("expected oldest item second, got %#v", secondItem["id"])
	}
}

func TestHandleHistoryRedownload_MP4Direct(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_redownload_mp4",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=mp4",
		SourceURLHash: "hash_redownload_mp4",
		Title:         "MP4 Sample",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item upsert error: %v", err)
	}
	_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_redownload_mp4",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusDone,
		FormatID:      "18",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected attempt create error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for mp4 redownload, got %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONMap(t, rec.Body.Bytes())
	if payload["mode"] != "direct" {
		t.Fatalf("expected direct mode, got %#v", payload["mode"])
	}
	downloadURL, _ := payload["download_url"].(string)
	if !strings.Contains(downloadURL, "/api/v1/download/mp4?") || !strings.Contains(downloadURL, "format_id=18") {
		t.Fatalf("unexpected direct download url: %s", downloadURL)
	}
}

func TestHandleHistoryRedownload_MP3DirectAndQueued(t *testing.T) {
	t.Run("direct_when_unexpired", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, userID := registerUserAndGetToken(t, server)
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		expires := now.Add(30 * time.Minute)

		item, err := server.historyStore.UpsertItem(ctx, history.Item{
			ID:            "his_redownload_mp3_direct",
			UserID:        userID,
			Platform:      history.PlatformYouTube,
			SourceURL:     "https://www.youtube.com/watch?v=mp3direct",
			SourceURLHash: "hash_redownload_mp3_direct",
			Title:         "MP3 Direct",
			LastAttemptAt: ptrTimeTest(now),
			AttemptCount:  1,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			t.Fatalf("unexpected item upsert error: %v", err)
		}
		_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
			ID:            "hat_redownload_mp3_direct",
			HistoryItemID: item.ID,
			UserID:        userID,
			RequestKind:   history.RequestKindMP3,
			Status:        history.StatusDone,
			DownloadURL:   "https://signed.example.com/direct.mp3",
			ExpiresAt:     &expires,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			t.Fatalf("unexpected attempt create error: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["mode"] != "direct" {
			t.Fatalf("expected direct mode, got %#v", payload["mode"])
		}
		if payload["download_url"] != "https://signed.example.com/direct.mp3" {
			t.Fatalf("unexpected direct download url: %#v", payload["download_url"])
		}
	})

	t.Run("queued_when_expired", func(t *testing.T) {
		cfg := baseTestConfig()
		resolver := &fakeResolver{}
		queue := &fakeQueue{}
		jobStore := newFakeJobStore()
		server := newTestServer(t, cfg, resolver, queue, jobStore)
		token, userID := registerUserAndGetToken(t, server)
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		expires := now.Add(-30 * time.Minute)

		item, err := server.historyStore.UpsertItem(ctx, history.Item{
			ID:            "his_redownload_mp3_queue",
			UserID:        userID,
			Platform:      history.PlatformYouTube,
			SourceURL:     "https://www.youtube.com/watch?v=mp3queue",
			SourceURLHash: "hash_redownload_mp3_queue",
			Title:         "MP3 Queue",
			ThumbnailURL:  "https://img.example.com/queue.jpg",
			LastAttemptAt: ptrTimeTest(now),
			AttemptCount:  1,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			t.Fatalf("unexpected item upsert error: %v", err)
		}
		_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
			ID:            "hat_redownload_mp3_queue",
			HistoryItemID: item.ID,
			UserID:        userID,
			RequestKind:   history.RequestKindMP3,
			Status:        history.StatusDone,
			DownloadURL:   "https://signed.example.com/expired.mp3",
			ExpiresAt:     &expires,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			t.Fatalf("unexpected attempt create error: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", bytes.NewBufferString(`{"request_kind":"mp3"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["mode"] != "queued" {
			t.Fatalf("expected queued mode, got %#v", payload["mode"])
		}
		jobID, _ := payload["job_id"].(string)
		if strings.TrimSpace(jobID) == "" {
			t.Fatalf("missing queued job id in payload: %s", rec.Body.String())
		}

		if queue.enqueueTask == nil {
			t.Fatalf("expected queue enqueue task for redownload")
		}
		var mp3Payload map[string]any
		if err := json.Unmarshal(queue.enqueueTask.Payload(), &mp3Payload); err != nil {
			t.Fatalf("failed to decode enqueued payload: %v", err)
		}
		if mp3Payload["job_id"] != jobID {
			t.Fatalf("queue payload job id mismatch, payload=%#v api=%s", mp3Payload["job_id"], jobID)
		}

		historyAttempt, err := server.historyStore.GetAttemptByJobID(ctx, jobID)
		if err != nil {
			t.Fatalf("expected history attempt for queued redownload job, err=%v", err)
		}
		if historyAttempt.Status != history.StatusQueued {
			t.Fatalf("expected queued history status, got %s", historyAttempt.Status)
		}
	})
}

func ptrTimeTest(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}
