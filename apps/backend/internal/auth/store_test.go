package auth

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

type fakeBackend struct {
	createUserFn               func(ctx context.Context, user User) error
	createUserAndSessionFn     func(ctx context.Context, user User, session Session) error
	getUserByEmailFn           func(ctx context.Context, email string) (User, error)
	getUserByIDFn              func(ctx context.Context, userID string) (User, error)
	createSessionFn            func(ctx context.Context, session Session) error
	getSessionByTokenHashFn    func(ctx context.Context, tokenHash string) (Session, error)
	touchSessionFn             func(ctx context.Context, tokenHash string, touchedAt time.Time) error
	revokeSessionByTokenHashFn func(ctx context.Context, tokenHash string, revokedAt time.Time) error
	closeFn                    func() error
}

func (f *fakeBackend) CreateUser(ctx context.Context, user User) error {
	if f.createUserFn != nil {
		return f.createUserFn(ctx, user)
	}
	return nil
}

func (f *fakeBackend) CreateUserAndSession(ctx context.Context, user User, session Session) error {
	if f.createUserAndSessionFn != nil {
		return f.createUserAndSessionFn(ctx, user, session)
	}
	return nil
}

func (f *fakeBackend) GetUserByEmail(ctx context.Context, email string) (User, error) {
	if f.getUserByEmailFn != nil {
		return f.getUserByEmailFn(ctx, email)
	}
	return User{}, nil
}

func (f *fakeBackend) GetUserByID(ctx context.Context, userID string) (User, error) {
	if f.getUserByIDFn != nil {
		return f.getUserByIDFn(ctx, userID)
	}
	return User{}, nil
}

func (f *fakeBackend) CreateSession(ctx context.Context, session Session) error {
	if f.createSessionFn != nil {
		return f.createSessionFn(ctx, session)
	}
	return nil
}

func (f *fakeBackend) GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	if f.getSessionByTokenHashFn != nil {
		return f.getSessionByTokenHashFn(ctx, tokenHash)
	}
	return Session{}, nil
}

func (f *fakeBackend) TouchSession(ctx context.Context, tokenHash string, touchedAt time.Time) error {
	if f.touchSessionFn != nil {
		return f.touchSessionFn(ctx, tokenHash, touchedAt)
	}
	return nil
}

func (f *fakeBackend) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if f.revokeSessionByTokenHashFn != nil {
		return f.revokeSessionByTokenHashFn(ctx, tokenHash, revokedAt)
	}
	return nil
}

func (f *fakeBackend) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func TestNewStore_SelectsBackend(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	memoryStore := NewStore(config.Config{}, logger)
	if memoryStore == nil {
		t.Fatalf("expected non-nil store")
	}
	if _, ok := memoryStore.backend.(*memoryBackend); !ok {
		t.Fatalf("expected memory backend when POSTGRES_DSN is empty")
	}

	postgresStore := NewStore(config.Config{PostgresDSN: "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"}, logger)
	if postgresStore == nil {
		t.Fatalf("expected non-nil postgres store")
	}
	if _, ok := postgresStore.backend.(*postgresBackend); !ok {
		t.Fatalf("expected postgres backend when POSTGRES_DSN is set")
	}
	_ = postgresStore.Close()
}

func TestStore_NilSafetyGuards(t *testing.T) {
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil store Close should be nil error, got %v", err)
	}

	s := &Store{}
	ctx := context.Background()

	if err := s.CreateUser(ctx, User{}); err == nil {
		t.Fatalf("expected error for uninitialized CreateUser")
	}
	if err := s.CreateUserAndSession(ctx, User{}, Session{}); err == nil {
		t.Fatalf("expected error for uninitialized CreateUserAndSession")
	}
	if _, err := s.GetUserByEmail(ctx, "a@example.com"); err == nil {
		t.Fatalf("expected error for uninitialized GetUserByEmail")
	}
	if _, err := s.GetUserByID(ctx, "usr"); err == nil {
		t.Fatalf("expected error for uninitialized GetUserByID")
	}
	if err := s.CreateSession(ctx, Session{}); err == nil {
		t.Fatalf("expected error for uninitialized CreateSession")
	}
	if _, err := s.GetSessionByTokenHash(ctx, "hash"); err == nil {
		t.Fatalf("expected error for uninitialized GetSessionByTokenHash")
	}
	if err := s.TouchSession(ctx, "hash", time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized TouchSession")
	}
	if err := s.RevokeSessionByTokenHash(ctx, "hash", time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized RevokeSessionByTokenHash")
	}
}

func TestStore_WrapperForwardingAndNormalization(t *testing.T) {
	ctx := context.Background()
	capturedEmail := ""
	capturedUserID := ""
	capturedTokenHash := ""
	capturedTouch := time.Time{}
	capturedRevoke := time.Time{}

	backend := &fakeBackend{
		createUserFn: func(_ context.Context, user User) error {
			if user.Email == "" {
				t.Fatalf("expected non-empty user email")
			}
			return nil
		},
		createUserAndSessionFn: func(_ context.Context, user User, session Session) error {
			if user.ID == "" || session.ID == "" {
				t.Fatalf("expected user/session ids")
			}
			return nil
		},
		getUserByEmailFn: func(_ context.Context, email string) (User, error) {
			capturedEmail = email
			return User{Email: email}, nil
		},
		getUserByIDFn: func(_ context.Context, userID string) (User, error) {
			capturedUserID = userID
			return User{ID: userID}, nil
		},
		createSessionFn: func(_ context.Context, session Session) error {
			if session.TokenHash == "" {
				t.Fatalf("expected token hash")
			}
			return nil
		},
		getSessionByTokenHashFn: func(_ context.Context, tokenHash string) (Session, error) {
			capturedTokenHash = tokenHash
			return Session{TokenHash: tokenHash}, nil
		},
		touchSessionFn: func(_ context.Context, tokenHash string, touchedAt time.Time) error {
			capturedTokenHash = tokenHash
			capturedTouch = touchedAt
			return nil
		},
		revokeSessionByTokenHashFn: func(_ context.Context, tokenHash string, revokedAt time.Time) error {
			capturedTokenHash = tokenHash
			capturedRevoke = revokedAt
			return nil
		},
		closeFn: func() error {
			return nil
		},
	}

	s := &Store{backend: backend}

	if err := s.CreateUser(ctx, User{Email: "  user@example.com  "}); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if err := s.CreateUserAndSession(ctx, User{ID: "usr_1"}, Session{ID: "ses_1"}); err != nil {
		t.Fatalf("CreateUserAndSession returned error: %v", err)
	}

	if _, err := s.GetUserByEmail(ctx, "  USER@Example.COM "); err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if capturedEmail != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", capturedEmail)
	}

	if _, err := s.GetUserByID(ctx, "  usr_abc "); err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if capturedUserID != "usr_abc" {
		t.Fatalf("expected normalized user id, got %q", capturedUserID)
	}

	if err := s.CreateSession(ctx, Session{TokenHash: "abc"}); err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if _, err := s.GetSessionByTokenHash(ctx, "  ABcDEF "); err != nil {
		t.Fatalf("GetSessionByTokenHash returned error: %v", err)
	}
	if capturedTokenHash != "abcdef" {
		t.Fatalf("expected normalized token hash, got %q", capturedTokenHash)
	}

	now := time.Now().UTC()
	if err := s.TouchSession(ctx, "  ABcDEF ", now); err != nil {
		t.Fatalf("TouchSession returned error: %v", err)
	}
	if capturedTokenHash != "abcdef" || !capturedTouch.Equal(now) {
		t.Fatalf("unexpected touch capture token=%q touched=%s", capturedTokenHash, capturedTouch)
	}

	if err := s.RevokeSessionByTokenHash(ctx, "  ABcDEF ", now); err != nil {
		t.Fatalf("RevokeSessionByTokenHash returned error: %v", err)
	}
	if capturedTokenHash != "abcdef" || !capturedRevoke.Equal(now) {
		t.Fatalf("unexpected revoke capture token=%q revoked=%s", capturedTokenHash, capturedRevoke)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestStore_WrapperErrorPropagation(t *testing.T) {
	expectedErr := errors.New("backend boom")
	s := &Store{backend: &fakeBackend{
		createUserFn: func(context.Context, User) error { return expectedErr },
	}}

	err := s.CreateUser(context.Background(), User{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}
