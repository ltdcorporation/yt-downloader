package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func newTestService(t *testing.T, opts Options) *Service {
	t.Helper()
	store := &Store{backend: newMemoryBackend()}
	svc := NewService(store, opts)
	if svc == nil {
		t.Fatalf("expected non-nil auth service")
	}
	return svc
}

func TestRegisterLoginAuthenticateLogoutFlow(t *testing.T) {
	svc := newTestService(t, Options{
		SessionTTL:         2 * time.Hour,
		RememberSessionTTL: 72 * time.Hour,
		BcryptCost:         10,
	})

	ctx := context.Background()

	registerResult, err := svc.Register(ctx, RegisterInput{
		FullName:     "Senior Dev",
		Email:        "senior@example.com",
		Password:     "StrongPass123",
		KeepLoggedIn: true,
		ClientIP:     "127.0.0.1",
		UserAgent:    "unit-test",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if registerResult.AccessToken == "" {
		t.Fatalf("expected access token on register")
	}
	if registerResult.User.Email != "senior@example.com" {
		t.Fatalf("unexpected register email: %s", registerResult.User.Email)
	}
	if registerResult.ExpiresAt.Before(time.Now().UTC().Add(71 * time.Hour)) {
		t.Fatalf("expected remember-me expiration around 72h, got %s", registerResult.ExpiresAt)
	}

	sessionIdentity, err := svc.AuthenticateToken(ctx, registerResult.AccessToken)
	if err != nil {
		t.Fatalf("authenticate register token failed: %v", err)
	}
	if sessionIdentity.User.ID != registerResult.User.ID {
		t.Fatalf("unexpected session user id: %s", sessionIdentity.User.ID)
	}

	_, err = svc.Register(ctx, RegisterInput{
		FullName: "Another",
		Email:    "senior@example.com",
		Password: "StrongPass123",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	_, err = svc.Login(ctx, LoginInput{
		Email:    "senior@example.com",
		Password: "WrongPass123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}

	loginResult, err := svc.Login(ctx, LoginInput{
		Email:    "senior@example.com",
		Password: "StrongPass123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResult.ExpiresAt.Before(time.Now().UTC().Add(90*time.Minute)) || loginResult.ExpiresAt.After(time.Now().UTC().Add(3*time.Hour)) {
		t.Fatalf("expected login expiration around 2h, got %s", loginResult.ExpiresAt)
	}

	if err := svc.Logout(ctx, loginResult.AccessToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, err = svc.AuthenticateToken(ctx, loginResult.AccessToken)
	if !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked after logout, got %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newTestService(t, Options{})
	ctx := context.Background()

	tests := []struct {
		name  string
		input RegisterInput
	}{
		{
			name: "missing name",
			input: RegisterInput{
				Email:    "x@example.com",
				Password: "StrongPass123",
			},
		},
		{
			name: "invalid email",
			input: RegisterInput{
				FullName: "Tester",
				Email:    "bad-email",
				Password: "StrongPass123",
			},
		},
		{
			name: "weak password",
			input: RegisterInput{
				FullName: "Tester",
				Email:    "x@example.com",
				Password: "weakpass",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Register(ctx, tc.input)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestAuthenticateTokenErrors(t *testing.T) {
	svc := newTestService(t, Options{})
	ctx := context.Background()

	if _, err := svc.AuthenticateToken(ctx, ""); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken, got %v", err)
	}

	if _, err := svc.AuthenticateToken(ctx, "st_unknown"); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken for unknown token, got %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid input" {
		t.Fatalf("unexpected empty validation error message: %q", got)
	}
	if got := (&ValidationError{Message: "boom"}).Error(); got != "boom" {
		t.Fatalf("unexpected validation error message: %q", got)
	}
}

func TestNewService_DefaultFallbacks(t *testing.T) {
	svc := newTestService(t, Options{
		SessionTTL:         -time.Minute,
		RememberSessionTTL: -time.Minute,
		BcryptCost:         999,
		DummyPasswordHash:  "",
	})

	if svc.sessionTTL != defaultSessionTTL {
		t.Fatalf("expected default session TTL, got %s", svc.sessionTTL)
	}
	if svc.rememberSessionTTL != defaultRememberSessionTTL {
		t.Fatalf("expected default remember session TTL, got %s", svc.rememberSessionTTL)
	}
	if svc.bcryptCost != defaultBcryptCost {
		t.Fatalf("expected default bcrypt cost, got %d", svc.bcryptCost)
	}
	if svc.dummyPasswordHash != defaultDummyPasswordHash {
		t.Fatalf("expected default dummy hash")
	}
}

func TestService_LoginUnknownUser(t *testing.T) {
	svc := newTestService(t, Options{})
	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "unknown@example.com",
		Password: "StrongPass123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_RegisterAndLoginStoreErrors(t *testing.T) {
	ctx := context.Background()
	expected := errors.New("store exploded")

	registerStore := &Store{backend: &fakeBackend{
		createUserAndSessionFn: func(context.Context, User, Session) error {
			return expected
		},
	}}
	registerSvc := NewService(registerStore, Options{})
	_, err := registerSvc.Register(ctx, RegisterInput{
		FullName: "Tester",
		Email:    "tester@example.com",
		Password: "StrongPass123",
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected register error propagation, got %v", err)
	}

	hash, hashErr := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if hashErr != nil {
		t.Fatalf("failed to generate bcrypt hash for test: %v", hashErr)
	}
	loginStore := &Store{backend: &fakeBackend{
		getUserByEmailFn: func(context.Context, string) (User, error) {
			return User{ID: "usr_1", Email: "tester@example.com", PasswordHash: string(hash)}, nil
		},
		createSessionFn: func(context.Context, Session) error {
			return expected
		},
	}}
	loginSvc := NewService(loginStore, Options{})
	_, err = loginSvc.Login(ctx, LoginInput{
		Email:    "tester@example.com",
		Password: "password",
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected session create error propagation, got %v", err)
	}
}

func TestService_AuthenticateTokenEdgeCases(t *testing.T) {
	now := time.Now().UTC()

	t.Run("expired session", func(t *testing.T) {
		store := &Store{backend: &fakeBackend{
			getSessionByTokenHashFn: func(context.Context, string) (Session, error) {
				return Session{ID: "ses_1", UserID: "usr_1", ExpiresAt: now.Add(-time.Minute)}, nil
			},
		}}
		svc := NewService(store, Options{})
		svc.now = func() time.Time { return now }

		_, err := svc.AuthenticateToken(context.Background(), "st_test")
		if !errors.Is(err, ErrSessionExpired) {
			t.Fatalf("expected ErrSessionExpired, got %v", err)
		}
	})

	t.Run("revoked session", func(t *testing.T) {
		revoked := now.Add(-time.Minute)
		store := &Store{backend: &fakeBackend{
			getSessionByTokenHashFn: func(context.Context, string) (Session, error) {
				return Session{ID: "ses_1", UserID: "usr_1", ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}, nil
			},
		}}
		svc := NewService(store, Options{})
		svc.now = func() time.Time { return now }

		_, err := svc.AuthenticateToken(context.Background(), "st_test")
		if !errors.Is(err, ErrSessionRevoked) {
			t.Fatalf("expected ErrSessionRevoked, got %v", err)
		}
	})

	t.Run("user disappeared", func(t *testing.T) {
		store := &Store{backend: &fakeBackend{
			getSessionByTokenHashFn: func(context.Context, string) (Session, error) {
				return Session{ID: "ses_1", UserID: "usr_missing", ExpiresAt: now.Add(time.Hour)}, nil
			},
			getUserByIDFn: func(context.Context, string) (User, error) {
				return User{}, ErrUserNotFound
			},
		}}
		svc := NewService(store, Options{})
		svc.now = func() time.Time { return now }

		_, err := svc.AuthenticateToken(context.Background(), "st_test")
		if !errors.Is(err, ErrInvalidSessionToken) {
			t.Fatalf("expected ErrInvalidSessionToken, got %v", err)
		}
	})
}

func TestService_LogoutEdgeCases(t *testing.T) {
	svc := newTestService(t, Options{})
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("expected empty token logout to be no-op, got %v", err)
	}

	expected := errors.New("boom")
	store := &Store{backend: &fakeBackend{
		revokeSessionByTokenHashFn: func(context.Context, string, time.Time) error {
			return expected
		},
	}}
	svc = NewService(store, Options{})
	if err := svc.Logout(context.Background(), "st_abc"); !errors.Is(err, expected) {
		t.Fatalf("expected revoke error propagation, got %v", err)
	}

	store = &Store{backend: &fakeBackend{
		revokeSessionByTokenHashFn: func(context.Context, string, time.Time) error {
			return ErrSessionNotFound
		},
	}}
	svc = NewService(store, Options{})
	if err := svc.Logout(context.Background(), "st_abc"); err != nil {
		t.Fatalf("expected missing session logout to be nil, got %v", err)
	}
}

func TestNormalizationHelpers(t *testing.T) {
	if _, err := normalizeFullName("A"); err == nil {
		t.Fatalf("expected short full name validation error")
	}
	if _, err := normalizeFullName(strings.Repeat("x", 121)); err == nil {
		t.Fatalf("expected long full name validation error")
	}
	if fullName, err := normalizeFullName("  John   Doe  "); err != nil || fullName != "John Doe" {
		t.Fatalf("unexpected normalized full name: value=%q err=%v", fullName, err)
	}

	if _, err := normalizeEmail(""); err == nil {
		t.Fatalf("expected missing email validation error")
	}
	if _, err := normalizeEmail(strings.Repeat("a", 255) + "@x.com"); err == nil {
		t.Fatalf("expected long email validation error")
	}
	if _, err := normalizeEmail("invalid-email"); err == nil {
		t.Fatalf("expected invalid email validation error")
	}
	if email, err := normalizeEmail("  USER@Example.COM "); err != nil || email != "user@example.com" {
		t.Fatalf("unexpected normalized email: value=%q err=%v", email, err)
	}

	if err := validatePassword(""); err == nil {
		t.Fatalf("expected empty password validation error")
	}
	if err := validatePassword("short1"); err == nil {
		t.Fatalf("expected short password validation error")
	}
	if err := validatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatalf("expected long password validation error")
	}
	if err := validatePassword("alllettersonly"); err == nil {
		t.Fatalf("expected no-number validation error")
	}
	if err := validatePassword("1234567890"); err == nil {
		t.Fatalf("expected no-letter validation error")
	}
	if err := validatePassword("StrongPass123"); err != nil {
		t.Fatalf("expected valid password, got %v", err)
	}

	ua := normalizeUserAgent(strings.Repeat("a", 600))
	if len(ua) != 512 {
		t.Fatalf("expected truncated user-agent length 512, got %d", len(ua))
	}
	if normalizeUserAgent("short") != "short" {
		t.Fatalf("unexpected short user-agent normalization")
	}
}

func TestGenerateSessionTokenAndHash(t *testing.T) {
	token, tokenHash, err := generateSessionToken()
	if err != nil {
		t.Fatalf("generateSessionToken failed: %v", err)
	}
	if !strings.HasPrefix(token, "st_") {
		t.Fatalf("unexpected token prefix: %s", token)
	}
	if len(tokenHash) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(tokenHash))
	}
	if tokenHash != hashToken(token) {
		t.Fatalf("expected hashToken consistency")
	}

	result := buildAuthResult(User{ID: "usr", FullName: "Name", Email: "x@example.com", CreatedAt: time.Now().UTC()}, token, time.Now().UTC())
	if result.AccessToken == "" || result.TokenType != "Bearer" {
		t.Fatalf("unexpected auth result: %+v", result)
	}

	token2, hash2, err := generateSessionToken()
	if err != nil {
		t.Fatalf("second token generation failed: %v", err)
	}
	if token == token2 || hash2 == tokenHash {
		t.Fatalf("expected unique tokens/hashes across generations")
	}
}

func TestService_GuardsAndAdditionalErrorPaths(t *testing.T) {
	ctx := context.Background()

	var nilService *Service
	if _, err := nilService.Register(ctx, RegisterInput{}); err == nil {
		t.Fatalf("expected nil service register error")
	}
	if _, err := nilService.Login(ctx, LoginInput{}); err == nil {
		t.Fatalf("expected nil service login error")
	}
	if _, err := nilService.AuthenticateToken(ctx, "token"); err == nil {
		t.Fatalf("expected nil service authenticate error")
	}
	if err := nilService.Logout(ctx, "token"); err == nil {
		t.Fatalf("expected nil service logout error")
	}

	svc := newTestService(t, Options{})
	if _, err := svc.Login(ctx, LoginInput{Email: "bad-email", Password: "abc"}); err == nil {
		t.Fatalf("expected invalid email login error")
	}
	if _, err := svc.Login(ctx, LoginInput{Email: "x@example.com", Password: ""}); err == nil {
		t.Fatalf("expected missing password login error")
	}

	expected := errors.New("backend generic")
	storeWithErrors := &Store{backend: &fakeBackend{
		getUserByEmailFn: func(context.Context, string) (User, error) {
			return User{}, expected
		},
	}}
	svc = NewService(storeWithErrors, Options{})
	if _, err := svc.Login(ctx, LoginInput{Email: "x@example.com", Password: "password1"}); !errors.Is(err, expected) {
		t.Fatalf("expected getUserByEmail error propagation, got %v", err)
	}

	storeWithErrors = &Store{backend: &fakeBackend{
		getSessionByTokenHashFn: func(context.Context, string) (Session, error) {
			return Session{}, expected
		},
	}}
	svc = NewService(storeWithErrors, Options{})
	if _, err := svc.AuthenticateToken(ctx, "st_token"); !errors.Is(err, expected) {
		t.Fatalf("expected getSession error propagation, got %v", err)
	}

	storeWithErrors = &Store{backend: &fakeBackend{
		getSessionByTokenHashFn: func(context.Context, string) (Session, error) {
			return Session{ID: "ses", UserID: "usr", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		getUserByIDFn: func(context.Context, string) (User, error) {
			return User{}, expected
		},
	}}
	svc = NewService(storeWithErrors, Options{})
	if _, err := svc.AuthenticateToken(ctx, "st_token"); !errors.Is(err, expected) {
		t.Fatalf("expected getUserByID error propagation, got %v", err)
	}
}
