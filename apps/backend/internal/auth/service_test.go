package auth

import (
	"context"
	"errors"
	"testing"
	"time"
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
