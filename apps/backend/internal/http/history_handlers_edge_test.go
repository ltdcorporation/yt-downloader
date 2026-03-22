package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/history"
)

func TestHistoryHelpers_CodecAndValidation(t *testing.T) {
	t.Run("cursor encode decode roundtrip", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		encoded := encodeHistoryCursor(&history.ListCursor{SortAt: now, ItemID: "his_1"})
		if strings.TrimSpace(encoded) == "" {
			t.Fatalf("expected encoded cursor")
		}

		decoded, err := decodeHistoryCursor(encoded)
		if err != nil {
			t.Fatalf("unexpected decode error: %v", err)
		}
		if !decoded.SortAt.Equal(now) || decoded.ItemID != "his_1" {
			t.Fatalf("unexpected decoded cursor: %+v", decoded)
		}
	})

	t.Run("cursor decode errors", func(t *testing.T) {
		if _, err := decodeHistoryCursor("%%%bad"); err == nil {
			t.Fatalf("expected error for invalid base64")
		}

		badJSON := base64.RawURLEncoding.EncodeToString([]byte("{bad"))
		if _, err := decodeHistoryCursor(badJSON); err == nil {
			t.Fatalf("expected error for invalid cursor JSON")
		}

		invalidTimePayload, _ := json.Marshal(historyCursorPayload{SortAt: "not-time", ItemID: "his_1"})
		invalidTimeCursor := base64.RawURLEncoding.EncodeToString(invalidTimePayload)
		if _, err := decodeHistoryCursor(invalidTimeCursor); err == nil {
			t.Fatalf("expected error for invalid cursor time")
		}

		emptyItemPayload, _ := json.Marshal(historyCursorPayload{SortAt: time.Now().UTC().Format(time.RFC3339Nano), ItemID: ""})
		emptyItemCursor := base64.RawURLEncoding.EncodeToString(emptyItemPayload)
		if _, err := decodeHistoryCursor(emptyItemCursor); err == nil {
			t.Fatalf("expected error for empty cursor item id")
		}
	})

	t.Run("encode nil cursor", func(t *testing.T) {
		if got := encodeHistoryCursor(nil); got != "" {
			t.Fatalf("expected empty encoded cursor, got %q", got)
		}
	})

	t.Run("decode redownload request variants", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		decoded, err := decodeHistoryRedownloadRequest(req)
		if err != nil {
			t.Fatalf("unexpected decode error on empty body: %v", err)
		}
		if decoded.RequestKind != "" || decoded.FormatID != "" {
			t.Fatalf("expected empty request on empty body, got %+v", decoded)
		}

		req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"request_kind":"mp4","format_id":"18"}`))
		decoded, err = decodeHistoryRedownloadRequest(req)
		if err != nil {
			t.Fatalf("unexpected decode error on valid body: %v", err)
		}
		if decoded.RequestKind != "mp4" || decoded.FormatID != "18" {
			t.Fatalf("unexpected decoded request: %+v", decoded)
		}

		req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"request_kind":1}`))
		if _, err := decodeHistoryRedownloadRequest(req); err == nil {
			t.Fatalf("expected decode error for invalid request_kind type")
		}

		req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"unknown":true}`))
		if _, err := decodeHistoryRedownloadRequest(req); err == nil {
			t.Fatalf("expected decode error for unknown field")
		}

		req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"request_kind":"mp3"}{"request_kind":"mp4"}`))
		if _, err := decodeHistoryRedownloadRequest(req); err == nil {
			t.Fatalf("expected decode error for multiple JSON objects")
		}

		req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"request_kind":"mp3"}x`))
		if _, err := decodeHistoryRedownloadRequest(req); err == nil {
			t.Fatalf("expected decode error for trailing invalid JSON token")
		}
	})

	t.Run("resolve redownload kind", func(t *testing.T) {
		kind, err := resolveRedownloadKind("", history.RequestKindMP3)
		if err != nil || kind != history.RequestKindMP3 {
			t.Fatalf("expected fallback request kind mp3, got kind=%s err=%v", kind, err)
		}

		kind, err = resolveRedownloadKind("MP4", history.RequestKindMP3)
		if err != nil || kind != history.RequestKindMP4 {
			t.Fatalf("expected explicit request kind mp4, got kind=%s err=%v", kind, err)
		}

		if _, err := resolveRedownloadKind("", ""); err == nil {
			t.Fatalf("expected error for empty request kind without fallback")
		}
		if _, err := resolveRedownloadKind("archive", history.RequestKindMP3); err == nil {
			t.Fatalf("expected error for unsupported request kind")
		}
	})

	t.Run("status filter helper and history error writer", func(t *testing.T) {
		for _, status := range []history.AttemptStatus{
			history.StatusQueued,
			history.StatusProcessing,
			history.StatusDone,
			history.StatusFailed,
			history.StatusExpired,
		} {
			if !isHistoryStatusFilterSupported(status) {
				t.Fatalf("expected status %s to be supported", status)
			}
		}
		if isHistoryStatusFilterSupported(history.AttemptStatus("custom")) {
			t.Fatalf("expected custom status to be unsupported")
		}

		rec := httptest.NewRecorder()
		writeHistoryError(rec, http.StatusBadRequest, "bad request", " history_invalid_request ")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status code: %d", rec.Code)
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["error"] != "bad request" || payload["code"] != "history_invalid_request" {
			t.Fatalf("unexpected payload with code: %#v", payload)
		}

		rec = httptest.NewRecorder()
		writeHistoryError(rec, http.StatusBadRequest, "bad request", "   ")
		payload = decodeJSONMap(t, rec.Body.Bytes())
		if _, hasCode := payload["code"]; hasCode {
			t.Fatalf("expected no code field when input code is blank: %#v", payload)
		}
	})
}

func TestHandleHistory_ServiceAndValidationErrors(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	t.Run("history list unavailable when store nil", func(t *testing.T) {
		server.historyStore = nil
		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_unavailable" {
			t.Fatalf("unexpected code: %#v", payload["code"])
		}
	})

	// restore server with store for next tests
	server = newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ = registerUserAndGetToken(t, server)

	t.Run("history list with invalid query params", func(t *testing.T) {
		cases := []struct {
			name string
			url  string
			code string
		}{
			{name: "invalid limit text", url: "/v1/history?limit=abc", code: "history_invalid_request"},
			{name: "invalid limit zero", url: "/v1/history?limit=0", code: "history_invalid_request"},
			{name: "invalid platform", url: "/v1/history?platform=soundcloud", code: "history_invalid_request"},
			{name: "invalid status", url: "/v1/history?status=unknown", code: "history_invalid_request"},
			{name: "invalid cursor", url: "/v1/history?cursor=not-base64!", code: "history_invalid_cursor"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tc.url, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
				}
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != tc.code {
					t.Fatalf("unexpected error code for %s: %#v", tc.name, payload["code"])
				}
			})
		}
	})

	t.Run("history list clamps limit > max", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/history?limit=999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		page, ok := payload["page"].(map[string]any)
		if !ok {
			t.Fatalf("expected page object, got %#v", payload["page"])
		}
		if page["limit"] != float64(history.MaxListLimit) {
			t.Fatalf("expected clamped limit %d, got %#v", history.MaxListLimit, page["limit"])
		}
	})

	t.Run("history list accepts valid platform and status filters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/history?platform=youtube&status=done&limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if _, ok := payload["items"].([]any); !ok {
			t.Fatalf("expected items array, got %#v", payload["items"])
		}
	})

	t.Run("require session identity fails when auth service nil", func(t *testing.T) {
		server.authService = nil
		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when auth service nil, got %d", rec.Code)
		}
	})
}

func TestHandleHistoryDeleteAndRedownloadErrorBranches(t *testing.T) {
	cfg := baseTestConfig()
	queue := &fakeQueue{}
	server := newTestServer(t, cfg, &fakeResolver{}, queue, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_edge_1",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=edge",
		SourceURLHash: "hash_edge_1",
		Title:         "Edge Case Item",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item upsert error: %v", err)
	}

	t.Run("delete missing id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/history/%20", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/history/not-found", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload missing id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/%20/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload item not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/unknown/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload item without attempts returns conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_edge_1",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusDone,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	t.Run("redownload invalid json body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", bytes.NewBufferString("{"))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload invalid request kind", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", bytes.NewBufferString(`{"request_kind":"archive"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload mp4 without format id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+item.ID+"/redownload", bytes.NewBufferString(`{"request_kind":"mp4","format_id":""}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	// add an mp3 attempt with expired URL so redownload will attempt queueing
	itemMP3, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_edge_mp3",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=edge-mp3",
		SourceURLHash: "hash_edge_mp3",
		Title:         "Edge MP3",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item mp3 upsert error: %v", err)
	}
	expired := now.Add(-time.Minute)
	_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_edge_mp3",
		HistoryItemID: itemMP3.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP3,
		Status:        history.StatusDone,
		DownloadURL:   "https://signed.example.com/expired.mp3",
		ExpiresAt:     &expired,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected create mp3 attempt error: %v", err)
	}

	queue.err = fmt.Errorf("queue down")
	t.Run("redownload mp3 queue failure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/history/"+itemMP3.ID+"/redownload", bytes.NewBufferString(`{"request_kind":"mp3"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_unavailable" {
			t.Fatalf("unexpected code for queue failure: %#v", payload["code"])
		}
	})
}

func TestRequireSessionIdentityAndOptionalIdentityBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	t.Run("requireSessionIdentity success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		identity, ok := server.requireSessionIdentity(rec, req)
		if !ok || identity == nil {
			t.Fatalf("expected requireSessionIdentity to succeed")
		}
		if identity.User.ID != userID {
			t.Fatalf("unexpected identity user: %+v", identity.User)
		}
	})

	t.Run("requireSessionIdentity service unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)

		backup := server.authService
		server.authService = nil
		defer func() { server.authService = backup }()

		identity, ok := server.requireSessionIdentity(rec, req)
		if ok || identity != nil {
			t.Fatalf("expected requireSessionIdentity to fail when auth service is nil")
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("optionalSessionIdentity branches", func(t *testing.T) {
		reqNoToken := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		if identity := server.optionalSessionIdentity(reqNoToken); identity != nil {
			t.Fatalf("expected nil identity without token")
		}
		if identity := (*Server)(nil).optionalSessionIdentity(reqNoToken); identity != nil {
			t.Fatalf("expected nil identity on nil server")
		}

		reqValid := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		reqValid.Header.Set("Authorization", "Bearer "+token)
		identity := server.optionalSessionIdentity(reqValid)
		if identity == nil || identity.User.ID != userID {
			t.Fatalf("expected valid optional session identity, got %+v", identity)
		}

		reqInvalid := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		reqInvalid.Header.Set("Authorization", "Bearer invalid-token")
		if identity := server.optionalSessionIdentity(reqInvalid); identity != nil {
			t.Fatalf("expected nil identity on invalid session token")
		}

		backup := server.authService
		server.authService = &auth.Service{}
		if identity := server.optionalSessionIdentity(reqValid); identity != nil {
			t.Fatalf("expected nil identity when auth service returns internal error")
		}
		server.authService = backup
	})
}

func TestHistoryWriteHelpersAndPlatformMap(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_write_helper",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=helper",
		SourceURLHash: "hash_write_helper",
		Title:         "Write Helper",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error: %v", err)
	}

	attempt, err := server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_write_helper",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusProcessing,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected create attempt error: %v", err)
	}

	server.markHistoryAttemptDone(&attempt, nil)
	updated, err := server.historyStore.GetAttemptByID(ctx, userID, attempt.ID)
	if err != nil {
		t.Fatalf("unexpected read updated attempt error: %v", err)
	}
	if updated.Status != history.StatusDone {
		t.Fatalf("expected attempt status done, got %s", updated.Status)
	}

	if err := server.historyStore.SoftDeleteItem(ctx, userID, item.ID, time.Now().UTC()); err != nil {
		t.Fatalf("unexpected soft delete error: %v", err)
	}
	server.markHistoryAttemptDone(&updated, nil) // mark item success path should tolerate deleted item

	server.markHistoryAttemptFailed(&history.Attempt{ID: "missing", UserID: userID}, "failed_code", io.EOF)
	server.markHistoryAttemptFailed(nil, "failed_code", io.EOF)
	server.markHistoryAttemptDone(nil, nil)

	if got := clipHistoryError(nil); got != "" {
		t.Fatalf("expected empty clipped error for nil, got %q", got)
	}
	longErr := strings.Repeat("x", 500)
	if got := clipHistoryError(errors.New(longErr)); len(got) != 400 {
		t.Fatalf("expected clipped error length 400, got %d", len(got))
	}

	if got := firstNonEmpty("", "   ", "abc", "def"); got != "abc" {
		t.Fatalf("unexpected firstNonEmpty result: %q", got)
	}
	if got := firstNonEmpty("", "   "); got != "" {
		t.Fatalf("expected empty firstNonEmpty result, got %q", got)
	}

	for input, expected := range map[string]history.Platform{
		"youtube":   history.PlatformYouTube,
		"tiktok":    history.PlatformTikTok,
		"instagram": history.PlatformInstagram,
		"x":         history.PlatformX,
	} {
		platform, ok := toHistoryPlatform(input)
		if !ok || platform != expected {
			t.Fatalf("unexpected platform mapping input=%s platform=%s ok=%v", input, platform, ok)
		}
	}
	if _, ok := toHistoryPlatform("vimeo"); ok {
		t.Fatalf("expected unsupported platform mapping for vimeo")
	}

	cursor := &history.ListCursor{SortAt: now, ItemID: "his_cursor_helper"}
	encoded := encodeHistoryCursor(cursor)
	if strings.TrimSpace(encoded) == "" {
		t.Fatalf("expected encoded cursor helper")
	}
	decoded, err := decodeHistoryCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected decode cursor helper error: %v", err)
	}
	if decoded.ItemID != cursor.ItemID {
		t.Fatalf("unexpected decoded cursor helper item id: %+v", decoded)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/history/"+url.PathEscape(item.ID)+"/redownload", bytes.NewBufferString(`{"request_kind":"mp4","format_id":"18"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected non-2xx response for deleted item redownload edge, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleHistoryListAndStatsInternalErrorPaths(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	backupStore := server.historyStore
	server.historyStore = &history.Store{}
	defer func() { server.historyStore = backupStore }()

	t.Run("list internal error path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_unavailable" {
			t.Fatalf("unexpected code for internal list error: %#v", payload["code"])
		}
	})

	t.Run("stats internal error path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/history/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_unavailable" {
			t.Fatalf("unexpected code for internal stats error: %#v", payload["code"])
		}
	})

	t.Run("delete internal error path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/history/his_internal", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_unavailable" {
			t.Fatalf("unexpected code for internal delete error: %#v", payload["code"])
		}
	})
}

func TestHandleHistoryStatsAndDeleteServiceUnavailable(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	// auth required branches for stats/delete/redownload
	unauthCases := []struct {
		name string
		url  string
		verb string
	}{
		{name: "stats unauthorized", verb: http.MethodGet, url: "/v1/history/stats"},
		{name: "delete unauthorized", verb: http.MethodDelete, url: "/v1/history/his_missing"},
		{name: "redownload unauthorized", verb: http.MethodPost, url: "/v1/history/his_missing/redownload"},
	}
	for _, tc := range unauthCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.verb, tc.url, nil)
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	server.historyStore = nil

	statsReq := httptest.NewRequest(http.MethodGet, "/v1/history/stats", nil)
	statsReq.Header.Set("Authorization", "Bearer "+token)
	statsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(statsRec, statsReq)
	if statsRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for stats unavailable, got %d", statsRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/history/his_missing", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for delete unavailable, got %d", deleteRec.Code)
	}

	redownloadReq := httptest.NewRequest(http.MethodPost, "/v1/history/his_missing/redownload", nil)
	redownloadReq.Header.Set("Authorization", "Bearer "+token)
	redownloadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(redownloadRec, redownloadReq)
	if redownloadRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for redownload unavailable, got %d", redownloadRec.Code)
	}

	// Internal get-item error branch with non-nil but uninitialized store
	server.historyStore = &history.Store{}
	internalReq := httptest.NewRequest(http.MethodPost, "/v1/history/his_internal/redownload", nil)
	internalReq.Header.Set("Authorization", "Bearer "+token)
	internalRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(internalRec, internalReq)
	if internalRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for internal redownload error, got %d body=%s", internalRec.Code, internalRec.Body.String())
	}
}

func TestHistoryHandlerDirectDefaultBranch(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item, err := server.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_direct_default",
		UserID:        userID,
		Platform:      history.PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=default",
		SourceURLHash: "hash_direct_default",
		Title:         "Direct Default",
		LastAttemptAt: ptrTimeTest(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected item upsert error: %v", err)
	}
	_, err = server.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_direct_default",
		HistoryItemID: item.ID,
		UserID:        userID,
		RequestKind:   history.RequestKindMP3,
		Status:        history.StatusDone,
		DownloadURL:   "https://signed.example.com/direct-default.mp3",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected attempt create error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/unused", bytes.NewBufferString(`{"request_kind":"mp3"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("id", item.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rec := httptest.NewRecorder()
	server.handleHistoryRedownload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from direct handler call, got %d body=%s", rec.Code, rec.Body.String())
	}
}
