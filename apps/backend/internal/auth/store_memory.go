package auth

import (
	"context"
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
