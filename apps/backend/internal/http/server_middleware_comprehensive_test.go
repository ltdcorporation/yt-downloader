package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"yt-downloader/backend/internal/auth"
)

func TestServerMiddleware_BasicAuth(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	handler := server.basicAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	unauthorizedRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing basic auth, got %d body=%s", unauthorizedRec.Code, unauthorizedRec.Body.String())
	}
	if got := unauthorizedRec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatalf("expected WWW-Authenticate header on unauthorized response")
	}

	authorizedReq := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	authorizedReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	authorizedRec := httptest.NewRecorder()
	handler.ServeHTTP(authorizedRec, authorizedReq)
	if authorizedRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid basic auth, got %d body=%s", authorizedRec.Code, authorizedRec.Body.String())
	}
}

func TestServerMiddleware_RequireSessionMiddleware(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	handler := server.requireSessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := r.Context().Value("identity").(auth.SessionIdentity)
		if !ok {
			t.Fatalf("expected auth.SessionIdentity in context")
		}
		if identity.User.ID != userID {
			t.Fatalf("expected identity user id %q, got %q", userID, identity.User.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	validReq := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	validReq.Header.Set("Authorization", "Bearer "+token)
	validRec := httptest.NewRecorder()
	handler.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid session token, got %d body=%s", validRec.Code, validRec.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing session token, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	server.authService = nil
	nilServiceReq := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
	nilServiceReq.Header.Set("Authorization", "Bearer "+token)
	nilServiceRec := httptest.NewRecorder()
	handler.ServeHTTP(nilServiceRec, nilServiceReq)
	if nilServiceRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when auth service is nil, got %d body=%s", nilServiceRec.Code, nilServiceRec.Body.String())
	}
}

func TestServerMiddleware_AdminAuthBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	nonAdminToken, _ := registerUserAndGetToken(t, server)
	adminToken, adminUserID := registerUserAndGetToken(t, server)
	adminRole := auth.RoleAdmin
	if _, err := server.authStore.UpdateUserByAdmin(context.Background(), adminUserID, auth.AdminUserPatch{Role: &adminRole}, time.Now().UTC()); err != nil {
		t.Fatalf("failed to promote test user to admin role: %v", err)
	}

	handler := server.adminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := r.Context().Value("identity").(auth.SessionIdentity)
		if ok {
			if identity.User.Role != auth.RoleAdmin {
				t.Fatalf("expected admin role in context identity, got %s", identity.User.Role)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("basic auth valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for valid basic auth, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing auth prompts basic", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for missing admin auth, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("WWW-Authenticate"); got == "" {
			t.Fatalf("expected WWW-Authenticate header on unauthorized admin auth response")
		}
	})

	t.Run("bearer token non-admin rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+nonAdminToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-admin bearer token, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bearer token invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.Header.Set("Authorization", "Bearer st_invalid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for invalid bearer token, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "invalid_session" {
			t.Fatalf("expected invalid_session code, got %#v", payload["code"])
		}
	})

	t.Run("bearer token admin passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for admin bearer token, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("auth service nil with bearer token", func(t *testing.T) {
		token, _ := registerUserAndGetToken(t, server)
		server.authService = nil
		req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when auth service is nil with bearer token, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestServerMiddleware_RequireAdmin(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	handler := server.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	missingReq := httptest.NewRequest(http.MethodGet, "/internal/admin", nil)
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when identity missing, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	userReq := httptest.NewRequest(http.MethodGet, "/internal/admin", nil)
	userReq = userReq.WithContext(context.WithValue(userReq.Context(), "identity", auth.SessionIdentity{User: auth.PublicUser{ID: "usr_1", Role: auth.RoleUser}}))
	userRec := httptest.NewRecorder()
	handler.ServeHTTP(userRec, userReq)
	if userRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin identity, got %d body=%s", userRec.Code, userRec.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/internal/admin", nil)
	adminReq = adminReq.WithContext(context.WithValue(adminReq.Context(), "identity", auth.SessionIdentity{User: auth.PublicUser{ID: "usr_admin", Role: auth.RoleAdmin}}))
	adminRec := httptest.NewRecorder()
	handler.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for admin identity, got %d body=%s", adminRec.Code, adminRec.Body.String())
	}
}

func TestIPRateLimiter_CleanupEveryRemovesStaleEntries(t *testing.T) {
	limiter := &ipRateLimiter{
		visitors: map[string]*visitor{
			"stale": {
				limiter:  rate.NewLimiter(1, 1),
				lastSeen: time.Now().UTC().Add(-10 * time.Minute),
			},
			"fresh": {
				limiter:  rate.NewLimiter(1, 1),
				lastSeen: time.Now().UTC(),
			},
		},
		limit: 1,
		burst: 1,
	}

	go limiter.cleanupEvery(5 * time.Millisecond)

	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		limiter.mu.Lock()
		_, staleExists := limiter.visitors["stale"]
		_, freshExists := limiter.visitors["fresh"]
		limiter.mu.Unlock()

		if !staleExists {
			if !freshExists {
				t.Fatalf("fresh visitor should not be removed by cleanup")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale visitor was not cleaned up before timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNewIPRateLimiter_ClampsBurst(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 0)
	if limiter == nil {
		t.Fatalf("expected non-nil limiter for positive limit")
	}
	if limiter.burst != 1 {
		t.Fatalf("expected burst clamped to 1, got %d", limiter.burst)
	}
}

func TestHandleAdminUsersList_ServiceAndErrorBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	server.authService = nil
	nilServiceReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	nilServiceReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	nilServiceRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(nilServiceRec, nilServiceReq)
	if nilServiceRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when auth service is nil, got %d body=%s", nilServiceRec.Code, nilServiceRec.Body.String())
	}

	server = newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	server.authService = auth.NewService(&auth.Store{}, auth.Options{})
	internalErrReq := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	internalErrReq.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	internalErrRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(internalErrRec, internalErrReq)
	if internalErrRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when auth service list users fails, got %d body=%s", internalErrRec.Code, internalErrRec.Body.String())
	}
}
