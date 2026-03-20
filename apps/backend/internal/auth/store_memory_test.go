package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryBackend_UserAndSessionLifecycle(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	if err := backend.Close(); err != nil {
		t.Fatalf("memory Close should not error: %v", err)
	}

	user := User{ID: "usr_1", Email: "user@example.com", FullName: "User"}
	if err := backend.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if err := backend.CreateUser(ctx, user); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	if _, err := backend.GetUserByEmail(ctx, "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if _, err := backend.GetUserByID(ctx, "usr_missing"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound by id, got %v", err)
	}

	gotUser, err := backend.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if gotUser.ID != user.ID {
		t.Fatalf("unexpected user id: %s", gotUser.ID)
	}

	session := Session{ID: "ses_1", UserID: user.ID, TokenHash: "hash_1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := backend.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if err := backend.CreateSession(ctx, session); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken duplicate session, got %v", err)
	}

	if _, err := backend.GetSessionByTokenHash(ctx, "missing_hash"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	now := time.Now().UTC()
	if err := backend.TouchSession(ctx, session.TokenHash, now); err != nil {
		t.Fatalf("TouchSession failed: %v", err)
	}
	if err := backend.TouchSession(ctx, "missing_hash", now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound touch missing, got %v", err)
	}

	if err := backend.RevokeSessionByTokenHash(ctx, session.TokenHash, now); err != nil {
		t.Fatalf("RevokeSessionByTokenHash failed: %v", err)
	}
	if err := backend.RevokeSessionByTokenHash(ctx, "missing_hash", now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound revoke missing, got %v", err)
	}

	gotSession, err := backend.GetSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if gotSession.LastSeenAt == nil {
		t.Fatalf("expected LastSeenAt to be set")
	}
	if gotSession.RevokedAt == nil {
		t.Fatalf("expected RevokedAt to be set")
	}
}

func TestMemoryBackend_CreateUserAndSession(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()

	user := User{ID: "usr_1", Email: "u@example.com"}
	session := Session{ID: "ses_1", UserID: user.ID, TokenHash: "hash_1"}

	if err := backend.CreateUserAndSession(ctx, user, session); err != nil {
		t.Fatalf("CreateUserAndSession failed: %v", err)
	}

	if err := backend.CreateUserAndSession(ctx, User{ID: "usr_2", Email: user.Email}, Session{ID: "ses_2", UserID: "usr_2", TokenHash: "hash_2"}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken in transaction-style create, got %v", err)
	}

	if err := backend.CreateUserAndSession(ctx, User{ID: "usr_3", Email: "other@example.com"}, Session{ID: "ses_3", UserID: "usr_3", TokenHash: session.TokenHash}); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken in transaction-style create, got %v", err)
	}
}

func TestMemoryBackend_GoogleIdentityFlows(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC()

	user := User{ID: "usr_1", Email: "google@example.com", FullName: "Google User"}
	if err := backend.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	identity := GoogleIdentity{
		UserID:        user.ID,
		GoogleSubject: "sub_1",
		Email:         user.Email,
		FullName:      user.FullName,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := backend.UpsertGoogleIdentity(ctx, identity); err != nil {
		t.Fatalf("UpsertGoogleIdentity failed: %v", err)
	}

	resolvedUser, err := backend.GetUserByGoogleSubject(ctx, identity.GoogleSubject)
	if err != nil {
		t.Fatalf("GetUserByGoogleSubject failed: %v", err)
	}
	if resolvedUser.ID != user.ID {
		t.Fatalf("unexpected user by google subject: %s", resolvedUser.ID)
	}

	if err := backend.UpsertGoogleIdentity(ctx, GoogleIdentity{
		UserID:        user.ID,
		GoogleSubject: identity.GoogleSubject,
		Email:         "updated@example.com",
		FullName:      "Updated Name",
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("google identity update should succeed: %v", err)
	}

	otherUser := User{ID: "usr_2", Email: "other@example.com", FullName: "Other User"}
	if err := backend.CreateUser(ctx, otherUser); err != nil {
		t.Fatalf("CreateUser other failed: %v", err)
	}

	if err := backend.UpsertGoogleIdentity(ctx, GoogleIdentity{
		UserID:        otherUser.ID,
		GoogleSubject: identity.GoogleSubject,
		Email:         otherUser.Email,
		FullName:      otherUser.FullName,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict by duplicate subject, got %v", err)
	}

	if err := backend.UpsertGoogleIdentity(ctx, GoogleIdentity{
		UserID:        user.ID,
		GoogleSubject: "sub_2",
		Email:         user.Email,
		FullName:      user.FullName,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict by duplicate user_id mapping, got %v", err)
	}

	if _, err := backend.GetUserByGoogleSubject(ctx, "sub_missing"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for missing subject, got %v", err)
	}

	if err := backend.UpsertGoogleIdentity(ctx, GoogleIdentity{
		UserID:        "usr_missing",
		GoogleSubject: "sub_missing",
		Email:         "missing@example.com",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for missing user upsert, got %v", err)
	}
}

func TestMemoryBackend_CreateUserSessionAndGoogleIdentity(t *testing.T) {
	backend := newMemoryBackend()
	ctx := context.Background()
	now := time.Now().UTC()

	user := User{ID: "usr_1", Email: "google@example.com", FullName: "Google User"}
	session := Session{ID: "ses_1", UserID: user.ID, TokenHash: "hash_1"}
	identity := GoogleIdentity{UserID: user.ID, GoogleSubject: "sub_1", Email: user.Email, FullName: user.FullName, CreatedAt: now, UpdatedAt: now, EmailVerified: true}

	if err := backend.CreateUserSessionAndGoogleIdentity(ctx, user, session, identity); err != nil {
		t.Fatalf("CreateUserSessionAndGoogleIdentity failed: %v", err)
	}

	if err := backend.CreateUserSessionAndGoogleIdentity(ctx, User{ID: "usr_2", Email: user.Email}, Session{ID: "ses_2", UserID: "usr_2", TokenHash: "hash_2"}, GoogleIdentity{UserID: "usr_2", GoogleSubject: "sub_2", Email: user.Email}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	if err := backend.CreateUserSessionAndGoogleIdentity(ctx, User{ID: "usr_3", Email: "third@example.com"}, Session{ID: "ses_3", UserID: "usr_3", TokenHash: session.TokenHash}, GoogleIdentity{UserID: "usr_3", GoogleSubject: "sub_3", Email: "third@example.com"}); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken, got %v", err)
	}

	if err := backend.CreateUserSessionAndGoogleIdentity(ctx, User{ID: "usr_4", Email: "fourth@example.com"}, Session{ID: "ses_4", UserID: "usr_4", TokenHash: "hash_4"}, GoogleIdentity{UserID: "usr_4", GoogleSubject: identity.GoogleSubject, Email: "fourth@example.com"}); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict, got %v", err)
	}
}

func TestCopySession(t *testing.T) {
	revokedAt := time.Now().UTC().Add(-time.Hour)
	lastSeenAt := time.Now().UTC()

	in := Session{
		ID:         "ses_1",
		RevokedAt:  &revokedAt,
		LastSeenAt: &lastSeenAt,
	}
	out := copySession(in)

	if out.RevokedAt == nil || out.LastSeenAt == nil {
		t.Fatalf("expected copied pointers")
	}
	if out.RevokedAt == in.RevokedAt || out.LastSeenAt == in.LastSeenAt {
		t.Fatalf("expected deep copy for pointer fields")
	}

	outNil := copySession(Session{ID: "ses_nil"})
	if outNil.RevokedAt != nil || outNil.LastSeenAt != nil {
		t.Fatalf("expected nil pointers to remain nil")
	}
}
