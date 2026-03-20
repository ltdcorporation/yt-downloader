package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type authResponsePayload struct {
	User struct {
		ID       string `json:"id"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
	} `json:"user"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   string `json:"expires_at"`
}

func TestAuth_RegisterLoginMeLogout(t *testing.T) {
	cfg := baseTestConfig()
	cfg.AuthSessionCookieName = "qs_session_test"
	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()

	server := newTestServer(t, cfg, resolver, queue, store)
	handler := server.Handler()

	registerBody := map[string]any{
		"full_name":      "Test User",
		"email":          "test@example.com",
		"password":       "StrongPass123",
		"keep_logged_in": true,
	}
	registerRaw, _ := json.Marshal(registerBody)

	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerRaw))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)

	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	setCookieHeader := registerRec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookieHeader, "qs_session_test=") {
		t.Fatalf("expected auth cookie, got header=%q", setCookieHeader)
	}
	if !strings.Contains(strings.ToLower(setCookieHeader), "httponly") {
		t.Fatalf("expected HttpOnly cookie, got header=%q", setCookieHeader)
	}

	var registerPayload authResponsePayload
	if err := json.Unmarshal(registerRec.Body.Bytes(), &registerPayload); err != nil {
		t.Fatalf("decode register response failed: %v", err)
	}
	if registerPayload.AccessToken == "" {
		t.Fatalf("expected access token in register response")
	}

	meReqBearer := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReqBearer.Header.Set("Authorization", "Bearer "+registerPayload.AccessToken)
	meRecBearer := httptest.NewRecorder()
	handler.ServeHTTP(meRecBearer, meReqBearer)
	if meRecBearer.Code != http.StatusOK {
		t.Fatalf("auth/me with bearer expected 200, got %d body=%s", meRecBearer.Code, meRecBearer.Body.String())
	}

	cookieParts := strings.Split(setCookieHeader, ";")
	if len(cookieParts) == 0 {
		t.Fatalf("invalid Set-Cookie header: %q", setCookieHeader)
	}
	authCookiePair := strings.TrimSpace(cookieParts[0])

	meReqCookie := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReqCookie.Header.Set("Cookie", authCookiePair)
	meRecCookie := httptest.NewRecorder()
	handler.ServeHTTP(meRecCookie, meReqCookie)
	if meRecCookie.Code != http.StatusOK {
		t.Fatalf("auth/me with cookie expected 200, got %d body=%s", meRecCookie.Code, meRecCookie.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutReq.Header.Set("Cookie", authCookiePair)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
	if !strings.Contains(strings.ToLower(logoutRec.Header().Get("Set-Cookie")), "max-age=0") &&
		!strings.Contains(strings.ToLower(logoutRec.Header().Get("Set-Cookie")), "max-age=-1") {
		t.Fatalf("logout should clear cookie, got Set-Cookie=%q", logoutRec.Header().Get("Set-Cookie"))
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meAfterLogoutReq.Header.Set("Authorization", "Bearer "+registerPayload.AccessToken)
	meAfterLogoutRec := httptest.NewRecorder()
	handler.ServeHTTP(meAfterLogoutRec, meAfterLogoutReq)
	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("auth/me after logout expected 401, got %d body=%s", meAfterLogoutRec.Code, meAfterLogoutRec.Body.String())
	}
}

func TestAuth_LoginInvalidCredentials(t *testing.T) {
	cfg := baseTestConfig()
	resolver := &fakeResolver{}
	queue := &fakeQueue{}
	store := newFakeJobStore()

	server := newTestServer(t, cfg, resolver, queue, store)
	handler := server.Handler()

	registerBody := map[string]any{
		"full_name": "Test User",
		"email":     "test@example.com",
		"password":  "StrongPass123",
	}
	registerRaw, _ := json.Marshal(registerBody)
	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerRaw))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	loginBody := map[string]any{
		"email":    "test@example.com",
		"password": "wrong-pass-123",
	}
	loginRaw, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginRaw))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("login expected 401, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	payload := decodeJSONMap(t, loginRec.Body.Bytes())
	if payload["code"] != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials code, got %v", payload["code"])
	}
}

func TestAuth_RejectUnknownRequestFields(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	body := `{"email":"a@example.com","password":"StrongPass123","unknown":"field"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d body=%s", rec.Code, rec.Body.String())
	}
}
