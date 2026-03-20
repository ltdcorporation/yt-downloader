package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSessionTTL         = 24 * time.Hour
	defaultRememberSessionTTL = 30 * 24 * time.Hour
	defaultBcryptCost         = 12

	minPasswordLength = 8
	maxPasswordLength = 128
	maxEmailLength    = 254
	minNameLength     = 2
	maxNameLength     = 120
)

const defaultDummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOe2fN5z9Yx8RJWb1x1o1t4bm/FvGV8e6"

type Options struct {
	SessionTTL         time.Duration
	RememberSessionTTL time.Duration
	BcryptCost         int
	DummyPasswordHash  string
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "invalid input"
	}
	return e.Message
}

type PublicUser struct {
	ID        string    `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResult struct {
	User        PublicUser `json:"user"`
	AccessToken string     `json:"access_token"`
	TokenType   string     `json:"token_type"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

type SessionIdentity struct {
	User      PublicUser
	SessionID string
	ExpiresAt time.Time
}

type RegisterInput struct {
	FullName     string
	Email        string
	Password     string
	KeepLoggedIn bool
	ClientIP     string
	UserAgent    string
}

type LoginInput struct {
	Email        string
	Password     string
	KeepLoggedIn bool
	ClientIP     string
	UserAgent    string
}

type Service struct {
	store *Store
	now   func() time.Time

	sessionTTL         time.Duration
	rememberSessionTTL time.Duration
	bcryptCost         int
	dummyPasswordHash  string
}

func NewService(store *Store, opts Options) *Service {
	sessionTTL := opts.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = defaultSessionTTL
	}

	rememberSessionTTL := opts.RememberSessionTTL
	if rememberSessionTTL <= 0 {
		rememberSessionTTL = defaultRememberSessionTTL
	}

	bcryptCost := opts.BcryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
		bcryptCost = defaultBcryptCost
	}

	dummyPasswordHash := strings.TrimSpace(opts.DummyPasswordHash)
	if dummyPasswordHash == "" {
		dummyPasswordHash = defaultDummyPasswordHash
	}

	return &Service{
		store:              store,
		now:                func() time.Time { return time.Now().UTC() },
		sessionTTL:         sessionTTL,
		rememberSessionTTL: rememberSessionTTL,
		bcryptCost:         bcryptCost,
		dummyPasswordHash:  dummyPasswordHash,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	if s == nil || s.store == nil {
		return AuthResult{}, errors.New("auth service is not initialized")
	}

	fullName, err := normalizeFullName(input.FullName)
	if err != nil {
		return AuthResult{}, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return AuthResult{}, err
	}

	password := strings.TrimSpace(input.Password)
	if err := validatePassword(password); err != nil {
		return AuthResult{}, err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now()
	user := User{
		ID:           "usr_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		FullName:     fullName,
		Email:        email,
		PasswordHash: string(hashBytes),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	token, tokenHash, err := generateSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	session := s.newSession(now, user.ID, tokenHash, input.KeepLoggedIn, input.ClientIP, input.UserAgent)

	if err := s.store.CreateUserAndSession(ctx, user, session); err != nil {
		return AuthResult{}, err
	}

	return buildAuthResult(user, token, session.ExpiresAt), nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	if s == nil || s.store == nil {
		return AuthResult{}, errors.New("auth service is not initialized")
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return AuthResult{}, err
	}

	password := strings.TrimSpace(input.Password)
	if password == "" {
		return AuthResult{}, &ValidationError{Message: "password is required"}
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			s.consumeDummyCompare(password)
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	now := s.now()
	token, tokenHash, err := generateSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	session := s.newSession(now, user.ID, tokenHash, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
	if err := s.store.CreateSession(ctx, session); err != nil {
		return AuthResult{}, err
	}

	return buildAuthResult(user, token, session.ExpiresAt), nil
}

func (s *Service) AuthenticateToken(ctx context.Context, rawToken string) (SessionIdentity, error) {
	if s == nil || s.store == nil {
		return SessionIdentity{}, errors.New("auth service is not initialized")
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return SessionIdentity{}, ErrInvalidSessionToken
	}

	tokenHash := hashToken(token)
	session, err := s.store.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return SessionIdentity{}, ErrInvalidSessionToken
		}
		return SessionIdentity{}, err
	}

	now := s.now()
	if session.RevokedAt != nil {
		return SessionIdentity{}, ErrSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return SessionIdentity{}, ErrSessionExpired
	}

	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return SessionIdentity{}, ErrInvalidSessionToken
		}
		return SessionIdentity{}, err
	}

	_ = s.store.TouchSession(ctx, tokenHash, now)

	return SessionIdentity{
		User: PublicUser{
			ID:        user.ID,
			FullName:  user.FullName,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if s == nil || s.store == nil {
		return errors.New("auth service is not initialized")
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return nil
	}

	tokenHash := hashToken(token)
	err := s.store.RevokeSessionByTokenHash(ctx, tokenHash, s.now())
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	return err
}

func (s *Service) newSession(now time.Time, userID, tokenHash string, keepLoggedIn bool, clientIP, userAgent string) Session {
	ttl := s.sessionTTL
	if keepLoggedIn {
		ttl = s.rememberSessionTTL
	}
	expiresAt := now.Add(ttl)

	lastSeenAt := now
	return Session{
		ID:           "ses_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:       userID,
		TokenHash:    tokenHash,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastSeenAt:   &lastSeenAt,
		ClientIP:     strings.TrimSpace(clientIP),
		UserAgent:    normalizeUserAgent(userAgent),
		KeepLoggedIn: keepLoggedIn,
	}
}

func (s *Service) consumeDummyCompare(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(s.dummyPasswordHash), []byte(password))
}

func buildAuthResult(user User, token string, expiresAt time.Time) AuthResult {
	return AuthResult{
		User: PublicUser{
			ID:        user.ID,
			FullName:  user.FullName,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}
}

func normalizeFullName(raw string) (string, error) {
	name := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if len(name) < minNameLength {
		return "", &ValidationError{Message: "full_name must be at least 2 characters"}
	}
	if len(name) > maxNameLength {
		return "", &ValidationError{Message: "full_name is too long"}
	}
	return name, nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(raw))
	if email == "" {
		return "", &ValidationError{Message: "email is required"}
	}
	if len(email) > maxEmailLength {
		return "", &ValidationError{Message: "email is too long"}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", &ValidationError{Message: "email format is invalid"}
	}
	return email, nil
}

func validatePassword(password string) error {
	if password == "" {
		return &ValidationError{Message: "password is required"}
	}
	if len(password) < minPasswordLength {
		return &ValidationError{Message: "password must be at least 8 characters"}
	}
	if len(password) > maxPasswordLength {
		return &ValidationError{Message: "password is too long"}
	}

	hasLetter := false
	hasNumber := false
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsNumber(r) {
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return &ValidationError{Message: "password must include letters and numbers"}
	}

	return nil
}

func generateSessionToken() (token string, tokenHash string, err error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}

	token = "st_" + base64.RawURLEncoding.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func normalizeUserAgent(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= 512 {
		return trimmed
	}
	return trimmed[:512]
}
