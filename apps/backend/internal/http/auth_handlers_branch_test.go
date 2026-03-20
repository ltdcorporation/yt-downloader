package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
)

type fakeHTTPGoogleVerifier struct {
	verifyFn func(ctx context.Context, rawIDToken string) (auth.GoogleTokenClaims, error)
}

func (f fakeHTTPGoogleVerifier) Verify(ctx context.Context, rawIDToken string) (auth.GoogleTokenClaims, error) {
	if f.verifyFn != nil {
		return f.verifyFn(ctx, rawIDToken)
	}
	return auth.GoogleTokenClaims{}, nil
}

func TestAuthHandlers_ServiceUnavailable(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	server.authService = nil

	handler := server.Handler()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "register", method: http.MethodPost, path: "/v1/auth/register", body: `{"full_name":"A","email":"a@example.com","password":"StrongPass123"}`},
		{name: "login", method: http.MethodPost, path: "/v1/auth/login", body: `{"email":"a@example.com","password":"StrongPass123"}`},
		{name: "google-login", method: http.MethodPost, path: "/v1/auth/google", body: `{"id_token":"dummy"}`},
		{name: "me", method: http.MethodGet, path: "/v1/auth/me"},
		{name: "logout", method: http.MethodPost, path: "/v1/auth/logout"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAuth_RegisterValidationAndDuplicate(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	invalidJSONReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", strings.NewReader(`{"full_name":`))
	invalidJSONReq.Header.Set("Content-Type", "application/json")
	invalidJSONRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidJSONRec, invalidJSONReq)
	if invalidJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", invalidJSONRec.Code)
	}

	validationBody := map[string]any{
		"full_name": "",
		"email":     "test@example.com",
		"password":  "StrongPass123",
	}
	validationRaw, _ := json.Marshal(validationBody)
	validationReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(validationRaw))
	validationReq.Header.Set("Content-Type", "application/json")
	validationRec := httptest.NewRecorder()
	handler.ServeHTTP(validationRec, validationReq)
	if validationRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for validation error, got %d body=%s", validationRec.Code, validationRec.Body.String())
	}

	registerBody := map[string]any{
		"full_name": "Test User",
		"email":     "dup@example.com",
		"password":  "StrongPass123",
	}
	registerRaw, _ := json.Marshal(registerBody)

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerRaw))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first register 201, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerRaw))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected duplicate register 409, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}
}

func TestAuth_LoginValidationAndInvalidJSON(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	invalidJSONReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":`))
	invalidJSONReq.Header.Set("Content-Type", "application/json")
	invalidJSONRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidJSONRec, invalidJSONReq)
	if invalidJSONRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid json, got %d", invalidJSONRec.Code)
	}

	validationReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"user@example.com"}`))
	validationReq.Header.Set("Content-Type", "application/json")
	validationRec := httptest.NewRecorder()
	handler.ServeHTTP(validationRec, validationReq)
	if validationRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 missing password, got %d body=%s", validationRec.Code, validationRec.Body.String())
	}
}

func TestAuthMeAndLogoutAdditionalPaths(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d body=%s", meRec.Code, meRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected logout without token to succeed, got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
}

func TestAuth_GoogleLoginFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	server.authService = auth.NewService(server.authStore, auth.Options{
		GoogleTokenVerifier: fakeHTTPGoogleVerifier{
			verifyFn: func(_ context.Context, rawIDToken string) (auth.GoogleTokenClaims, error) {
				if rawIDToken != "google-id-token" {
					return auth.GoogleTokenClaims{}, auth.ErrGoogleTokenInvalid
				}
				return auth.GoogleTokenClaims{Subject: "sub_http", Email: "http-google@example.com", EmailVerified: true, FullName: "HTTP Google"}, nil
			},
		},
	})
	handler := server.Handler()

	invalidReq := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"id_token":"bad-token"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid google token, got %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"id_token":"google-id-token","keep_logged_in":true}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for google login, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	if !strings.Contains(loginRec.Header().Get("Set-Cookie"), "qs_session=") {
		t.Fatalf("expected auth session cookie from google login, got %q", loginRec.Header().Get("Set-Cookie"))
	}
}

func TestWriteAuthError(t *testing.T) {
	s := &Server{logger: log.New(io.Discard, "", 0)}

	rec := httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "register", &auth.ValidationError{Message: "bad input"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 validation, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "register", auth.ErrEmailTaken)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 email taken, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "login", auth.ErrInvalidCredentials)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 invalid credentials, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "google_login", auth.ErrGoogleAuthDisabled)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 google disabled, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "google_login", auth.ErrGoogleTokenInvalid)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 invalid google token, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "google_login", auth.ErrGoogleEmailUnverified)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 google email unverified, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "google_login", auth.ErrGoogleIdentityConflict)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 google identity conflict, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.writeAuthError(rec, "x@example.com", "login", io.EOF)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 generic auth error, got %d", rec.Code)
	}
}

func TestReadSessionTokenAndCookieBranches(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := s.readSessionToken(req); got != "" {
		t.Fatalf("expected empty token without auth header/cookie, got %q", got)
	}

	s.cfg.AuthSessionCookieName = "custom_cookie"
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "custom_cookie", Value: "cookie-value"})
	if got := s.readSessionToken(req); got != "cookie-value" {
		t.Fatalf("expected cookie token, got %q", got)
	}
}

func TestSetSessionCookiePastExpiry(t *testing.T) {
	s := &Server{cfg: baseTestConfig()}
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "token", time.Now().UTC().Add(-time.Minute))

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(strings.ToLower(setCookie), "max-age=1") {
		t.Fatalf("expected max-age=1 safeguard for past expiry, got %q", setCookie)
	}
}

func TestAuth_LoginSuccessSetsCookie(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	handler := server.Handler()

	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", strings.NewReader(`{"full_name":"User Login","email":"login@example.com","password":"StrongPass123"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register failed: status=%d body=%s", registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"login@example.com","password":"StrongPass123","keep_logged_in":true}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	if !strings.Contains(loginRec.Header().Get("Set-Cookie"), "qs_session=") {
		t.Fatalf("expected auth cookie from login, got %q", loginRec.Header().Get("Set-Cookie"))
	}
}

func TestAuthHandlers_UninitializedServiceErrors(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	server.authService = &auth.Service{}
	handler := server.Handler()

	registerReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", strings.NewReader(`{"full_name":"User","email":"x@example.com","password":"StrongPass123"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected register 500 for uninitialized service, got %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"x@example.com","password":"StrongPass123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected login 500 for uninitialized service, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	googleReq := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"id_token":"dummy"}`))
	googleReq.Header.Set("Content-Type", "application/json")
	googleRec := httptest.NewRecorder()
	handler.ServeHTTP(googleRec, googleReq)
	if googleRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected google login 500 for uninitialized service, got %d body=%s", googleRec.Code, googleRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected logout 500 for uninitialized service, got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}
}
