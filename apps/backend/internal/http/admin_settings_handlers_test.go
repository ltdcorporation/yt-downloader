package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminSettingsHandlers_GetAndPatchFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	getReq := httptest.NewRequest(http.MethodGet, "/v1/admin/settings", nil)
	getReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/admin/settings expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	getPayload := decodeJSONMap(t, getRec.Body.Bytes())
	metaObj, ok := getPayload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object in admin settings response: %v", getPayload)
	}
	versionFloat, ok := metaObj["version"].(float64)
	if !ok || int(versionFloat) != 1 {
		t.Fatalf("expected admin settings version=1, got %+v", metaObj["version"])
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

	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/admin/settings", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("X-Request-ID", "req_admin_settings_test")
	patchReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/admin/settings expected 200, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	patchPayload := decodeJSONMap(t, patchRec.Body.Bytes())
	metaObj, ok = patchPayload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object in patch response")
	}
	versionFloat, ok = metaObj["version"].(float64)
	if !ok || int(versionFloat) != 2 {
		t.Fatalf("expected admin settings version=2 after patch, got %+v", metaObj["version"])
	}

	conflictBody := map[string]any{
		"settings": map[string]any{
			"preferences": map[string]any{"default_quality": "1080p"},
		},
		"meta": map[string]any{"version": 1},
	}
	conflictRaw, _ := json.Marshal(conflictBody)

	conflictReq := httptest.NewRequest(http.MethodPatch, "/v1/admin/settings", bytes.NewReader(conflictRaw))
	conflictReq.Header.Set("Content-Type", "application/json")
	conflictReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	conflictRec := httptest.NewRecorder()
	handler.ServeHTTP(conflictRec, conflictReq)

	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("PATCH /v1/admin/settings conflict expected 409, got %d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
	conflictPayload := decodeJSONMap(t, conflictRec.Body.Bytes())
	if conflictPayload["code"] != "admin_settings_version_conflict" {
		t.Fatalf("expected admin_settings_version_conflict code, got %v", conflictPayload["code"])
	}
}
