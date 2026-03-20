package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"yt-downloader/backend/internal/auth"
)

const maxAuthBodyBytes = 1 << 20 // 1MB

type authRegisterRequest struct {
	FullName     string `json:"full_name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	KeepLoggedIn bool   `json:"keep_logged_in"`
}

type authLoginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	KeepLoggedIn bool   `json:"keep_logged_in"`
}

type authGoogleLoginRequest struct {
	IDToken      string `json:"id_token"`
	KeepLoggedIn bool   `json:"keep_logged_in"`
}

type authMeResponse struct {
	User      auth.PublicUser `json:"user"`
	ExpiresAt time.Time       `json:"expires_at"`
}

func (s *Server) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	var req authRegisterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := s.authService.Register(r.Context(), auth.RegisterInput{
		FullName:     req.FullName,
		Email:        req.Email,
		Password:     req.Password,
		KeepLoggedIn: req.KeepLoggedIn,
		ClientIP:     getClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		s.writeAuthError(w, req.Email, "register", err)
		return
	}

	s.setSessionCookie(w, result.AccessToken, result.ExpiresAt)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	var req authLoginRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := s.authService.Login(r.Context(), auth.LoginInput{
		Email:        req.Email,
		Password:     req.Password,
		KeepLoggedIn: req.KeepLoggedIn,
		ClientIP:     getClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		s.writeAuthError(w, req.Email, "login", err)
		return
	}

	s.setSessionCookie(w, result.AccessToken, result.ExpiresAt)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAuthGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	var req authGoogleLoginRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := s.authService.LoginWithGoogle(r.Context(), auth.GoogleLoginInput{
		IDToken:      req.IDToken,
		KeepLoggedIn: req.KeepLoggedIn,
		ClientIP:     getClientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		s.writeAuthError(w, "", "google_login", err)
		return
	}

	s.setSessionCookie(w, result.AccessToken, result.ExpiresAt)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	token := s.readSessionToken(r)
	identity, err := s.authService.AuthenticateToken(r.Context(), token)
	if err != nil {
		s.writeAuthSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, authMeResponse{
		User:      identity.User,
		ExpiresAt: identity.ExpiresAt,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	token := s.readSessionToken(r)
	if err := s.authService.Logout(r.Context(), token); err != nil {
		s.logger.Printf("auth logout failed err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) writeAuthError(w http.ResponseWriter, email, action string, err error) {
	var validationErr *auth.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "email already registered",
			"code":  "email_taken",
		})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "invalid email or password",
			"code":  "invalid_credentials",
		})
	case errors.Is(err, auth.ErrGoogleAuthDisabled):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "google auth is not configured",
			"code":  "google_auth_unavailable",
		})
	case errors.Is(err, auth.ErrGoogleTokenInvalid):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "invalid google token",
			"code":  "google_token_invalid",
		})
	case errors.Is(err, auth.ErrGoogleEmailUnverified):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "google email is not verified",
			"code":  "google_email_unverified",
		})
	case errors.Is(err, auth.ErrGoogleIdentityConflict):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "google account is linked to another user",
			"code":  "google_identity_conflict",
		})
	default:
		s.logger.Printf("auth %s failed email=%s err=%v", action, strings.TrimSpace(email), err)
		writeError(w, http.StatusInternalServerError, "authentication failed")
	}
}

func (s *Server) writeAuthSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidSessionToken):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "invalid session",
			"code":  "invalid_session",
		})
	case errors.Is(err, auth.ErrSessionRevoked):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "session revoked",
			"code":  "session_revoked",
		})
	case errors.Is(err, auth.ErrSessionExpired):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "session expired",
			"code":  "session_expired",
		})
	default:
		s.logger.Printf("auth session validation failed err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to validate session")
	}
}

func decodeJSONBody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxAuthBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON objects are not allowed")
		}
		return err
	}

	return nil
}

func (s *Server) readSessionToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if token := parseBearerToken(authHeader); token != "" {
		return token
	}

	cookie, err := r.Cookie(s.authCookieName())
	if err == nil && cookie != nil {
		return strings.TrimSpace(cookie.Value)
	}

	return ""
}

func parseBearerToken(headerValue string) string {
	if headerValue == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(headerValue), "bearer ") {
		return ""
	}
	return strings.TrimSpace(headerValue[7:])
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Name:     s.authCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.AuthSessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
	}

	if domain := strings.TrimSpace(s.cfg.AuthSessionCookieDomain); domain != "" {
		cookie.Domain = domain
	}

	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	cookie.MaxAge = maxAge

	http.SetCookie(w, cookie)
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     s.authCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.AuthSessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	}
	if domain := strings.TrimSpace(s.cfg.AuthSessionCookieDomain); domain != "" {
		cookie.Domain = domain
	}
	http.SetCookie(w, cookie)
}

func (s *Server) authCookieName() string {
	name := strings.TrimSpace(s.cfg.AuthSessionCookieName)
	if name == "" {
		return "qs_session"
	}
	return name
}
