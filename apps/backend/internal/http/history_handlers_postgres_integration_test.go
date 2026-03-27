package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yt-downloader/backend/internal/history"
)

func TestHandleHistoryCreate_PostgresResolvedStatus(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	cfg := baseTestConfig()
	cfg.PostgresDSN = dsn

	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	body, err := json.Marshal(map[string]any{
		"url":           "https://www.youtube.com/watch?v=abc123",
		"platform":      "youtube",
		"title":         "Postgres Resolved Attempt",
		"thumbnail_url": "https://img.example.com/postgres-resolved.jpg",
	})
	if err != nil {
		t.Fatalf("failed to marshal history create payload: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/history", bytes.NewBuffer(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for history create on postgres, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/history?status=resolved&limit=10", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for history list, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	payload := decodeJSONMap(t, listRec.Body.Bytes())
	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", payload["items"])
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 resolved history item, got %d payload=%#v", len(items), payload)
	}

	entry, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected history entry object, got %#v", items[0])
	}
	latestAttempt, ok := entry["latest_attempt"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest_attempt object, got %#v", entry["latest_attempt"])
	}
	if latestAttempt["status"] != string(history.StatusResolved) {
		t.Fatalf("expected latest attempt status resolved, got %#v", latestAttempt["status"])
	}
	if latestAttempt["request_kind"] != string(history.RequestKindMP4) {
		t.Fatalf("expected latest attempt request_kind mp4, got %#v", latestAttempt["request_kind"])
	}
}
