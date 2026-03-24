package auth

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

var (
	ErrEmailTaken             = errors.New("email already registered")
	ErrUserNotFound           = errors.New("user not found")
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionRevoked         = errors.New("session revoked")
	ErrSessionExpired         = errors.New("session expired")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrInvalidSessionToken    = errors.New("invalid session token")
	ErrGoogleAuthDisabled     = errors.New("google auth is disabled")
	ErrGoogleTokenInvalid     = errors.New("google token is invalid")
	ErrGoogleEmailUnverified  = errors.New("google email is not verified")
	ErrGoogleIdentityConflict = errors.New("google account is linked to a different user")
)

type User struct {
	ID           string
	FullName     string
	Email        string
	AvatarURL    string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID           string
	UserID       string
	TokenHash    string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	LastSeenAt   *time.Time
	ClientIP     string
	UserAgent    string
	KeepLoggedIn bool
}

type GoogleIdentity struct {
	UserID        string
	GoogleSubject string
	Email         string
	FullName      string
	PictureURL    string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type backend interface {
	CreateUser(ctx context.Context, user User) error
	CreateUserAndSession(ctx context.Context, user User, session Session) error
	CreateUserSessionAndGoogleIdentity(ctx context.Context, user User, session Session, identity GoogleIdentity) error
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, userID string) (User, error)
	UpdateUserFullName(ctx context.Context, userID, fullName string, updatedAt time.Time) (User, error)
	UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (User, error)
	GetUserByGoogleSubject(ctx context.Context, googleSubject string) (User, error)
	CreateSession(ctx context.Context, session Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	TouchSession(ctx context.Context, tokenHash string, touchedAt time.Time) error
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
	UpsertGoogleIdentity(ctx context.Context, identity GoogleIdentity) error
	Close() error
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("auth store engine=postgres")
		}
		return &Store{backend: newPostgresBackend(cfg.PostgresDSN)}
	}

	if logger != nil {
		logger.Printf("auth store engine=memory (POSTGRES_DSN empty)")
	}
	return &Store{backend: newMemoryBackend()}
}

func (s *Store) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.Close()
}

func (s *Store) CreateUser(ctx context.Context, user User) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	return s.backend.CreateUser(ctx, user)
}

func (s *Store) CreateUserAndSession(ctx context.Context, user User, session Session) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	return s.backend.CreateUserAndSession(ctx, user, session)
}

func (s *Store) CreateUserSessionAndGoogleIdentity(ctx context.Context, user User, session Session, identity GoogleIdentity) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	identity.GoogleSubject = strings.TrimSpace(identity.GoogleSubject)
	identity.Email = strings.TrimSpace(strings.ToLower(identity.Email))
	identity.FullName = strings.TrimSpace(identity.FullName)
	identity.PictureURL = strings.TrimSpace(identity.PictureURL)
	return s.backend.CreateUserSessionAndGoogleIdentity(ctx, user, session, identity)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	if s == nil || s.backend == nil {
		return User{}, errors.New("auth store is not initialized")
	}
	return s.backend.GetUserByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (User, error) {
	if s == nil || s.backend == nil {
		return User{}, errors.New("auth store is not initialized")
	}
	return s.backend.GetUserByID(ctx, strings.TrimSpace(userID))
}

func (s *Store) UpdateUserFullName(ctx context.Context, userID, fullName string, updatedAt time.Time) (User, error) {
	if s == nil || s.backend == nil {
		return User{}, errors.New("auth store is not initialized")
	}
	trimmedUserID := strings.TrimSpace(userID)
	trimmedFullName := strings.TrimSpace(fullName)
	if trimmedUserID == "" {
		return User{}, &ValidationError{Message: "user_id is required"}
	}
	if trimmedFullName == "" {
		return User{}, &ValidationError{Message: "full_name is required"}
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return s.backend.UpdateUserFullName(ctx, trimmedUserID, trimmedFullName, updatedAt.UTC())
}

func (s *Store) UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (User, error) {
	if s == nil || s.backend == nil {
		return User{}, errors.New("auth store is not initialized")
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return User{}, &ValidationError{Message: "user_id is required"}
	}

	trimmedAvatarURL := strings.TrimSpace(avatarURL)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	return s.backend.UpdateUserAvatarURL(ctx, trimmedUserID, trimmedAvatarURL, updatedAt.UTC())
}

func (s *Store) GetUserByGoogleSubject(ctx context.Context, googleSubject string) (User, error) {
	if s == nil || s.backend == nil {
		return User{}, errors.New("auth store is not initialized")
	}
	return s.backend.GetUserByGoogleSubject(ctx, strings.TrimSpace(googleSubject))
}

func (s *Store) CreateSession(ctx context.Context, session Session) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	return s.backend.CreateSession(ctx, session)
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	if s == nil || s.backend == nil {
		return Session{}, errors.New("auth store is not initialized")
	}
	return s.backend.GetSessionByTokenHash(ctx, strings.TrimSpace(strings.ToLower(tokenHash)))
}

func (s *Store) TouchSession(ctx context.Context, tokenHash string, touchedAt time.Time) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	return s.backend.TouchSession(ctx, strings.TrimSpace(strings.ToLower(tokenHash)), touchedAt)
}

func (s *Store) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	return s.backend.RevokeSessionByTokenHash(ctx, strings.TrimSpace(strings.ToLower(tokenHash)), revokedAt)
}

func (s *Store) UpsertGoogleIdentity(ctx context.Context, identity GoogleIdentity) error {
	if s == nil || s.backend == nil {
		return errors.New("auth store is not initialized")
	}
	identity.GoogleSubject = strings.TrimSpace(identity.GoogleSubject)
	identity.Email = strings.TrimSpace(strings.ToLower(identity.Email))
	identity.FullName = strings.TrimSpace(identity.FullName)
	identity.PictureURL = strings.TrimSpace(identity.PictureURL)
	return s.backend.UpsertGoogleIdentity(ctx, identity)
}
