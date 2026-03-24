package auth

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
)

func newMockPostgresBackend(t *testing.T) (*postgresBackend, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	backend := &postgresBackend{db: db}
	cleanup := func() { _ = db.Close() }
	return backend, mock, cleanup
}

func q(query string) string {
	return regexp.QuoteMeta(query)
}

func expectSchemaExec(mock sqlmock.Sqlmock) {
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS auth_users").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_users_email").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_users ADD COLUMN IF NOT EXISTS avatar_url").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS auth_sessions").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS revoked_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS last_seen_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS client_ip").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS user_agent").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS keep_logged_in").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_token_hash").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS auth_google_identities").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS full_name").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS picture_url").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS email_verified").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS created_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS updated_at").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_google_identities_user_id").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS idx_auth_google_identities_email").WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestPostgresBackend_EnsureSchema(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()

	expectSchemaExec(mock)
	if err := backend.ensureSchema(context.Background()); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}
	if err := backend.ensureSchema(context.Background()); err != nil {
		t.Fatalf("ensureSchema second call failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_EnsureSchemaError(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS auth_users").WillReturnError(errors.New("schema boom"))
	if err := backend.ensureSchema(context.Background()); err == nil {
		t.Fatalf("expected ensureSchema error")
	}
}

func TestPostgresBackend_CreateUser(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	user := User{ID: "usr_1", FullName: "User", Email: "user@example.com", PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}

	mock.ExpectExec("INSERT INTO auth_users").
		WithArgs(user.ID, user.FullName, user.Email, nil, user.PasswordHash, user.CreatedAt, user.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := backend.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	mock.ExpectExec("INSERT INTO auth_users").WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_users_email"})
	if err := backend.CreateUser(context.Background(), user); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	mock.ExpectExec("INSERT INTO auth_users").WillReturnError(errors.New("db fail"))
	if err := backend.CreateUser(context.Background(), user); err == nil {
		t.Fatalf("expected generic db error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_CreateUserAndSession(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	now := time.Now().UTC()
	user := User{ID: "usr_1", FullName: "User", Email: "user@example.com", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	session := Session{ID: "ses_1", UserID: user.ID, TokenHash: "token_hash", CreatedAt: now, ExpiresAt: now.Add(time.Hour), ClientIP: "127.0.0.1", UserAgent: "ua", KeepLoggedIn: true}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := backend.CreateUserAndSession(context.Background(), user, session); err != nil {
		t.Fatalf("CreateUserAndSession failed: %v", err)
	}

	mock.ExpectBegin().WillReturnError(errors.New("begin fail"))
	if err := backend.CreateUserAndSession(context.Background(), user, session); err == nil {
		t.Fatalf("expected begin error")
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_users_email"})
	mock.ExpectRollback()
	if err := backend.CreateUserAndSession(context.Background(), user, session); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_sessions_token_hash"})
	mock.ExpectRollback()
	if err := backend.CreateUserAndSession(context.Background(), user, session); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit fail"))
	if err := backend.CreateUserAndSession(context.Background(), user, session); err == nil {
		t.Fatalf("expected commit error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_CreateUserSessionAndGoogleIdentity(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	now := time.Now().UTC()
	user := User{ID: "usr_1", FullName: "Google User", Email: "google@example.com", PasswordHash: "hash", CreatedAt: now, UpdatedAt: now}
	session := Session{ID: "ses_1", UserID: user.ID, TokenHash: "token_hash", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	identity := GoogleIdentity{UserID: user.ID, GoogleSubject: "sub_1", Email: user.Email, FullName: user.FullName, EmailVerified: true, CreatedAt: now, UpdatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_google_identities").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := backend.CreateUserSessionAndGoogleIdentity(context.Background(), user, session, identity); err != nil {
		t.Fatalf("CreateUserSessionAndGoogleIdentity failed: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO auth_users").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO auth_google_identities").WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_google_identities_user_id"})
	mock.ExpectRollback()
	if err := backend.CreateUserSessionAndGoogleIdentity(context.Background(), user, session, identity); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_GetUserByEmailByIDAndGoogleSubject(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	now := time.Now().UTC()
	rowColumns := []string{"id", "full_name", "email", "avatar_url", "password_hash", "created_at", "updated_at"}
	queryByEmail := q(`SELECT id, full_name, email, avatar_url, password_hash, created_at, updated_at FROM auth_users WHERE email = $1`)
	queryByID := q(`SELECT id, full_name, email, avatar_url, password_hash, created_at, updated_at FROM auth_users WHERE id = $1`)
	queryByGoogleSubject := q(`SELECT u.id, u.full_name, u.email, u.avatar_url, u.password_hash, u.created_at, u.updated_at
		 FROM auth_google_identities gi
		 JOIN auth_users u ON u.id = gi.user_id
		 WHERE gi.google_subject = $1`)
	updateFullNameQuery := q(`UPDATE auth_users
		 SET full_name = $2,
		     updated_at = $3
		 WHERE id = $1
		 RETURNING id, full_name, email, avatar_url, password_hash, created_at, updated_at`)
	updateAvatarURLQuery := q(`UPDATE auth_users
		 SET avatar_url = $2,
		     updated_at = $3
		 WHERE id = $1
		 RETURNING id, full_name, email, avatar_url, password_hash, created_at, updated_at`)

	rows := sqlmock.NewRows(rowColumns).AddRow("usr_1", "User", "user@example.com", nil, "hash", now, now)
	mock.ExpectQuery(queryByEmail).WithArgs("user@example.com").WillReturnRows(rows)
	user, err := backend.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if user.ID != "usr_1" {
		t.Fatalf("unexpected user id: %s", user.ID)
	}

	mock.ExpectQuery(queryByEmail).WithArgs("missing@example.com").WillReturnError(sql.ErrNoRows)
	if _, err := backend.GetUserByEmail(context.Background(), "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	rows = sqlmock.NewRows(rowColumns).AddRow("usr_1", "User", "user@example.com", nil, "hash", now, now)
	mock.ExpectQuery(queryByID).WithArgs("usr_1").WillReturnRows(rows)
	if _, err := backend.GetUserByID(context.Background(), "usr_1"); err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	mock.ExpectQuery(queryByID).WithArgs("missing").WillReturnError(sql.ErrNoRows)
	if _, err := backend.GetUserByID(context.Background(), "missing"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	updateAt := now.Add(5 * time.Minute)
	rows = sqlmock.NewRows(rowColumns).AddRow("usr_1", "Renamed User", "user@example.com", "https://avatar.indobang.site/avatars/usr_1/current.webp", "hash", now, updateAt)
	mock.ExpectQuery(updateFullNameQuery).WithArgs("usr_1", "Renamed User", updateAt).WillReturnRows(rows)
	updatedUser, err := backend.UpdateUserFullName(context.Background(), "usr_1", "Renamed User", updateAt)
	if err != nil {
		t.Fatalf("UpdateUserFullName failed: %v", err)
	}
	if updatedUser.FullName != "Renamed User" {
		t.Fatalf("unexpected updated full name: %s", updatedUser.FullName)
	}
	if updatedUser.AvatarURL != "https://avatar.indobang.site/avatars/usr_1/current.webp" {
		t.Fatalf("unexpected avatar url after full name update: %s", updatedUser.AvatarURL)
	}

	mock.ExpectQuery(updateFullNameQuery).WithArgs("missing", "Renamed User", updateAt).WillReturnError(sql.ErrNoRows)
	if _, err := backend.UpdateUserFullName(context.Background(), "missing", "Renamed User", updateAt); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound on update, got %v", err)
	}

	rows = sqlmock.NewRows(rowColumns).AddRow("usr_1", "Renamed User", "user@example.com", "https://avatar.indobang.site/avatars/usr_1/new.webp", "hash", now, updateAt)
	mock.ExpectQuery(updateAvatarURLQuery).WithArgs("usr_1", "https://avatar.indobang.site/avatars/usr_1/new.webp", updateAt).WillReturnRows(rows)
	updatedUser, err = backend.UpdateUserAvatarURL(context.Background(), "usr_1", "https://avatar.indobang.site/avatars/usr_1/new.webp", updateAt)
	if err != nil {
		t.Fatalf("UpdateUserAvatarURL failed: %v", err)
	}
	if updatedUser.AvatarURL != "https://avatar.indobang.site/avatars/usr_1/new.webp" {
		t.Fatalf("unexpected updated avatar url: %s", updatedUser.AvatarURL)
	}

	mock.ExpectQuery(updateAvatarURLQuery).WithArgs("usr_1", nil, updateAt).WillReturnRows(sqlmock.NewRows(rowColumns).AddRow("usr_1", "Renamed User", "user@example.com", nil, "hash", now, updateAt))
	updatedUser, err = backend.UpdateUserAvatarURL(context.Background(), "usr_1", "", updateAt)
	if err != nil {
		t.Fatalf("UpdateUserAvatarURL clear failed: %v", err)
	}
	if updatedUser.AvatarURL != "" {
		t.Fatalf("expected cleared avatar url, got %q", updatedUser.AvatarURL)
	}

	mock.ExpectQuery(updateAvatarURLQuery).WithArgs("missing", "https://avatar.indobang.site/avatars/usr_1/new.webp", updateAt).WillReturnError(sql.ErrNoRows)
	if _, err := backend.UpdateUserAvatarURL(context.Background(), "missing", "https://avatar.indobang.site/avatars/usr_1/new.webp", updateAt); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound on avatar update, got %v", err)
	}

	rows = sqlmock.NewRows(rowColumns).AddRow("usr_2", "Google", "google@example.com", nil, "hash", now, now)
	mock.ExpectQuery(queryByGoogleSubject).WithArgs("sub_1").WillReturnRows(rows)
	user, err = backend.GetUserByGoogleSubject(context.Background(), "sub_1")
	if err != nil {
		t.Fatalf("GetUserByGoogleSubject failed: %v", err)
	}
	if user.Email != "google@example.com" {
		t.Fatalf("unexpected google user email: %s", user.Email)
	}

	mock.ExpectQuery(queryByGoogleSubject).WithArgs("sub_missing").WillReturnError(sql.ErrNoRows)
	if _, err := backend.GetUserByGoogleSubject(context.Background(), "sub_missing"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for subject, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_CreateSessionAndReadSession(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	now := time.Now().UTC()
	session := Session{ID: "ses_1", UserID: "usr_1", TokenHash: "hash", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}

	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnResult(sqlmock.NewResult(1, 1))
	if err := backend.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_sessions_token_hash"})
	if err := backend.CreateSession(context.Background(), session); !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken, got %v", err)
	}

	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnError(errors.New("db fail"))
	if err := backend.CreateSession(context.Background(), session); err == nil {
		t.Fatalf("expected create session error")
	}

	query := q(`SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at, client_ip, user_agent, keep_logged_in FROM auth_sessions WHERE token_hash = $1`)
	rows := sqlmock.NewRows([]string{"id", "user_id", "token_hash", "created_at", "expires_at", "revoked_at", "last_seen_at", "client_ip", "user_agent", "keep_logged_in"}).
		AddRow("ses_1", "usr_1", "hash", now, now.Add(time.Hour), nil, nil, nil, nil, false)
	mock.ExpectQuery(query).WithArgs("hash").WillReturnRows(rows)
	got, err := backend.GetSessionByTokenHash(context.Background(), "hash")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if got.RevokedAt != nil || got.LastSeenAt != nil {
		t.Fatalf("expected nil revoked/lastSeen for nullable columns")
	}

	revoked := now.Add(-time.Hour)
	lastSeen := now
	rows = sqlmock.NewRows([]string{"id", "user_id", "token_hash", "created_at", "expires_at", "revoked_at", "last_seen_at", "client_ip", "user_agent", "keep_logged_in"}).
		AddRow("ses_2", "usr_2", "hash2", now, now.Add(time.Hour), revoked, lastSeen, "127.0.0.1", "ua", true)
	mock.ExpectQuery(query).WithArgs("hash2").WillReturnRows(rows)
	got, err = backend.GetSessionByTokenHash(context.Background(), "hash2")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if got.RevokedAt == nil || got.LastSeenAt == nil || got.ClientIP != "127.0.0.1" || got.UserAgent != "ua" || !got.KeepLoggedIn {
		t.Fatalf("unexpected session read: %+v", got)
	}

	mock.ExpectQuery(query).WithArgs("missing").WillReturnError(sql.ErrNoRows)
	if _, err := backend.GetSessionByTokenHash(context.Background(), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	rows = sqlmock.NewRows([]string{"id"}).AddRow("bad")
	mock.ExpectQuery(query).WithArgs("bad-scan").WillReturnRows(rows)
	if _, err := backend.GetSessionByTokenHash(context.Background(), "bad-scan"); err == nil {
		t.Fatalf("expected scan error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresBackend_TouchRevokeAndUpsertGoogleIdentity(t *testing.T) {
	backend, mock, cleanup := newMockPostgresBackend(t)
	defer cleanup()
	backend.schemaReady = true

	now := time.Now().UTC()
	touchQuery := q(`UPDATE auth_sessions SET last_seen_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`)
	revokeQuery := q(`UPDATE auth_sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`)
	upsertQuery := q(`INSERT INTO auth_google_identities (google_subject, user_id, email, full_name, picture_url, email_verified, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (google_subject) DO UPDATE
		 SET email = EXCLUDED.email,
		     full_name = EXCLUDED.full_name,
		     picture_url = EXCLUDED.picture_url,
		     email_verified = EXCLUDED.email_verified,
		     updated_at = EXCLUDED.updated_at
		 WHERE auth_google_identities.user_id = EXCLUDED.user_id`)

	mock.ExpectExec(touchQuery).WithArgs("hash", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := backend.TouchSession(context.Background(), "hash", now); err != nil {
		t.Fatalf("TouchSession failed: %v", err)
	}

	mock.ExpectExec(touchQuery).WithArgs("missing", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := backend.TouchSession(context.Background(), "missing", now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	mock.ExpectExec(revokeQuery).WithArgs("hash", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := backend.RevokeSessionByTokenHash(context.Background(), "hash", now); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}

	mock.ExpectExec(revokeQuery).WithArgs("missing", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := backend.RevokeSessionByTokenHash(context.Background(), "missing", now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	identity := GoogleIdentity{GoogleSubject: "sub_1", UserID: "usr_1", Email: "user@example.com", FullName: "User", EmailVerified: true, CreatedAt: now, UpdatedAt: now}

	mock.ExpectExec(upsertQuery).WithArgs(identity.GoogleSubject, identity.UserID, identity.Email, identity.FullName, nil, identity.EmailVerified, identity.CreatedAt, identity.UpdatedAt).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := backend.UpsertGoogleIdentity(context.Background(), identity); err != nil {
		t.Fatalf("UpsertGoogleIdentity failed: %v", err)
	}

	mock.ExpectExec(upsertQuery).WithArgs(identity.GoogleSubject, identity.UserID, identity.Email, identity.FullName, nil, identity.EmailVerified, identity.CreatedAt, identity.UpdatedAt).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := backend.UpsertGoogleIdentity(context.Background(), identity); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict, got %v", err)
	}

	mock.ExpectExec(upsertQuery).WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_google_identities_user_id"})
	if err := backend.UpsertGoogleIdentity(context.Background(), identity); !errors.Is(err, ErrGoogleIdentityConflict) {
		t.Fatalf("expected unique violation google conflict, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostgresHelpers(t *testing.T) {
	if got := nullIfEmpty(" "); got != nil {
		t.Fatalf("expected nil for empty string, got %#v", got)
	}
	if got := nullIfEmpty("abc"); got != "abc" {
		t.Fatalf("expected raw value, got %#v", got)
	}

	if !isPGUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatalf("expected unique violation true")
	}
	if isPGUniqueViolation(errors.New("nope")) {
		t.Fatalf("expected unique violation false")
	}

	if mapped := mapUniqueViolationError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_users_email"}); !errors.Is(mapped, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken mapping, got %v", mapped)
	}
	if mapped := mapUniqueViolationError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_sessions_token_hash"}); !errors.Is(mapped, ErrInvalidSessionToken) {
		t.Fatalf("expected ErrInvalidSessionToken mapping, got %v", mapped)
	}
	if mapped := mapUniqueViolationError(&pgconn.PgError{Code: "23505", ConstraintName: "idx_auth_google_identities_user_id"}); !errors.Is(mapped, ErrGoogleIdentityConflict) {
		t.Fatalf("expected ErrGoogleIdentityConflict mapping, got %v", mapped)
	}
	if mapped := mapUniqueViolationError(errors.New("random")); mapped != nil {
		t.Fatalf("expected nil mapping for non-unique error, got %v", mapped)
	}
}

func TestNewPostgresBackend_Close(t *testing.T) {
	backend := newPostgresBackend("postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	if backend == nil || backend.db == nil {
		t.Fatalf("expected initialized backend")
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}
