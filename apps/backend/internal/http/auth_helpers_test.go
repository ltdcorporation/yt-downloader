package http

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
)

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "wrong prefix", raw: "Basic abc", want: ""},
		{name: "missing token", raw: "Bearer    ", want: ""},
		{name: "valid", raw: "Bearer token123", want: "token123"},
		{name: "case insensitive", raw: "bearer tokenABC", want: "tokenABC"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseBearerToken(tc.raw); got != tc.want {
				t.Fatalf("unexpected token parsing result: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestDecodeJSONBody(t *testing.T) {
	type payload struct {
		Email string `json:"email"`
	}

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com"}`))
		var p payload
		if err := decodeJSONBody(req, &p); err != nil {
			t.Fatalf("decodeJSONBody failed: %v", err)
		}
		if p.Email != "a@example.com" {
			t.Fatalf("unexpected payload email: %q", p.Email)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com","x":1}`))
		var p payload
		if err := decodeJSONBody(req, &p); err == nil {
			t.Fatalf("expected unknown field error")
		}
	})

	t.Run("multiple objects rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com"} {}`))
		var p payload
		if err := decodeJSONBody(req, &p); err == nil {
			t.Fatalf("expected multiple objects error")
		}
	})

	t.Run("trailing malformed payload rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com"} [`))
		var p payload
		if err := decodeJSONBody(req, &p); err == nil {
			t.Fatalf("expected trailing malformed payload error")
		}
	})
}

func TestAuthCookieNameAndReadSessionToken(t *testing.T) {
	s := &Server{}
	if got := s.authCookieName(); got != "qs_session" {
		t.Fatalf("expected default auth cookie name, got %q", got)
	}

	s.cfg.AuthSessionCookieName = "custom_cookie"
	if got := s.authCookieName(); got != "custom_cookie" {
		t.Fatalf("expected custom auth cookie name, got %q", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "custom_cookie", Value: "cookie-token"})
	if token := s.readSessionToken(req); token != "header-token" {
		t.Fatalf("expected bearer token precedence, got %q", token)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "custom_cookie", Value: "cookie-token"})
	if token := s.readSessionToken(req); token != "cookie-token" {
		t.Fatalf("expected cookie token fallback, got %q", token)
	}
}

func TestSetAndClearSessionCookie(t *testing.T) {
	s := &Server{}
	s.cfg.AuthSessionCookieName = "custom_cookie"
	s.cfg.AuthSessionCookieSecure = true
	s.cfg.AuthSessionCookieDomain = ".example.com"

	rec := httptest.NewRecorder()
	expiresAt := time.Now().UTC().Add(2 * time.Hour)
	s.setSessionCookie(rec, "token123", expiresAt)

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "custom_cookie=token123") {
		t.Fatalf("expected session cookie value, got %q", setCookie)
	}
	if !strings.Contains(strings.ToLower(setCookie), "httponly") {
		t.Fatalf("expected HttpOnly cookie, got %q", setCookie)
	}
	if !strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Fatalf("expected Secure cookie, got %q", setCookie)
	}
	if !strings.Contains(strings.ToLower(setCookie), "domain=example.com") {
		t.Fatalf("expected cookie domain, got %q", setCookie)
	}

	rec = httptest.NewRecorder()
	s.clearSessionCookie(rec)
	clearCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(clearCookie, "custom_cookie=") {
		t.Fatalf("expected clear cookie entry, got %q", clearCookie)
	}
	if !strings.Contains(strings.ToLower(clearCookie), "max-age=0") && !strings.Contains(strings.ToLower(clearCookie), "max-age=-1") {
		t.Fatalf("expected cleared max-age, got %q", clearCookie)
	}
}

func TestWriteAuthSessionError(t *testing.T) {
	s := &Server{logger: log.New(io.Discard, "", 0)}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid", err: auth.ErrInvalidSessionToken, wantStatus: http.StatusUnauthorized, wantCode: "invalid_session"},
		{name: "revoked", err: auth.ErrSessionRevoked, wantStatus: http.StatusUnauthorized, wantCode: "session_revoked"},
		{name: "expired", err: auth.ErrSessionExpired, wantStatus: http.StatusUnauthorized, wantCode: "session_expired"},
		{name: "generic", err: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.writeAuthSessionError(rec, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("unexpected status: got=%d want=%d", rec.Code, tc.wantStatus)
			}
			if tc.wantCode != "" {
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != tc.wantCode {
					t.Fatalf("unexpected code: got=%v want=%s", payload["code"], tc.wantCode)
				}
			}
		})
	}
}
