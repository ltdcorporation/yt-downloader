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
	SessionTTL          time.Duration
	RememberSessionTTL  time.Duration
	BcryptCost          int
	DummyPasswordHash   string
	GoogleTokenVerifier GoogleTokenVerifier
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
	ID            string     `json:"id"`
	FullName      string     `json:"full_name"`
	Email         string     `json:"email"`
	AvatarURL     string     `json:"avatar_url,omitempty"`
	Role          Role       `json:"role"`
	Plan          Plan       `json:"plan"`
	PlanExpiresAt *time.Time `json:"plan_expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
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

type GoogleLoginInput struct {
	IDToken      string
	KeepLoggedIn bool
	ClientIP     string
	UserAgent    string
}

type UpdateProfileInput struct {
	FullName string
}

type AdminUpdateUserInput struct {
	FullName         *string
	Role             *Role
	Plan             *Plan
	PlanExpiresAtSet bool
	PlanExpiresAt    *time.Time
}

type Service struct {
	store *Store
	now   func() time.Time

	sessionTTL          time.Duration
	rememberSessionTTL  time.Duration
	bcryptCost          int
	dummyPasswordHash   string
	googleTokenVerifier GoogleTokenVerifier
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
		store:               store,
		now:                 func() time.Time { return time.Now().UTC() },
		sessionTTL:          sessionTTL,
		rememberSessionTTL:  rememberSessionTTL,
		bcryptCost:          bcryptCost,
		dummyPasswordHash:   dummyPasswordHash,
		googleTokenVerifier: opts.GoogleTokenVerifier,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	return s.registerWithRole(ctx, input, RoleUser)
}

func (s *Service) RegisterAdmin(ctx context.Context, input RegisterInput) (AuthResult, error) {
	return s.registerWithRole(ctx, input, RoleAdmin)
}

func (s *Service) registerWithRole(ctx context.Context, input RegisterInput, role Role) (AuthResult, error) {
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
		Role:         role,
		Plan:         PlanFree,
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

	return s.issueSessionForUser(ctx, user, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
}

func (s *Service) LoginWithGoogle(ctx context.Context, input GoogleLoginInput) (AuthResult, error) {
	if s == nil || s.store == nil {
		return AuthResult{}, errors.New("auth service is not initialized")
	}
	if s.googleTokenVerifier == nil {
		return AuthResult{}, ErrGoogleAuthDisabled
	}

	claims, err := s.googleTokenVerifier.Verify(ctx, input.IDToken)
	if err != nil {
		return AuthResult{}, wrapGoogleVerifyError(err)
	}
	if !claims.EmailVerified {
		return AuthResult{}, ErrGoogleEmailUnverified
	}

	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return AuthResult{}, ErrGoogleTokenInvalid
	}

	email, err := normalizeEmail(claims.Email)
	if err != nil {
		return AuthResult{}, ErrGoogleTokenInvalid
	}

	fullName, err := normalizeGoogleDisplayName(claims.FullName, email)
	if err != nil {
		return AuthResult{}, ErrGoogleTokenInvalid
	}

	now := s.now()
	identity := GoogleIdentity{
		GoogleSubject: subject,
		Email:         email,
		FullName:      fullName,
		PictureURL:    claims.PictureURL,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	user, err := s.store.GetUserByGoogleSubject(ctx, subject)
	if err == nil {
		identity.UserID = user.ID
		if upsertErr := s.store.UpsertGoogleIdentity(ctx, identity); upsertErr != nil {
			return AuthResult{}, upsertErr
		}
		return s.issueSessionForUser(ctx, user, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return AuthResult{}, err
	}

	user, err = s.store.GetUserByEmail(ctx, email)
	if err == nil {
		identity.UserID = user.ID
		if upsertErr := s.store.UpsertGoogleIdentity(ctx, identity); upsertErr != nil {
			return AuthResult{}, upsertErr
		}
		return s.issueSessionForUser(ctx, user, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return AuthResult{}, err
	}

	passwordHash, err := s.generateUnusablePasswordHash()
	if err != nil {
		return AuthResult{}, err
	}

	user = User{
		ID:           "usr_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		FullName:     fullName,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         RoleUser,
		Plan:         PlanFree,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	token, tokenHash, err := generateSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	session := s.newSession(now, user.ID, tokenHash, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
	identity.UserID = user.ID

	if err := s.store.CreateUserSessionAndGoogleIdentity(ctx, user, session, identity); err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			linkedUser, lookupErr := s.store.GetUserByEmail(ctx, email)
			if lookupErr != nil {
				return AuthResult{}, lookupErr
			}
			identity.UserID = linkedUser.ID
			if upsertErr := s.store.UpsertGoogleIdentity(ctx, identity); upsertErr != nil {
				return AuthResult{}, upsertErr
			}
			return s.issueSessionForUser(ctx, linkedUser, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
		case errors.Is(err, ErrGoogleIdentityConflict):
			linkedUser, lookupErr := s.store.GetUserByGoogleSubject(ctx, subject)
			if lookupErr != nil {
				return AuthResult{}, err
			}
			return s.issueSessionForUser(ctx, linkedUser, input.KeepLoggedIn, input.ClientIP, input.UserAgent)
		default:
			return AuthResult{}, err
		}
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
			ID:            user.ID,
			FullName:      user.FullName,
			Email:         user.Email,
			AvatarURL:     user.AvatarURL,
			Role:          user.Role,
			Plan:          user.Plan,
			PlanExpiresAt: user.PlanExpiresAt,
			CreatedAt:     user.CreatedAt,
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

func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (PublicUser, error) {
	if s == nil || s.store == nil {
		return PublicUser{}, errors.New("auth service is not initialized")
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return PublicUser{}, &ValidationError{Message: "user_id is required"}
	}

	fullName, err := normalizeFullName(input.FullName)
	if err != nil {
		return PublicUser{}, err
	}

	user, err := s.store.UpdateUserFullName(ctx, trimmedUserID, fullName, s.now())
	if err != nil {
		return PublicUser{}, err
	}

	return publicUserFromUser(user), nil
}

func (s *Service) UpdateAvatarURL(ctx context.Context, userID, avatarURL string) (PublicUser, error) {
	if s == nil || s.store == nil {
		return PublicUser{}, errors.New("auth service is not initialized")
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return PublicUser{}, &ValidationError{Message: "user_id is required"}
	}

	user, err := s.store.UpdateUserAvatarURL(ctx, trimmedUserID, strings.TrimSpace(avatarURL), s.now())
	if err != nil {
		return PublicUser{}, err
	}

	return publicUserFromUser(user), nil
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]PublicUser, int, error) {
	if s == nil || s.store == nil {
		return nil, 0, errors.New("auth service is not initialized")
	}

	users, total, err := s.store.ListUsers(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	publicUsers := make([]PublicUser, 0, len(users))
	for _, user := range users {
		publicUsers = append(publicUsers, publicUserFromUser(user))
	}

	return publicUsers, total, nil
}

func (s *Service) GetUser(ctx context.Context, userID string) (PublicUser, error) {
	if s == nil || s.store == nil {
		return PublicUser{}, errors.New("auth service is not initialized")
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return PublicUser{}, &ValidationError{Message: "user_id is required"}
	}

	user, err := s.store.GetUserByID(ctx, trimmedUserID)
	if err != nil {
		return PublicUser{}, err
	}

	return publicUserFromUser(user), nil
}

func (s *Service) UpdateUserByAdmin(ctx context.Context, actorUserID, targetUserID string, input AdminUpdateUserInput) (PublicUser, error) {
	if s == nil || s.store == nil {
		return PublicUser{}, errors.New("auth service is not initialized")
	}

	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		return PublicUser{}, &ValidationError{Message: "user_id is required"}
	}

	if input.FullName == nil && input.Role == nil && input.Plan == nil && !input.PlanExpiresAtSet {
		return PublicUser{}, &ValidationError{Message: "at least one field must be provided"}
	}

	current, err := s.store.GetUserByID(ctx, targetUserID)
	if err != nil {
		return PublicUser{}, err
	}

	next := current

	if input.FullName != nil {
		normalizedName, err := normalizeFullName(*input.FullName)
		if err != nil {
			return PublicUser{}, err
		}
		next.FullName = normalizedName
	}

	if input.Role != nil {
		role := Role(strings.TrimSpace(string(*input.Role)))
		if role != RoleAdmin && role != RoleUser {
			return PublicUser{}, &ValidationError{Message: "role must be one of: admin, user"}
		}
		if strings.TrimSpace(actorUserID) != "" && strings.TrimSpace(actorUserID) == targetUserID && current.Role == RoleAdmin && role != RoleAdmin {
			return PublicUser{}, &ValidationError{Message: "cannot remove your own admin role"}
		}
		next.Role = role
	}

	if input.Plan != nil {
		plan := Plan(strings.TrimSpace(string(*input.Plan)))
		switch plan {
		case PlanFree, PlanDaily, PlanWeekly, PlanMonthly:
			// valid
		default:
			return PublicUser{}, &ValidationError{Message: "plan must be one of: free, daily, weekly, monthly"}
		}
		next.Plan = plan
	}

	if input.PlanExpiresAtSet {
		if input.PlanExpiresAt == nil {
			next.PlanExpiresAt = nil
		} else {
			expiresAt := input.PlanExpiresAt.UTC()
			next.PlanExpiresAt = &expiresAt
		}
	}

	if next.Plan == PlanFree {
		next.PlanExpiresAt = nil
	} else if next.PlanExpiresAt == nil {
		expiresAt := s.now().Add(defaultPlanDuration(next.Plan))
		next.PlanExpiresAt = &expiresAt
	}

	patch := AdminUserPatch{}
	if next.FullName != current.FullName {
		patch.FullName = &next.FullName
	}
	if next.Role != current.Role {
		patch.Role = &next.Role
	}
	if next.Plan != current.Plan {
		patch.Plan = &next.Plan
	}
	if !timesEqual(next.PlanExpiresAt, current.PlanExpiresAt) {
		patch.PlanExpiresAtSet = true
		patch.PlanExpiresAt = next.PlanExpiresAt
	}

	if patch.FullName == nil && patch.Role == nil && patch.Plan == nil && !patch.PlanExpiresAtSet {
		return publicUserFromUser(current), nil
	}

	updatedUser, err := s.store.UpdateUserByAdmin(ctx, targetUserID, patch, s.now())
	if err != nil {
		return PublicUser{}, err
	}

	return publicUserFromUser(updatedUser), nil
}

func (s *Service) issueSessionForUser(ctx context.Context, user User, keepLoggedIn bool, clientIP, userAgent string) (AuthResult, error) {
	now := s.now()
	token, tokenHash, err := generateSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	session := s.newSession(now, user.ID, tokenHash, keepLoggedIn, clientIP, userAgent)
	if err := s.store.CreateSession(ctx, session); err != nil {
		return AuthResult{}, err
	}

	return buildAuthResult(user, token, session.ExpiresAt), nil
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

func (s *Service) generateUnusablePasswordHash() (string, error) {
	randomPassword := make([]byte, 48)
	if _, err := rand.Read(randomPassword); err != nil {
		return "", fmt.Errorf("generate random password seed: %w", err)
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(randomPassword)), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash random password: %w", err)
	}

	return string(hashBytes), nil
}

func buildAuthResult(user User, token string, expiresAt time.Time) AuthResult {
	return AuthResult{
		User:        publicUserFromUser(user),
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}
}

func publicUserFromUser(user User) PublicUser {
	return PublicUser{
		ID:            user.ID,
		FullName:      user.FullName,
		Email:         user.Email,
		AvatarURL:     user.AvatarURL,
		Role:          user.Role,
		Plan:          user.Plan,
		PlanExpiresAt: user.PlanExpiresAt,
		CreatedAt:     user.CreatedAt,
	}
}

func timesEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.UTC().Equal(b.UTC())
}

func defaultPlanDuration(plan Plan) time.Duration {
	switch plan {
	case PlanDaily:
		return 24 * time.Hour
	case PlanWeekly:
		return 7 * 24 * time.Hour
	case PlanMonthly:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

func normalizeGoogleDisplayName(rawName, email string) (string, error) {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(rawName)), " ")
	if cleaned == "" {
		localPart := email
		if idx := strings.Index(localPart, "@"); idx > 0 {
			localPart = localPart[:idx]
		}
		mapped := strings.Map(func(r rune) rune {
			switch {
			case unicode.IsLetter(r), unicode.IsNumber(r):
				return r
			case r == '_', r == '-', r == '.':
				return ' '
			default:
				return -1
			}
		}, localPart)
		cleaned = strings.Join(strings.Fields(mapped), " ")
	}
	if cleaned == "" {
		cleaned = "Google User"
	}
	return normalizeFullName(cleaned)
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
