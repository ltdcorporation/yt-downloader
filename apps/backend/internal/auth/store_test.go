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
	createUserFn                         func(ctx context.Context, user User) error
	createUserAndSessionFn               func(ctx context.Context, user User, session Session) error
	createUserSessionAndGoogleIdentityFn func(ctx context.Context, user User, session Session, identity GoogleIdentity) error
	getUserByEmailFn                     func(ctx context.Context, email string) (User, error)
	getUserByIDFn                        func(ctx context.Context, userID string) (User, error)
	updateUserFullNameFn                 func(ctx context.Context, userID, fullName string, updatedAt time.Time) (User, error)
	updateUserAvatarURLFn                func(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (User, error)
	updateUserByAdminFn                  func(ctx context.Context, userID string, patch AdminUserPatch, updatedAt time.Time) (User, error)
	getUserByGoogleSubjectFn             func(ctx context.Context, googleSubject string) (User, error)
	listUsersFn                          func(ctx context.Context, limit, offset int) ([]User, int, error)
	getUserStatsFn                       func(ctx context.Context, now time.Time) (UserStats, error)
	createSessionFn                      func(ctx context.Context, session Session) error
	getSessionByTokenHashFn              func(ctx context.Context, tokenHash string) (Session, error)
	touchSessionFn                       func(ctx context.Context, tokenHash string, touchedAt time.Time) error
	revokeSessionByTokenHashFn           func(ctx context.Context, tokenHash string, revokedAt time.Time) error
	upsertGoogleIdentityFn               func(ctx context.Context, identity GoogleIdentity) error
	closeFn                              func() error
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

func (f *fakeBackend) CreateUserSessionAndGoogleIdentity(ctx context.Context, user User, session Session, identity GoogleIdentity) error {
	if f.createUserSessionAndGoogleIdentityFn != nil {
		return f.createUserSessionAndGoogleIdentityFn(ctx, user, session, identity)
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

func (f *fakeBackend) UpdateUserFullName(ctx context.Context, userID, fullName string, updatedAt time.Time) (User, error) {
	if f.updateUserFullNameFn != nil {
		return f.updateUserFullNameFn(ctx, userID, fullName, updatedAt)
	}
	return User{ID: userID, FullName: fullName, UpdatedAt: updatedAt}, nil
}

func (f *fakeBackend) UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (User, error) {
	if f.updateUserAvatarURLFn != nil {
		return f.updateUserAvatarURLFn(ctx, userID, avatarURL, updatedAt)
	}
	return User{ID: userID, AvatarURL: avatarURL, UpdatedAt: updatedAt}, nil
}

func (f *fakeBackend) UpdateUserByAdmin(ctx context.Context, userID string, patch AdminUserPatch, updatedAt time.Time) (User, error) {
	if f.updateUserByAdminFn != nil {
		return f.updateUserByAdminFn(ctx, userID, patch, updatedAt)
	}
	return User{ID: userID, UpdatedAt: updatedAt}, nil
}

func (f *fakeBackend) GetUserByGoogleSubject(ctx context.Context, googleSubject string) (User, error) {
	if f.getUserByGoogleSubjectFn != nil {
		return f.getUserByGoogleSubjectFn(ctx, googleSubject)
	}
	return User{}, nil
}

func (f *fakeBackend) ListUsers(ctx context.Context, limit int, offset int) ([]User, int, error) {
	if f.listUsersFn != nil {
		return f.listUsersFn(ctx, limit, offset)
	}
	return nil, 0, nil
}

func (f *fakeBackend) GetUserStats(ctx context.Context, now time.Time) (UserStats, error) {
	if f.getUserStatsFn != nil {
		return f.getUserStatsFn(ctx, now)
	}
	return UserStats{}, nil
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

func (f *fakeBackend) UpsertGoogleIdentity(ctx context.Context, identity GoogleIdentity) error {
	if f.upsertGoogleIdentityFn != nil {
		return f.upsertGoogleIdentityFn(ctx, identity)
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
	if err := s.CreateUserSessionAndGoogleIdentity(ctx, User{}, Session{}, GoogleIdentity{}); err == nil {
		t.Fatalf("expected error for uninitialized CreateUserSessionAndGoogleIdentity")
	}
	if _, err := s.GetUserByEmail(ctx, "a@example.com"); err == nil {
		t.Fatalf("expected error for uninitialized GetUserByEmail")
	}
	if _, err := s.GetUserByID(ctx, "usr"); err == nil {
		t.Fatalf("expected error for uninitialized GetUserByID")
	}
	if _, err := s.UpdateUserFullName(ctx, "usr", "Name", time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized UpdateUserFullName")
	}
	if _, err := s.UpdateUserAvatarURL(ctx, "usr", "https://avatar.indobang.site/a.webp", time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized UpdateUserAvatarURL")
	}
	if _, err := s.UpdateUserByAdmin(ctx, "usr", AdminUserPatch{}, time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized UpdateUserByAdmin")
	}
	if _, err := s.GetUserByGoogleSubject(ctx, "sub"); err == nil {
		t.Fatalf("expected error for uninitialized GetUserByGoogleSubject")
	}
	if _, err := s.GetUserStats(ctx, time.Now()); err == nil {
		t.Fatalf("expected error for uninitialized GetUserStats")
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
	if err := s.UpsertGoogleIdentity(ctx, GoogleIdentity{}); err == nil {
		t.Fatalf("expected error for uninitialized UpsertGoogleIdentity")
	}
}

func TestStore_WrapperForwardingAndNormalization(t *testing.T) {
	ctx := context.Background()
	capturedEmail := ""
	capturedUserID := ""
	capturedUpdatedUserID := ""
	capturedUpdatedFullName := ""
	capturedUpdatedAvatarURL := ""
	capturedAdminPatch := AdminUserPatch{}
	capturedUpdatedAt := time.Time{}
	capturedTokenHash := ""
	capturedTouch := time.Time{}
	capturedRevoke := time.Time{}
	capturedGoogleSubject := ""
	capturedStatsNow := time.Time{}
	capturedIdentity := GoogleIdentity{}

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
		createUserSessionAndGoogleIdentityFn: func(_ context.Context, user User, session Session, identity GoogleIdentity) error {
			if user.ID == "" || session.ID == "" || identity.GoogleSubject == "" {
				t.Fatalf("expected user/session/google identity fields")
			}
			capturedIdentity = identity
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
		updateUserFullNameFn: func(_ context.Context, userID, fullName string, updatedAt time.Time) (User, error) {
			capturedUpdatedUserID = userID
			capturedUpdatedFullName = fullName
			capturedUpdatedAt = updatedAt
			return User{ID: userID, FullName: fullName, UpdatedAt: updatedAt}, nil
		},
		updateUserAvatarURLFn: func(_ context.Context, userID, avatarURL string, updatedAt time.Time) (User, error) {
			capturedUpdatedUserID = userID
			capturedUpdatedAvatarURL = avatarURL
			capturedUpdatedAt = updatedAt
			return User{ID: userID, AvatarURL: avatarURL, UpdatedAt: updatedAt}, nil
		},
		updateUserByAdminFn: func(_ context.Context, userID string, patch AdminUserPatch, updatedAt time.Time) (User, error) {
			capturedUpdatedUserID = userID
			capturedAdminPatch = patch
			capturedUpdatedAt = updatedAt
			return User{ID: userID, UpdatedAt: updatedAt}, nil
		},
		getUserByGoogleSubjectFn: func(_ context.Context, googleSubject string) (User, error) {
			capturedGoogleSubject = googleSubject
			return User{ID: "usr_google"}, nil
		},
		getUserStatsFn: func(_ context.Context, now time.Time) (UserStats, error) {
			capturedStatsNow = now
			return UserStats{TotalUsers: 9, ActivePaidUsers: 4}, nil
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
		upsertGoogleIdentityFn: func(_ context.Context, identity GoogleIdentity) error {
			capturedIdentity = identity
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
	if err := s.CreateUserSessionAndGoogleIdentity(ctx, User{ID: "usr_2"}, Session{ID: "ses_2"}, GoogleIdentity{GoogleSubject: "sub_1", Email: "USER@Example.COM"}); err != nil {
		t.Fatalf("CreateUserSessionAndGoogleIdentity returned error: %v", err)
	}
	if capturedIdentity.GoogleSubject != "sub_1" || capturedIdentity.Email != "user@example.com" {
		t.Fatalf("unexpected normalized google identity: %+v", capturedIdentity)
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

	nowForUpdate := time.Now().UTC()
	if _, err := s.UpdateUserFullName(ctx, "  usr_abc ", "  John Doe  ", nowForUpdate); err != nil {
		t.Fatalf("UpdateUserFullName returned error: %v", err)
	}
	if capturedUpdatedUserID != "usr_abc" {
		t.Fatalf("expected normalized update user id, got %q", capturedUpdatedUserID)
	}
	if capturedUpdatedFullName != "John Doe" {
		t.Fatalf("expected normalized update full name, got %q", capturedUpdatedFullName)
	}
	if !capturedUpdatedAt.Equal(nowForUpdate) {
		t.Fatalf("expected forwarded updatedAt, got %s want %s", capturedUpdatedAt, nowForUpdate)
	}

	if _, err := s.UpdateUserAvatarURL(ctx, "  usr_abc ", "  https://avatar.indobang.site/avatars/usr_abc/new.webp  ", nowForUpdate); err != nil {
		t.Fatalf("UpdateUserAvatarURL returned error: %v", err)
	}
	if capturedUpdatedUserID != "usr_abc" {
		t.Fatalf("expected normalized avatar update user id, got %q", capturedUpdatedUserID)
	}
	if capturedUpdatedAvatarURL != "https://avatar.indobang.site/avatars/usr_abc/new.webp" {
		t.Fatalf("expected normalized avatar url, got %q", capturedUpdatedAvatarURL)
	}
	if !capturedUpdatedAt.Equal(nowForUpdate) {
		t.Fatalf("expected avatar updatedAt forwarding, got %s want %s", capturedUpdatedAt, nowForUpdate)
	}

	role := RoleAdmin
	plan := PlanMonthly
	planExpiresAt := nowForUpdate.Add(30 * 24 * time.Hour)
	if _, err := s.UpdateUserByAdmin(ctx, "  usr_abc ", AdminUserPatch{
		Role:             &role,
		Plan:             &plan,
		PlanExpiresAtSet: true,
		PlanExpiresAt:    &planExpiresAt,
	}, nowForUpdate); err != nil {
		t.Fatalf("UpdateUserByAdmin returned error: %v", err)
	}
	if capturedUpdatedUserID != "usr_abc" {
		t.Fatalf("expected normalized admin update user id, got %q", capturedUpdatedUserID)
	}
	if capturedAdminPatch.Role == nil || *capturedAdminPatch.Role != RoleAdmin {
		t.Fatalf("expected admin role patch, got %+v", capturedAdminPatch)
	}
	if capturedAdminPatch.Plan == nil || *capturedAdminPatch.Plan != PlanMonthly {
		t.Fatalf("expected monthly plan patch, got %+v", capturedAdminPatch)
	}
	if !capturedAdminPatch.PlanExpiresAtSet || capturedAdminPatch.PlanExpiresAt == nil || !capturedAdminPatch.PlanExpiresAt.Equal(planExpiresAt.UTC()) {
		t.Fatalf("expected plan expires patch forwarding, got %+v", capturedAdminPatch)
	}

	if _, err := s.GetUserByGoogleSubject(ctx, "  sub_abc  "); err != nil {
		t.Fatalf("GetUserByGoogleSubject returned error: %v", err)
	}
	if capturedGoogleSubject != "sub_abc" {
		t.Fatalf("expected normalized google subject, got %q", capturedGoogleSubject)
	}

	statsNow := time.Now().UTC().Truncate(time.Second)
	stats, err := s.GetUserStats(ctx, statsNow)
	if err != nil {
		t.Fatalf("GetUserStats returned error: %v", err)
	}
	if capturedStatsNow.IsZero() || !capturedStatsNow.Equal(statsNow) {
		t.Fatalf("expected stats now forwarding, got %s want %s", capturedStatsNow, statsNow)
	}
	if stats.TotalUsers != 9 || stats.ActivePaidUsers != 4 {
		t.Fatalf("unexpected stats payload: %+v", stats)
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

	if err := s.UpsertGoogleIdentity(ctx, GoogleIdentity{GoogleSubject: "  sub_new ", Email: "  NEW@EXAMPLE.COM ", UserID: "usr_1"}); err != nil {
		t.Fatalf("UpsertGoogleIdentity returned error: %v", err)
	}
	if capturedIdentity.GoogleSubject != "sub_new" || capturedIdentity.Email != "new@example.com" {
		t.Fatalf("unexpected upsert google identity payload: %+v", capturedIdentity)
	}

	if _, err := s.UpdateUserByAdmin(ctx, "", AdminUserPatch{Role: &role}, nowForUpdate); err == nil {
		t.Fatalf("expected validation error for empty user id")
	}
	if _, err := s.UpdateUserByAdmin(ctx, "usr_abc", AdminUserPatch{}, nowForUpdate); err == nil {
		t.Fatalf("expected validation error for empty admin patch")
	}
	blankName := "   "
	if _, err := s.UpdateUserByAdmin(ctx, "usr_abc", AdminUserPatch{FullName: &blankName}, nowForUpdate); err == nil {
		t.Fatalf("expected validation error for blank full_name")
	}
	invalidRole := Role("owner")
	if _, err := s.UpdateUserByAdmin(ctx, "usr_abc", AdminUserPatch{Role: &invalidRole}, nowForUpdate); err == nil {
		t.Fatalf("expected validation error for invalid role")
	}
	invalidPlan := Plan("yearly")
	if _, err := s.UpdateUserByAdmin(ctx, "usr_abc", AdminUserPatch{Plan: &invalidPlan}, nowForUpdate); err == nil {
		t.Fatalf("expected validation error for invalid plan")
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
