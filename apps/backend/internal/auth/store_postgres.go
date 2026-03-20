package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresBackend struct {
	db *sql.DB

	schemaMu    sync.Mutex
	schemaReady bool
}

func newPostgresBackend(dsn string) *postgresBackend {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}

	return &postgresBackend{db: db}
}

func (p *postgresBackend) Close() error {
	return p.db.Close()
}

func (p *postgresBackend) CreateUser(ctx context.Context, user User) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO auth_users (id, full_name, email, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID,
		user.FullName,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return ErrEmailTaken
		}
		return fmt.Errorf("create auth user: %w", err)
	}

	return nil
}

func (p *postgresBackend) CreateUserAndSession(ctx context.Context, user User, session Session) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth registration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO auth_users (id, full_name, email, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID,
		user.FullName,
		user.Email,
		user.PasswordHash,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return ErrEmailTaken
		}
		return fmt.Errorf("create auth user (transaction): %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO auth_sessions (id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at, client_ip, user_agent, keep_logged_in) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.CreatedAt,
		session.ExpiresAt,
		session.RevokedAt,
		session.LastSeenAt,
		nullIfEmpty(session.ClientIP),
		nullIfEmpty(session.UserAgent),
		session.KeepLoggedIn,
	)
	if err != nil {
		return fmt.Errorf("create auth session (transaction): %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth registration transaction: %w", err)
	}

	return nil
}

func (p *postgresBackend) GetUserByEmail(ctx context.Context, email string) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT id, full_name, email, password_hash, created_at, updated_at FROM auth_users WHERE email = $1`,
		email,
	)

	var user User
	if err := row.Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get auth user by email: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) GetUserByID(ctx context.Context, userID string) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT id, full_name, email, password_hash, created_at, updated_at FROM auth_users WHERE id = $1`,
		userID,
	)

	var user User
	if err := row.Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get auth user by id: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) CreateSession(ctx context.Context, session Session) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO auth_sessions (id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at, client_ip, user_agent, keep_logged_in) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.CreatedAt,
		session.ExpiresAt,
		session.RevokedAt,
		session.LastSeenAt,
		nullIfEmpty(session.ClientIP),
		nullIfEmpty(session.UserAgent),
		session.KeepLoggedIn,
	)
	if err != nil {
		if isPGUniqueViolation(err) {
			return ErrInvalidSessionToken
		}
		return fmt.Errorf("create auth session: %w", err)
	}

	return nil
}

func (p *postgresBackend) GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Session{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at, client_ip, user_agent, keep_logged_in FROM auth_sessions WHERE token_hash = $1`,
		tokenHash,
	)

	var session Session
	var revokedAt sql.NullTime
	var lastSeenAt sql.NullTime
	var clientIP sql.NullString
	var userAgent sql.NullString
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.CreatedAt,
		&session.ExpiresAt,
		&revokedAt,
		&lastSeenAt,
		&clientIP,
		&userAgent,
		&session.KeepLoggedIn,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("get auth session by token hash: %w", err)
	}

	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		session.RevokedAt = &t
	}
	if lastSeenAt.Valid {
		t := lastSeenAt.Time.UTC()
		session.LastSeenAt = &t
	}
	session.ClientIP = clientIP.String
	session.UserAgent = userAgent.String

	return session, nil
}

func (p *postgresBackend) TouchSession(ctx context.Context, tokenHash string, touchedAt time.Time) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	result, err := p.db.ExecContext(
		ctx,
		`UPDATE auth_sessions SET last_seen_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
		touchedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (p *postgresBackend) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	result, err := p.db.ExecContext(
		ctx,
		`UPDATE auth_sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
		revokedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS auth_users (
			id TEXT PRIMARY KEY,
			full_name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
		`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users (email)`,
		`
		CREATE TABLE IF NOT EXISTS auth_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ,
			last_seen_at TIMESTAMPTZ,
			client_ip TEXT,
			user_agent TEXT,
			keep_logged_in BOOLEAN NOT NULL DEFAULT FALSE
		)
		`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS client_ip TEXT`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS user_agent TEXT`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS keep_logged_in BOOLEAN NOT NULL DEFAULT FALSE`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_token_hash ON auth_sessions (token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions (expires_at)`,
	}

	for _, statement := range statements {
		if _, err := p.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure auth schema: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
