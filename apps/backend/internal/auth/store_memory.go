package auth

import (
	"context"
	"sort"
	"sync"
	"time"
)

type memoryBackend struct {
	mu sync.RWMutex

	usersByEmail        map[string]User
	usersByID           map[string]User
	sessionsByTokenHash map[string]Session
	googleBySubject     map[string]GoogleIdentity
	googleByUserID      map[string]string
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		usersByEmail:        make(map[string]User),
		usersByID:           make(map[string]User),
		sessionsByTokenHash: make(map[string]Session),
		googleBySubject:     make(map[string]GoogleIdentity),
		googleByUserID:      make(map[string]string),
	}
}

func (m *memoryBackend) Close() error {
	return nil
}

func (m *memoryBackend) CreateUser(_ context.Context, user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.usersByEmail[user.Email]; exists {
		return ErrEmailTaken
	}

	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *memoryBackend) CreateUserAndSession(_ context.Context, user User, session Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.usersByEmail[user.Email]; exists {
		return ErrEmailTaken
	}
	if _, exists := m.sessionsByTokenHash[session.TokenHash]; exists {
		return ErrInvalidSessionToken
	}

	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	m.sessionsByTokenHash[session.TokenHash] = session
	return nil
}

func (m *memoryBackend) CreateUserSessionAndGoogleIdentity(_ context.Context, user User, session Session, identity GoogleIdentity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.usersByEmail[user.Email]; exists {
		return ErrEmailTaken
	}
	if _, exists := m.sessionsByTokenHash[session.TokenHash]; exists {
		return ErrInvalidSessionToken
	}
	if err := m.validateGoogleIdentityForCreateLocked(identity); err != nil {
		return err
	}

	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	m.sessionsByTokenHash[session.TokenHash] = session
	m.upsertGoogleIdentityLocked(identity)

	return nil
}

func (m *memoryBackend) GetUserByEmail(_ context.Context, email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.usersByEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}

	return user, nil
}

func (m *memoryBackend) GetUserByID(_ context.Context, userID string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	return user, nil
}

func (m *memoryBackend) UpdateUserFullName(_ context.Context, userID, fullName string, updatedAt time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	user.FullName = fullName
	user.UpdatedAt = updatedAt.UTC()

	m.usersByID[userID] = user
	m.usersByEmail[user.Email] = user

	return user, nil
}

func (m *memoryBackend) UpdateUserAvatarURL(_ context.Context, userID, avatarURL string, updatedAt time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	user.AvatarURL = avatarURL
	user.UpdatedAt = updatedAt.UTC()

	m.usersByID[userID] = user
	m.usersByEmail[user.Email] = user

	return user, nil
}

func (m *memoryBackend) UpdateUserByAdmin(_ context.Context, userID string, patch AdminUserPatch, updatedAt time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	if patch.FullName != nil {
		user.FullName = *patch.FullName
	}
	if patch.Role != nil {
		user.Role = *patch.Role
	}
	if patch.Plan != nil {
		user.Plan = *patch.Plan
	}
	if patch.PlanExpiresAtSet {
		if patch.PlanExpiresAt == nil {
			user.PlanExpiresAt = nil
		} else {
			expiresAt := patch.PlanExpiresAt.UTC()
			user.PlanExpiresAt = &expiresAt
		}
	}
	user.UpdatedAt = updatedAt.UTC()

	m.usersByID[userID] = user
	m.usersByEmail[user.Email] = user

	return user, nil
}

func (m *memoryBackend) GetUserByGoogleSubject(_ context.Context, googleSubject string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	identity, ok := m.googleBySubject[googleSubject]
	if !ok {
		return User{}, ErrUserNotFound
	}

	user, ok := m.usersByID[identity.UserID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	return user, nil
}

func (m *memoryBackend) ListUsers(_ context.Context, limit, offset int) ([]User, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	allUsers := make([]User, 0, len(m.usersByID))
	for _, user := range m.usersByID {
		allUsers = append(allUsers, user)
	}

	// Sort by CreatedAt DESC
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].CreatedAt.After(allUsers[j].CreatedAt)
	})

	total := len(allUsers)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return allUsers[start:end], total, nil
}

func (m *memoryBackend) GetUserStats(_ context.Context, now time.Time) (UserStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if now.IsZero() {
		now = time.Now().UTC()
	}

	stats := UserStats{}
	for _, user := range m.usersByID {
		stats.TotalUsers++

		switch user.Role {
		case RoleAdmin:
			stats.AdminUsers++
		default:
			stats.MemberUsers++
		}

		switch user.Plan {
		case PlanDaily:
			stats.DailyUsers++
		case PlanWeekly:
			stats.WeeklyUsers++
		case PlanMonthly:
			stats.MonthlyUsers++
		default:
			stats.FreeUsers++
		}

		if user.Plan != PlanFree {
			if user.PlanExpiresAt == nil || !user.PlanExpiresAt.UTC().Before(now.UTC()) {
				stats.ActivePaidUsers++
			}
		}
	}

	return stats, nil
}

func (m *memoryBackend) CreateSession(_ context.Context, session Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessionsByTokenHash[session.TokenHash]; exists {
		return ErrInvalidSessionToken
	}
	m.sessionsByTokenHash[session.TokenHash] = session
	return nil
}

func (m *memoryBackend) GetSessionByTokenHash(_ context.Context, tokenHash string) (Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessionsByTokenHash[tokenHash]
	if !ok {
		return Session{}, ErrSessionNotFound
	}

	return copySession(session), nil
}

func (m *memoryBackend) TouchSession(_ context.Context, tokenHash string, touchedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessionsByTokenHash[tokenHash]
	if !ok {
		return ErrSessionNotFound
	}

	t := touchedAt.UTC()
	session.LastSeenAt = &t
	m.sessionsByTokenHash[tokenHash] = session
	return nil
}

func (m *memoryBackend) RevokeSessionByTokenHash(_ context.Context, tokenHash string, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessionsByTokenHash[tokenHash]
	if !ok {
		return ErrSessionNotFound
	}

	t := revokedAt.UTC()
	session.RevokedAt = &t
	m.sessionsByTokenHash[tokenHash] = session
	return nil
}

func (m *memoryBackend) UpsertGoogleIdentity(_ context.Context, identity GoogleIdentity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validateGoogleIdentityLocked(identity); err != nil {
		return err
	}

	m.upsertGoogleIdentityLocked(identity)
	return nil
}

func (m *memoryBackend) validateGoogleIdentityForCreateLocked(identity GoogleIdentity) error {
	if existing, ok := m.googleBySubject[identity.GoogleSubject]; ok && existing.UserID != identity.UserID {
		return ErrGoogleIdentityConflict
	}
	if existingSubject, ok := m.googleByUserID[identity.UserID]; ok && existingSubject != identity.GoogleSubject {
		return ErrGoogleIdentityConflict
	}
	return nil
}

func (m *memoryBackend) validateGoogleIdentityLocked(identity GoogleIdentity) error {
	if err := m.validateGoogleIdentityForCreateLocked(identity); err != nil {
		return err
	}
	if _, ok := m.usersByID[identity.UserID]; !ok {
		return ErrUserNotFound
	}
	return nil
}

func (m *memoryBackend) upsertGoogleIdentityLocked(identity GoogleIdentity) {
	current, exists := m.googleBySubject[identity.GoogleSubject]
	if exists {
		identity.CreatedAt = current.CreatedAt
	}
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = time.Now().UTC()
	}
	if identity.UpdatedAt.IsZero() {
		identity.UpdatedAt = identity.CreatedAt
	}

	m.googleBySubject[identity.GoogleSubject] = identity
	m.googleByUserID[identity.UserID] = identity.GoogleSubject
}

func copySession(input Session) Session {
	out := input
	if input.RevokedAt != nil {
		v := input.RevokedAt.UTC()
		out.RevokedAt = &v
	}
	if input.LastSeenAt != nil {
		v := input.LastSeenAt.UTC()
		out.LastSeenAt = &v
	}
	return out
}
