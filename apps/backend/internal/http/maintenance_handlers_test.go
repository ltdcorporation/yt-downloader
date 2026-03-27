package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaintenanceEndpointsAndGuard(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	getReq := httptest.NewRequest(http.MethodGet, "/v1/admin/maintenance", nil)
	getReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin maintenance get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	getPayload := decodeJSONMap(t, getRec.Body.Bytes())
	metaObj, ok := getPayload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta object, got %#v", getPayload["meta"])
	}
	versionFloat, ok := metaObj["version"].(float64)
	if !ok {
		t.Fatalf("expected meta.version number, got %#v", metaObj["version"])
	}

	patchBody := bytes.NewBufferString(fmt.Sprintf(`{"maintenance":{"enabled":true,"public_message":"Scheduled maintenance"},"meta":{"version":%d}}`, int64(versionFloat)))
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/admin/maintenance", patchBody)
	patchReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin maintenance patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/v1/maintenance", nil)
	publicRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for maintenance public get, got %d body=%s", publicRec.Code, publicRec.Body.String())
	}
	publicPayload := decodeJSONMap(t, publicRec.Body.Bytes())
	maintenanceObj, ok := publicPayload["maintenance"].(map[string]any)
	if !ok {
		t.Fatalf("expected maintenance object, got %#v", publicPayload["maintenance"])
	}
	if maintenanceObj["enabled"] != true {
		t.Fatalf("expected maintenance enabled=true, got %#v", maintenanceObj["enabled"])
	}

	blockedReq := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	blockedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for blocked route in maintenance mode, got %d body=%s", blockedRec.Code, blockedRec.Body.String())
	}
	blockedPayload := decodeJSONMap(t, blockedRec.Body.Bytes())
	if blockedPayload["code"] != "maintenance_mode" {
		t.Fatalf("expected maintenance_mode code, got %#v", blockedPayload["code"])
	}

	adminBypassReq := httptest.NewRequest(http.MethodGet, "/v1/admin/maintenance", nil)
	adminBypassReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	adminBypassRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(adminBypassRec, adminBypassReq)
	if adminBypassRec.Code != http.StatusOK {
		t.Fatalf("expected admin maintenance route to bypass maintenance guard, got %d body=%s", adminBypassRec.Code, adminBypassRec.Body.String())
	}
}
