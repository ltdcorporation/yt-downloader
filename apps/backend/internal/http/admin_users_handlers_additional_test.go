package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
)

func TestAdminUsersListEndpoint_PaginationAndFallback(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	_, _ = registerUserAndGetToken(t, server)
	_, _ = registerUserAndGetToken(t, server)
	_, _ = registerUserAndGetToken(t, server)

	pagedReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users?limit=1&offset=1", nil)
	pagedReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	pagedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(pagedRec, pagedReq)
	if pagedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for paged admin users list, got %d body=%s", pagedRec.Code, pagedRec.Body.String())
	}
	pagedPayload := decodeJSONMap(t, pagedRec.Body.Bytes())
	pagedPage := pagedPayload["page"].(map[string]any)
	if got := int(pagedPage["limit"].(float64)); got != 1 {
		t.Fatalf("expected limit=1, got %d", got)
	}
	if got := int(pagedPage["offset"].(float64)); got != 1 {
		t.Fatalf("expected offset=1, got %d", got)
	}

	fallbackReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users?limit=999&offset=-5", nil)
	fallbackReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	fallbackRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(fallbackRec, fallbackReq)
	if fallbackRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for fallback admin users list, got %d body=%s", fallbackRec.Code, fallbackRec.Body.String())
	}
	fallbackPayload := decodeJSONMap(t, fallbackRec.Body.Bytes())
	fallbackPage := fallbackPayload["page"].(map[string]any)
	if got := int(fallbackPage["limit"].(float64)); got != 20 {
		t.Fatalf("expected fallback limit=20, got %d", got)
	}
	if got := int(fallbackPage["offset"].(float64)); got != 0 {
		t.Fatalf("expected fallback offset=0, got %d", got)
	}
}

func TestAdminUserHandlers_ErrorScenarios(t *testing.T) {
	t.Run("auth service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.authService = nil

		statsReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users/stats", nil)
		statsReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		statsRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(statsRec, statsReq)
		if statsRec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for stats when auth service missing, got %d body=%s", statsRec.Code, statsRec.Body.String())
		}
	})

	t.Run("missing route param for get and patch", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		getReq := httptest.NewRequest(http.MethodGet, "/unused", nil)
		getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		getRec := httptest.NewRecorder()
		server.handleAdminUserGet(getRec, getReq)
		if getRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing user id on get handler, got %d body=%s", getRec.Code, getRec.Body.String())
		}

		patchReq := httptest.NewRequest(http.MethodPatch, "/unused", bytes.NewBufferString(`{"full_name":"Any Name"}`))
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq = patchReq.WithContext(context.WithValue(patchReq.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		patchRec := httptest.NewRecorder()
		server.handleAdminUserPatch(patchRec, patchReq)
		if patchRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing user id on patch handler, got %d body=%s", patchRec.Code, patchRec.Body.String())
		}
	})

	t.Run("patch missing target user", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

		req := httptest.NewRequest(http.MethodPatch, "/v1/admin/users/usr_does_not_exist", bytes.NewBufferString(`{"plan":"weekly"}`))
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 when patching missing user, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestParseAdminUserPatchRequest_ClearPlanExpiry(t *testing.T) {
	expires := "   "
	role := " USER "
	plan := " WEEKLY "
	name := "  Admin Patch Name  "

	parsed, err := parseAdminUserPatchRequest(adminUserPatchRequest{
		FullName:      &name,
		Role:          &role,
		Plan:          &plan,
		PlanExpiresAt: &expires,
	})
	if err != nil {
		t.Fatalf("parse should succeed for clear-plan-expiry payload: %v", err)
	}
	if parsed.FullName == nil || *parsed.FullName != "Admin Patch Name" {
		t.Fatalf("expected trimmed full_name, got %#v", parsed.FullName)
	}
	if parsed.Role == nil || *parsed.Role != auth.RoleUser {
		t.Fatalf("expected normalized role=user, got %#v", parsed.Role)
	}
	if parsed.Plan == nil || *parsed.Plan != auth.PlanWeekly {
		t.Fatalf("expected normalized plan=weekly, got %#v", parsed.Plan)
	}
	if !parsed.PlanExpiresAtSet {
		t.Fatalf("expected PlanExpiresAtSet=true")
	}
	if parsed.PlanExpiresAt != nil {
		t.Fatalf("expected plan expiry to be explicitly cleared, got %#v", parsed.PlanExpiresAt)
	}

	// Also verify explicit timestamp remains UTC-normalized.
	rfcTime := time.Date(2026, 4, 2, 12, 0, 0, 0, time.FixedZone("wib", 7*3600)).Format(time.RFC3339)
	parsed, err = parseAdminUserPatchRequest(adminUserPatchRequest{PlanExpiresAt: &rfcTime})
	if err != nil {
		t.Fatalf("parse should accept RFC3339 plan_expires_at: %v", err)
	}
	if parsed.PlanExpiresAt == nil || parsed.PlanExpiresAt.Location() != time.UTC {
		t.Fatalf("expected parsed plan_expires_at to be stored in UTC, got %#v", parsed.PlanExpiresAt)
	}
}
