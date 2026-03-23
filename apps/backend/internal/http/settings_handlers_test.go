package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSettingsHandlers_GetAndPatchFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	token, _ := registerUserAndGetToken(t, server)

	getReq := httptest.NewRequest(http.MethodGet, "/v1/settings", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/settings expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	getPayload := decodeJSONMap(t, getRec.Body.Bytes())
	metaObj, ok := getPayload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object in settings response: %v", getPayload)
	}
	versionFloat, ok := metaObj["version"].(float64)
	if !ok || int(versionFloat) != 1 {
		t.Fatalf("expected settings version=1, got %+v", metaObj["version"])
	}

	patchBody := map[string]any{
		"settings": map[string]any{
			"preferences": map[string]any{
				"default_quality":      "720p",
				"auto_trim_silence":    true,
				"thumbnail_generation": true,
			},
			"notifications": map[string]any{
				"email": map[string]any{
					"summary": true,
				},
			},
		},
		"meta": map[string]any{"version": 1},
	}
	patchRaw, _ := json.Marshal(patchBody)

	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/settings", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("X-Request-ID", "req_settings_test")
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/settings expected 200, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	patchPayload := decodeJSONMap(t, patchRec.Body.Bytes())
	metaObj, ok = patchPayload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object in patch response")
	}
	versionFloat, ok = metaObj["version"].(float64)
	if !ok || int(versionFloat) != 2 {
		t.Fatalf("expected settings version=2 after patch, got %+v", metaObj["version"])
	}

	conflictBody := map[string]any{
		"settings": map[string]any{
			"preferences": map[string]any{"default_quality": "1080p"},
		},
		"meta": map[string]any{"version": 1},
	}
	conflictRaw, _ := json.Marshal(conflictBody)

	conflictReq := httptest.NewRequest(http.MethodPatch, "/v1/settings", bytes.NewReader(conflictRaw))
	conflictReq.Header.Set("Content-Type", "application/json")
	conflictReq.Header.Set("Authorization", "Bearer "+token)
	conflictRec := httptest.NewRecorder()
	handler.ServeHTTP(conflictRec, conflictReq)

	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("PATCH /v1/settings conflict expected 409, got %d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
	conflictPayload := decodeJSONMap(t, conflictRec.Body.Bytes())
	if conflictPayload["code"] != "settings_version_conflict" {
		t.Fatalf("expected settings_version_conflict code, got %v", conflictPayload["code"])
	}
}

func TestProfileHandlers_GetAndPatchFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	token, _ := registerUserAndGetToken(t, server)

	getReq := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/profile expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	patchBody := map[string]any{"profile": map[string]any{"full_name": "Renamed User"}}
	patchRaw, _ := json.Marshal(patchBody)
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/profile", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/profile expected 200, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	patchedProfilePayload := decodeJSONMap(t, patchRec.Body.Bytes())
	profileObj, ok := patchedProfilePayload["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile object in patch response")
	}
	if profileObj["full_name"] != "Renamed User" {
		t.Fatalf("unexpected patched full_name: %v", profileObj["full_name"])
	}

	invalidReq := httptest.NewRequest(http.MethodPatch, "/v1/profile", bytes.NewReader([]byte(`{"profile":{"full_name":"A"}}`)))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("Authorization", "Bearer "+token)
	invalidRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH /v1/profile invalid expected 400, got %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	invalidPayload := decodeJSONMap(t, invalidRec.Body.Bytes())
	if invalidPayload["code"] != "profile_invalid_request" {
		t.Fatalf("expected profile_invalid_request code, got %v", invalidPayload["code"])
	}
}
