package auth

import (
	"context"
	"errors"
	"testing"
)

type fakeGoogleTokenVerifier struct {
	verifyFn func(ctx context.Context, rawIDToken string) (GoogleTokenClaims, error)
}

func (f fakeGoogleTokenVerifier) Verify(ctx context.Context, rawIDToken string) (GoogleTokenClaims, error) {
	if f.verifyFn != nil {
		return f.verifyFn(ctx, rawIDToken)
	}
	return GoogleTokenClaims{}, nil
}

func TestService_LoginWithGoogle_NewAndExistingIdentity(t *testing.T) {
	store := &Store{backend: newMemoryBackend()}
	svc := NewService(store, Options{
		GoogleTokenVerifier: fakeGoogleTokenVerifier{
			verifyFn: func(_ context.Context, rawIDToken string) (GoogleTokenClaims, error) {
				if rawIDToken == "id-token-1" {
					return GoogleTokenClaims{Subject: "sub_1", Email: "google@example.com", EmailVerified: true, FullName: "Google User"}, nil
				}
				return GoogleTokenClaims{Subject: "sub_1", Email: "google.updated@example.com", EmailVerified: true, FullName: "Google User Updated"}, nil
			},
		},
	})

	ctx := context.Background()
	first, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "id-token-1", KeepLoggedIn: true})
	if err != nil {
		t.Fatalf("first LoginWithGoogle failed: %v", err)
	}
	if first.AccessToken == "" {
		t.Fatalf("expected access token on first google login")
	}
	if first.User.Email != "google@example.com" {
		t.Fatalf("unexpected first user email: %s", first.User.Email)
	}

	second, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "id-token-2"})
	if err != nil {
		t.Fatalf("second LoginWithGoogle failed: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("expected same user id across same google subject, got first=%s second=%s", first.User.ID, second.User.ID)
	}

	bySubject, err := store.GetUserByGoogleSubject(ctx, "sub_1")
	if err != nil {
		t.Fatalf("GetUserByGoogleSubject failed: %v", err)
	}
	if bySubject.ID != first.User.ID {
		t.Fatalf("unexpected linked user id by subject: %s", bySubject.ID)
	}
}

func TestService_LoginWithGoogle_LinksExistingEmailUser(t *testing.T) {
	store := &Store{backend: newMemoryBackend()}
	svc := NewService(store, Options{
		GoogleTokenVerifier: fakeGoogleTokenVerifier{
			verifyFn: func(_ context.Context, _ string) (GoogleTokenClaims, error) {
				return GoogleTokenClaims{Subject: "sub_link", Email: "existing@example.com", EmailVerified: true, FullName: "Existing Name"}, nil
			},
		},
	})

	ctx := context.Background()
	registered, err := svc.Register(ctx, RegisterInput{FullName: "Existing User", Email: "existing@example.com", Password: "StrongPass123"})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	googleLogin, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "id-token-link"})
	if err != nil {
		t.Fatalf("google login failed: %v", err)
	}
	if googleLogin.User.ID != registered.User.ID {
		t.Fatalf("expected google login to link existing user id, got register=%s google=%s", registered.User.ID, googleLogin.User.ID)
	}

	linkedUser, err := store.GetUserByGoogleSubject(ctx, "sub_link")
	if err != nil {
		t.Fatalf("GetUserByGoogleSubject failed: %v", err)
	}
	if linkedUser.ID != registered.User.ID {
		t.Fatalf("unexpected linked user id: %s", linkedUser.ID)
	}
}

func TestService_LoginWithGoogle_ErrorCases(t *testing.T) {
	ctx := context.Background()

	t.Run("disabled when verifier missing", func(t *testing.T) {
		svc := NewService(&Store{backend: newMemoryBackend()}, Options{})
		_, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "abc"})
		if !errors.Is(err, ErrGoogleAuthDisabled) {
			t.Fatalf("expected ErrGoogleAuthDisabled, got %v", err)
		}
	})

	t.Run("token verifier error is propagated", func(t *testing.T) {
		svc := NewService(&Store{backend: newMemoryBackend()}, Options{
			GoogleTokenVerifier: fakeGoogleTokenVerifier{verifyFn: func(_ context.Context, _ string) (GoogleTokenClaims, error) {
				return GoogleTokenClaims{}, ErrGoogleTokenInvalid
			}},
		})
		_, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "abc"})
		if !errors.Is(err, ErrGoogleTokenInvalid) {
			t.Fatalf("expected ErrGoogleTokenInvalid, got %v", err)
		}
	})

	t.Run("email must be verified", func(t *testing.T) {
		svc := NewService(&Store{backend: newMemoryBackend()}, Options{
			GoogleTokenVerifier: fakeGoogleTokenVerifier{verifyFn: func(_ context.Context, _ string) (GoogleTokenClaims, error) {
				return GoogleTokenClaims{Subject: "sub", Email: "x@example.com", EmailVerified: false, FullName: "Name"}, nil
			}},
		})
		_, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "abc"})
		if !errors.Is(err, ErrGoogleEmailUnverified) {
			t.Fatalf("expected ErrGoogleEmailUnverified, got %v", err)
		}
	})

	t.Run("invalid payload claims", func(t *testing.T) {
		svc := NewService(&Store{backend: newMemoryBackend()}, Options{
			GoogleTokenVerifier: fakeGoogleTokenVerifier{verifyFn: func(_ context.Context, _ string) (GoogleTokenClaims, error) {
				return GoogleTokenClaims{Subject: "", Email: "not-email", EmailVerified: true}, nil
			}},
		})
		_, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "abc"})
		if !errors.Is(err, ErrGoogleTokenInvalid) {
			t.Fatalf("expected ErrGoogleTokenInvalid, got %v", err)
		}
	})

	t.Run("identity conflict from store", func(t *testing.T) {
		store := &Store{backend: &fakeBackend{
			getUserByGoogleSubjectFn: func(context.Context, string) (User, error) {
				return User{}, ErrUserNotFound
			},
			getUserByEmailFn: func(context.Context, string) (User, error) {
				return User{ID: "usr_1", Email: "x@example.com", FullName: "X"}, nil
			},
			upsertGoogleIdentityFn: func(context.Context, GoogleIdentity) error {
				return ErrGoogleIdentityConflict
			},
		}}
		svc := NewService(store, Options{
			GoogleTokenVerifier: fakeGoogleTokenVerifier{verifyFn: func(_ context.Context, _ string) (GoogleTokenClaims, error) {
				return GoogleTokenClaims{Subject: "sub", Email: "x@example.com", EmailVerified: true, FullName: "X User"}, nil
			}},
		})
		_, err := svc.LoginWithGoogle(ctx, GoogleLoginInput{IDToken: "abc"})
		if !errors.Is(err, ErrGoogleIdentityConflict) {
			t.Fatalf("expected ErrGoogleIdentityConflict, got %v", err)
		}
	})
}
