package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yt-downloader/backend/internal/maintenance"
)

func TestMaintenancePathBypass_Table(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "empty", path: "", want: false},
		{name: "healthz", path: "/healthz", want: true},
		{name: "public maintenance", path: "/v1/maintenance", want: true},
		{name: "auth me", path: "/v1/auth/me", want: true},
		{name: "admin path", path: "/admin/jobs", want: true},
		{name: "v1 admin path", path: "/v1/admin/settings", want: true},
		{name: "protected api path", path: "/v1/history", want: false},
		{name: "trimmed bypass", path: "   /v1/maintenance   ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maintenancePathBypass(tt.path)
			if got != tt.want {
				t.Fatalf("unexpected bypass decision for path=%q got=%v want=%v", tt.path, got, tt.want)
			}
		})
	}
}

func TestMapMaintenancePatch_ValidationAndNormalization(t *testing.T) {
	t.Run("valid payload normalized", func(t *testing.T) {
		enabled := true
		downTime := " 30 minutes "
		message := " Scheduled update "
		status := " MAINTENANCE "
		serviceEnabled := false

		patch, err := mapMaintenancePatch(&maintenancePatchPayload{
			Enabled:           &enabled,
			EstimatedDowntime: &downTime,
			PublicMessage:     &message,
			Services: []maintenanceServicePatchItem{
				{Key: " API_GATEWAY ", Status: &status, Enabled: &serviceEnabled},
			},
		})
		if err != nil {
			t.Fatalf("expected valid maintenance patch, got err=%v", err)
		}
		if patch.Enabled == nil || !*patch.Enabled {
			t.Fatalf("expected enabled=true in patch, got %#v", patch.Enabled)
		}
		if patch.EstimatedDowntime == nil || *patch.EstimatedDowntime != "30 minutes" {
			t.Fatalf("expected trimmed estimated downtime, got %#v", patch.EstimatedDowntime)
		}
		if patch.PublicMessage == nil || *patch.PublicMessage != "Scheduled update" {
			t.Fatalf("expected trimmed public message, got %#v", patch.PublicMessage)
		}
		if len(patch.Services) != 1 {
			t.Fatalf("expected exactly 1 service patch, got %d", len(patch.Services))
		}
		service := patch.Services[0]
		if service.Key != maintenance.ServiceAPIGateway {
			t.Fatalf("expected api_gateway key, got %q", service.Key)
		}
		if service.Status == nil || *service.Status != maintenance.StatusMaintenance {
			t.Fatalf("expected normalized status=maintenance, got %#v", service.Status)
		}
		if service.Enabled == nil || *service.Enabled {
			t.Fatalf("expected enabled=false for service patch, got %#v", service.Enabled)
		}
	})

	t.Run("rejects empty payload", func(t *testing.T) {
		_, err := mapMaintenancePatch(&maintenancePatchPayload{})
		if err == nil || !strings.Contains(err.Error(), "at least one field") {
			t.Fatalf("expected empty patch error, got %v", err)
		}
	})

	t.Run("rejects unsupported service key", func(t *testing.T) {
		status := "active"
		_, err := mapMaintenancePatch(&maintenancePatchPayload{
			Services: []maintenanceServicePatchItem{{Key: "unknown", Status: &status}},
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported service key") {
			t.Fatalf("expected unsupported service key error, got %v", err)
		}
	})

	t.Run("rejects unsupported service status", func(t *testing.T) {
		status := "broken"
		_, err := mapMaintenancePatch(&maintenancePatchPayload{
			Services: []maintenanceServicePatchItem{{Key: "api_gateway", Status: &status}},
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported service status") {
			t.Fatalf("expected unsupported service status error, got %v", err)
		}
	})

	t.Run("rejects service patch without fields", func(t *testing.T) {
		_, err := mapMaintenancePatch(&maintenancePatchPayload{
			Services: []maintenanceServicePatchItem{{Key: "api_gateway"}},
		})
		if err == nil || !strings.Contains(err.Error(), "must include status or enabled") {
			t.Fatalf("expected missing service patch fields error, got %v", err)
		}
	})
}

func TestMaintenanceHandlers_ValidationAndServiceUnavailable(t *testing.T) {
	t.Run("public and admin get return unavailable when service missing", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.maintenanceService = nil

		publicReq := httptest.NewRequest(http.MethodGet, "/v1/maintenance", nil)
		publicRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(publicRec, publicReq)
		if publicRec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for public maintenance when service missing, got %d body=%s", publicRec.Code, publicRec.Body.String())
		}

		adminReq := httptest.NewRequest(http.MethodGet, "/v1/admin/maintenance", nil)
		adminReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		adminRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(adminRec, adminReq)
		if adminRec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for admin maintenance when service missing, got %d body=%s", adminRec.Code, adminRec.Body.String())
		}
	})

	t.Run("admin patch validates payload branches", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		cases := []struct {
			name string
			body string
			want int
		}{
			{name: "invalid json", body: `{`, want: http.StatusBadRequest},
			{name: "missing meta", body: `{"maintenance":{"enabled":true}}`, want: http.StatusBadRequest},
			{name: "missing maintenance", body: `{"meta":{"version":1}}`, want: http.StatusBadRequest},
			{name: "empty patch", body: `{"maintenance":{},"meta":{"version":1}}`, want: http.StatusBadRequest},
			{name: "invalid service status", body: `{"maintenance":{"services":[{"key":"api_gateway","status":"broken"}]},"meta":{"version":1}}`, want: http.StatusBadRequest},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPatch, "/v1/admin/maintenance", bytes.NewBufferString(tc.body))
				req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("unexpected status for %s got=%d want=%d body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
				}
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != "maintenance_invalid_request" {
					t.Fatalf("expected maintenance_invalid_request code, got %#v", payload["code"])
				}
			})
		}
	})

	t.Run("admin patch version conflict", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		getReq := httptest.NewRequest(http.MethodGet, "/v1/admin/maintenance", nil)
		getReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		getRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for admin maintenance get, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		payload := decodeJSONMap(t, getRec.Body.Bytes())
		meta, ok := payload["meta"].(map[string]any)
		if !ok {
			t.Fatalf("expected meta object, got %#v", payload["meta"])
		}
		version := int(meta["version"].(float64))

		patch := func(version int) *httptest.ResponseRecorder {
			body := map[string]any{
				"maintenance": map[string]any{"enabled": true},
				"meta":        map[string]any{"version": version},
			}
			raw, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPatch, "/v1/admin/maintenance", bytes.NewReader(raw))
			req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, req)
			return rec
		}

		first := patch(version)
		if first.Code != http.StatusOK {
			t.Fatalf("expected first patch 200, got %d body=%s", first.Code, first.Body.String())
		}

		conflict := patch(version)
		if conflict.Code != http.StatusConflict {
			t.Fatalf("expected stale patch 409, got %d body=%s", conflict.Code, conflict.Body.String())
		}
		conflictPayload := decodeJSONMap(t, conflict.Body.Bytes())
		if conflictPayload["code"] != "maintenance_version_conflict" {
			t.Fatalf("expected maintenance_version_conflict code, got %#v", conflictPayload["code"])
		}
	})
}

func TestMaintenanceGuardMiddleware_FailOpenWhenServiceErrors(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	// Use a service with an uninitialized store so Get() errors and middleware must fail-open.
	server.maintenanceService = maintenance.NewService(&maintenance.Store{})

	req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusServiceUnavailable {
		t.Fatalf("expected maintenance middleware fail-open to avoid 503, got body=%s", rec.Body.String())
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected downstream auth middleware response 401 after fail-open, got %d body=%s", rec.Code, rec.Body.String())
	}
}
