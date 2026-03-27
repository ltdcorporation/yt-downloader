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
		`INSERT INTO auth_users (id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID,
		user.FullName,
		user.Email,
		nullIfEmpty(user.AvatarURL),
		user.PasswordHash,
		user.Role,
		user.Plan,
		user.PlanExpiresAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
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
		`INSERT INTO auth_users (id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID,
		user.FullName,
		user.Email,
		nullIfEmpty(user.AvatarURL),
		user.PasswordHash,
		user.Role,
		user.Plan,
		user.PlanExpiresAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
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
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("create auth session (transaction): %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth registration transaction: %w", err)
	}

	return nil
}

func (p *postgresBackend) CreateUserSessionAndGoogleIdentity(ctx context.Context, user User, session Session, identity GoogleIdentity) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth google registration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO auth_users (id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID,
		user.FullName,
		user.Email,
		nullIfEmpty(user.AvatarURL),
		user.PasswordHash,
		user.Role,
		user.Plan,
		user.PlanExpiresAt,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("create auth user (google transaction): %w", err)
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
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("create auth session (google transaction): %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO auth_google_identities (google_subject, user_id, email, full_name, picture_url, email_verified, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		identity.GoogleSubject,
		identity.UserID,
		identity.Email,
		nullIfEmpty(identity.FullName),
		nullIfEmpty(identity.PictureURL),
		identity.EmailVerified,
		identity.CreatedAt,
		identity.UpdatedAt,
	)
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("create google identity (transaction): %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth google registration transaction: %w", err)
	}

	return nil
}

func (p *postgresBackend) GetUserByEmail(ctx context.Context, email string) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at FROM auth_users WHERE email = $1`,
		email,
	)

	user, err := scanUserRow(row)
	if err != nil {
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
		`SELECT id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at FROM auth_users WHERE id = $1`,
		userID,
	)

	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get auth user by id: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) UpdateUserFullName(ctx context.Context, userID, fullName string, updatedAt time.Time) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`UPDATE auth_users
		 SET full_name = $2,
		     updated_at = $3
		 WHERE id = $1
		 RETURNING id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at`,
		userID,
		fullName,
		updatedAt.UTC(),
	)

	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("update auth user full name: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`UPDATE auth_users
		 SET avatar_url = $2,
		     updated_at = $3
		 WHERE id = $1
		 RETURNING id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at`,
		userID,
		nullIfEmpty(avatarURL),
		updatedAt.UTC(),
	)

	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("update auth user avatar url: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) UpdateUserByAdmin(ctx context.Context, userID string, patch AdminUserPatch, updatedAt time.Time) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	setClauses := make([]string, 0, 5)
	args := make([]any, 0, 6)
	args = append(args, userID)
	argPos := 2

	if patch.FullName != nil {
		setClauses = append(setClauses, fmt.Sprintf("full_name = $%d", argPos))
		args = append(args, *patch.FullName)
		argPos++
	}
	if patch.Role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argPos))
		args = append(args, *patch.Role)
		argPos++
	}
	if patch.Plan != nil {
		setClauses = append(setClauses, fmt.Sprintf("plan = $%d", argPos))
		args = append(args, *patch.Plan)
		argPos++
	}
	if patch.PlanExpiresAtSet {
		setClauses = append(setClauses, fmt.Sprintf("plan_expires_at = $%d", argPos))
		if patch.PlanExpiresAt == nil {
			args = append(args, nil)
		} else {
			expiresAt := patch.PlanExpiresAt.UTC()
			args = append(args, expiresAt)
		}
		argPos++
	}

	if len(setClauses) == 0 {
		return p.GetUserByID(ctx, userID)
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, updatedAt.UTC())

	query := fmt.Sprintf(`UPDATE auth_users
		 SET %s
		 WHERE id = $1
		 RETURNING id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at`, strings.Join(setClauses, ",\n\t\t     "))

	row := p.db.QueryRowContext(ctx, query, args...)
	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("update auth user by admin: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) GetUserByGoogleSubject(ctx context.Context, googleSubject string) (User, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return User{}, err
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT u.id, u.full_name, u.email, u.avatar_url, u.password_hash, u.role, u.plan, u.plan_expires_at, u.created_at, u.updated_at
		 FROM auth_google_identities gi
		 JOIN auth_users u ON u.id = gi.user_id
		 WHERE gi.google_subject = $1`,
		googleSubject,
	)

	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("get auth user by google subject: %w", err)
	}

	return user, nil
}

func (p *postgresBackend) ListUsers(ctx context.Context, limit, offset int) ([]User, int, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_users`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count auth users: %w", err)
	}

	rows, err := p.db.QueryContext(
		ctx,
		`SELECT id, full_name, email, avatar_url, password_hash, role, plan, plan_expires_at, created_at, updated_at
		 FROM auth_users
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list auth users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUserRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan auth user: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
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
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
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

func (p *postgresBackend) UpsertGoogleIdentity(ctx context.Context, identity GoogleIdentity) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	result, err := p.db.ExecContext(
		ctx,
		`INSERT INTO auth_google_identities (google_subject, user_id, email, full_name, picture_url, email_verified, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (google_subject) DO UPDATE
		 SET email = EXCLUDED.email,
		     full_name = EXCLUDED.full_name,
		     picture_url = EXCLUDED.picture_url,
		     email_verified = EXCLUDED.email_verified,
		     updated_at = EXCLUDED.updated_at
		 WHERE auth_google_identities.user_id = EXCLUDED.user_id`,
		identity.GoogleSubject,
		identity.UserID,
		identity.Email,
		nullIfEmpty(identity.FullName),
		nullIfEmpty(identity.PictureURL),
		identity.EmailVerified,
		identity.CreatedAt,
		identity.UpdatedAt,
	)
	if mappedErr := mapUniqueViolationError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("upsert google identity: %w", err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrGoogleIdentityConflict
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
			avatar_url TEXT,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			plan TEXT NOT NULL DEFAULT 'free',
			plan_expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
		`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users (email)`,
		`ALTER TABLE auth_users ADD COLUMN IF NOT EXISTS avatar_url TEXT`,
		`ALTER TABLE auth_users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'`,
		`ALTER TABLE auth_users ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'free'`,
		`ALTER TABLE auth_users ADD COLUMN IF NOT EXISTS plan_expires_at TIMESTAMPTZ`,
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
		`
		CREATE TABLE IF NOT EXISTS auth_google_identities (
			google_subject TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
			email TEXT NOT NULL,
			full_name TEXT,
			picture_url TEXT,
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
		`,
		`ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS full_name TEXT`,
		`ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS picture_url TEXT`,
		`ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
		`ALTER TABLE auth_google_identities ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_google_identities_user_id ON auth_google_identities (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_google_identities_email ON auth_google_identities (email)`,
	}

	for _, statement := range statements {
		if _, err := p.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure auth schema: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func scanUserRow(row interface{ Scan(dest ...any) error }) (User, error) {
	var user User
	var avatarURL sql.NullString
	var planExpiresAt sql.NullTime
	if err := row.Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&avatarURL,
		&user.PasswordHash,
		&user.Role,
		&user.Plan,
		&planExpiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return User{}, err
	}
	user.AvatarURL = strings.TrimSpace(avatarURL.String)
	if planExpiresAt.Valid {
		t := planExpiresAt.Time.UTC()
		user.PlanExpiresAt = &t
	}
	return user, nil
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

func mapUniqueViolationError(err error) error {
	if !isPGUniqueViolation(err) {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}

	constraint := strings.ToLower(strings.TrimSpace(pgErr.ConstraintName))
	switch {
	case strings.Contains(constraint, "auth_users") || strings.Contains(constraint, "idx_auth_users_email"):
		return ErrEmailTaken
	case strings.Contains(constraint, "auth_sessions") || strings.Contains(constraint, "idx_auth_sessions_token_hash"):
		return ErrInvalidSessionToken
	case strings.Contains(constraint, "auth_google_identities") || strings.Contains(constraint, "idx_auth_google_identities"):
		return ErrGoogleIdentityConflict
	}

	detail := strings.ToLower(pgErr.Detail + " " + pgErr.Message)
	switch {
	case strings.Contains(detail, "auth_users") && strings.Contains(detail, "email"):
		return ErrEmailTaken
	case strings.Contains(detail, "auth_sessions") && strings.Contains(detail, "token"):
		return ErrInvalidSessionToken
	case strings.Contains(detail, "auth_google_identities") && (strings.Contains(detail, "google_subject") || strings.Contains(detail, "user_id")):
		return ErrGoogleIdentityConflict
	}

	return nil
}
