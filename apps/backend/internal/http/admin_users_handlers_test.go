package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
)

func TestParseAdminUserPatchRequest(t *testing.T) {
	t.Run("valid patch", func(t *testing.T) {
		role := "admin"
		plan := "monthly"
		expires := "2026-04-01T00:00:00Z"
		name := "  Updated User  "

		parsed, err := parseAdminUserPatchRequest(adminUserPatchRequest{
			FullName:      &name,
			Role:          &role,
			Plan:          &plan,
			PlanExpiresAt: &expires,
		})
		if err != nil {
			t.Fatalf("parse should succeed: %v", err)
		}
		if parsed.Role == nil || *parsed.Role != auth.RoleAdmin {
			t.Fatalf("expected role admin, got %+v", parsed.Role)
		}
		if parsed.Plan == nil || *parsed.Plan != auth.PlanMonthly {
			t.Fatalf("expected plan monthly, got %+v", parsed.Plan)
		}
		if parsed.FullName == nil || *parsed.FullName != "Updated User" {
			t.Fatalf("expected normalized full_name, got %+v", parsed.FullName)
		}
		if !parsed.PlanExpiresAtSet || parsed.PlanExpiresAt == nil || parsed.PlanExpiresAt.Format(time.RFC3339) != expires {
			t.Fatalf("expected parsed expires_at, got %+v", parsed.PlanExpiresAt)
		}
	})

	t.Run("empty patch", func(t *testing.T) {
		if _, err := parseAdminUserPatchRequest(adminUserPatchRequest{}); err == nil {
			t.Fatalf("expected empty patch error")
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		role := "owner"
		if _, err := parseAdminUserPatchRequest(adminUserPatchRequest{Role: &role}); err == nil {
			t.Fatalf("expected invalid role error")
		}
	})

	t.Run("invalid plan_expires_at", func(t *testing.T) {
		expires := "not-a-time"
		if _, err := parseAdminUserPatchRequest(adminUserPatchRequest{PlanExpiresAt: &expires}); err == nil {
			t.Fatalf("expected invalid plan_expires_at error")
		}
	})
}

func TestHandleAdminUserGetAndPatch(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	_, targetUserID := registerUserAndGetToken(t, server)

	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/admin/users/%s", targetUserID), nil)
	getReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin user get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	getPayload := decodeJSONMap(t, getRec.Body.Bytes())
	if getPayload["id"] != targetUserID {
		t.Fatalf("unexpected admin user payload: %#v", getPayload)
	}

	patchBody := bytes.NewBufferString(`{"full_name":"  Updated By Admin  ","role":"admin","plan":"weekly","plan_expires_at":"2026-04-01T00:00:00Z"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/admin/users/%s", targetUserID), patchBody)
	patchReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin user patch, got %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	patchPayload := decodeJSONMap(t, patchRec.Body.Bytes())
	if patchPayload["id"] != targetUserID {
		t.Fatalf("unexpected patched user id payload: %#v", patchPayload)
	}
	if patchPayload["full_name"] != "Updated By Admin" {
		t.Fatalf("expected normalized full_name in patch response, got %#v", patchPayload["full_name"])
	}
	if patchPayload["role"] != string(auth.RoleAdmin) {
		t.Fatalf("expected role admin in patch response, got %#v", patchPayload["role"])
	}
	if patchPayload["plan"] != string(auth.PlanWeekly) {
		t.Fatalf("expected plan weekly in patch response, got %#v", patchPayload["plan"])
	}

	verifyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/admin/users/%s", targetUserID), nil)
	verifyReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	verifyRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin user verify get, got %d body=%s", verifyRec.Code, verifyRec.Body.String())
	}
	verifyPayload := decodeJSONMap(t, verifyRec.Body.Bytes())
	if verifyPayload["role"] != string(auth.RoleAdmin) || verifyPayload["plan"] != string(auth.PlanWeekly) {
		t.Fatalf("expected persisted admin changes, got %#v", verifyPayload)
	}

	invalidPatchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/admin/users/%s", targetUserID), bytes.NewBufferString(`{"plan_expires_at":"invalid"}`))
	invalidPatchReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	invalidPatchReq.Header.Set("Content-Type", "application/json")
	invalidPatchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidPatchRec, invalidPatchReq)
	if invalidPatchRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid patch payload, got %d body=%s", invalidPatchRec.Code, invalidPatchRec.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users/usr_missing", nil)
	missingReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	missingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing admin user, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}
}
