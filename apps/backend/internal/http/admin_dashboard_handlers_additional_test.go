package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAdminDashboard_ErrorAndLimitBranches(t *testing.T) {
	t.Run("auth service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.authService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard", nil)
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when auth service missing, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("maintenance service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.maintenanceService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard", nil)
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when maintenance service missing, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "maintenance_unavailable" {
			t.Fatalf("expected maintenance_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("job store error surfaces 500", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		store.listErr = errors.New("list failed")
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard", nil)
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 when job store list fails, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("query limits clamp and fallback", func(t *testing.T) {
		cfg := baseTestConfig()
		store := newFakeJobStore()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)
		_, _ = registerUserAndGetToken(t, server)
		_, _ = registerUserAndGetToken(t, server)

		clampedReq := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard?users_limit=999&jobs_limit=999", nil)
		clampedReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		clampedRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(clampedRec, clampedReq)
		if clampedRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for clamped dashboard query, got %d body=%s", clampedRec.Code, clampedRec.Body.String())
		}
		clampedPayload := decodeJSONMap(t, clampedRec.Body.Bytes())
		usersObj := clampedPayload["users"].(map[string]any)
		pageObj := usersObj["page"].(map[string]any)
		if got := int(pageObj["limit"].(float64)); got != maxAdminDashboardUsersLimit {
			t.Fatalf("expected users limit clamped to %d, got %d", maxAdminDashboardUsersLimit, got)
		}
		if store.lastListLimit != maxAdminDashboardJobsLimit {
			t.Fatalf("expected jobs limit clamped to %d, got %d", maxAdminDashboardJobsLimit, store.lastListLimit)
		}

		fallbackReq := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard?users_limit=abc&jobs_limit=0", nil)
		fallbackReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		fallbackRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(fallbackRec, fallbackReq)
		if fallbackRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for fallback dashboard query, got %d body=%s", fallbackRec.Code, fallbackRec.Body.String())
		}
		fallbackPayload := decodeJSONMap(t, fallbackRec.Body.Bytes())
		usersObj = fallbackPayload["users"].(map[string]any)
		pageObj = usersObj["page"].(map[string]any)
		if got := int(pageObj["limit"].(float64)); got != defaultAdminDashboardUsersLimit {
			t.Fatalf("expected users limit fallback to %d, got %d", defaultAdminDashboardUsersLimit, got)
		}
		if store.lastListLimit != defaultAdminDashboardJobsLimit {
			t.Fatalf("expected jobs limit fallback to %d, got %d", defaultAdminDashboardJobsLimit, store.lastListLimit)
		}
	})
}
