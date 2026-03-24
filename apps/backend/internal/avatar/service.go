package avatar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"yt-downloader/backend/internal/auth"
)

const (
	DefaultMaxUploadBytes int64 = 2 * 1024 * 1024
	DefaultTargetSize           = 512

	defaultDeleteAttempts = 3
	defaultDeleteBackoff  = 250 * time.Millisecond
)

var (
	ErrPayloadTooLarge  = errors.New("avatar payload exceeds max upload size")
	ErrPayloadEmpty     = errors.New("avatar payload is required")
	ErrInvalidImage     = errors.New("avatar image is invalid")
	ErrDeleteFailed     = errors.New("avatar object delete failed")
	ErrRollbackFailed   = errors.New("avatar rollback failed")
	ErrInvalidAvatarURL = errors.New("invalid avatar configuration")
	ErrServiceNotReady  = errors.New("avatar service is not initialized")

	readRandomBytes = rand.Read
)

type userStore interface {
	GetUserByID(ctx context.Context, userID string) (auth.User, error)
	UpdateUserAvatarURL(ctx context.Context, userID, avatarURL string, updatedAt time.Time) (auth.User, error)
}

type objectStore interface {
	UploadObject(ctx context.Context, key, contentType string, body []byte) error
	DeleteObject(ctx context.Context, key string) error
}

type imageProcessor interface {
	Normalize(ctx context.Context, raw []byte) ([]byte, error)
}

type Options struct {
	PublicBaseURL  string
	KeyPrefix      string
	MaxUploadBytes int64
	DeleteAttempts int
	DeleteBackoff  time.Duration
}

type Service struct {
	store          userStore
	objectStore    objectStore
	processor      imageProcessor
	publicBaseURL  *url.URL
	keyPrefix      string
	maxUploadBytes int64
	deleteAttempts int
	deleteBackoff  time.Duration
	now            func() time.Time
}

func NewService(store userStore, objectStore objectStore, processor imageProcessor, opts Options) (*Service, error) {
	if store == nil || objectStore == nil || processor == nil {
		return nil, fmt.Errorf("%w: missing dependency", ErrServiceNotReady)
	}

	publicBase := strings.TrimSpace(opts.PublicBaseURL)
	if publicBase == "" {
		return nil, fmt.Errorf("%w: public base URL is required", ErrInvalidAvatarURL)
	}
	parsedBase, err := url.Parse(publicBase)
	if err != nil {
		return nil, fmt.Errorf("%w: parse base url: %v", ErrInvalidAvatarURL, err)
	}
	if strings.TrimSpace(parsedBase.Scheme) == "" || strings.TrimSpace(parsedBase.Host) == "" {
		return nil, fmt.Errorf("%w: base URL must include scheme and host", ErrInvalidAvatarURL)
	}
	parsedBase.RawQuery = ""
	parsedBase.Fragment = ""

	keyPrefix := strings.Trim(strings.TrimSpace(opts.KeyPrefix), "/")
	if keyPrefix == "" {
		keyPrefix = "avatars"
	}

	maxUploadBytes := opts.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = DefaultMaxUploadBytes
	}

	deleteAttempts := opts.DeleteAttempts
	if deleteAttempts <= 0 {
		deleteAttempts = defaultDeleteAttempts
	}

	deleteBackoff := opts.DeleteBackoff
	if deleteBackoff <= 0 {
		deleteBackoff = defaultDeleteBackoff
	}

	return &Service{
		store:          store,
		objectStore:    objectStore,
		processor:      processor,
		publicBaseURL:  parsedBase,
		keyPrefix:      keyPrefix,
		maxUploadBytes: maxUploadBytes,
		deleteAttempts: deleteAttempts,
		deleteBackoff:  deleteBackoff,
		now:            func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *Service) MaxUploadBytes() int64 {
	if s == nil {
		return DefaultMaxUploadBytes
	}
	if s.maxUploadBytes <= 0 {
		return DefaultMaxUploadBytes
	}
	return s.maxUploadBytes
}

func (s *Service) ReplaceAvatar(ctx context.Context, userID string, rawPayload []byte) (auth.PublicUser, error) {
	if s == nil || s.store == nil || s.objectStore == nil || s.processor == nil {
		return auth.PublicUser{}, ErrServiceNotReady
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return auth.PublicUser{}, &auth.ValidationError{Message: "user_id is required"}
	}
	if len(rawPayload) == 0 {
		return auth.PublicUser{}, ErrPayloadEmpty
	}
	if int64(len(rawPayload)) > s.maxUploadBytes {
		return auth.PublicUser{}, ErrPayloadTooLarge
	}

	normalizedAvatar, err := s.processor.Normalize(ctx, rawPayload)
	if err != nil {
		return auth.PublicUser{}, err
	}

	currentUser, err := s.store.GetUserByID(ctx, trimmedUserID)
	if err != nil {
		return auth.PublicUser{}, err
	}

	newObjectKey, err := s.generateObjectKey(trimmedUserID)
	if err != nil {
		return auth.PublicUser{}, fmt.Errorf("generate avatar object key: %w", err)
	}
	newAvatarURL := s.buildObjectURL(newObjectKey)

	if err := s.objectStore.UploadObject(ctx, newObjectKey, "image/webp", normalizedAvatar); err != nil {
		return auth.PublicUser{}, err
	}

	updatedUser, err := s.store.UpdateUserAvatarURL(ctx, trimmedUserID, newAvatarURL, s.now())
	if err != nil {
		cleanupErr := s.deleteObjectStrict(ctx, newObjectKey)
		if cleanupErr != nil {
			return auth.PublicUser{}, fmt.Errorf("update avatar url: %w (cleanup failed: %v)", err, cleanupErr)
		}
		return auth.PublicUser{}, err
	}

	oldObjectKey, oldManaged := s.extractManagedObjectKey(currentUser.AvatarURL)
	if !oldManaged || oldObjectKey == "" || oldObjectKey == newObjectKey {
		return toPublicUser(updatedUser), nil
	}

	if err := s.deleteObjectStrict(ctx, oldObjectKey); err != nil {
		rollbackErr := s.rollbackAvatarURL(ctx, trimmedUserID, currentUser.AvatarURL)
		cleanupErr := s.deleteObjectStrict(ctx, newObjectKey)
		if rollbackErr != nil || cleanupErr != nil {
			return auth.PublicUser{}, fmt.Errorf("%w: delete old avatar key=%s err=%v rollback_err=%v cleanup_new_err=%v", ErrRollbackFailed, oldObjectKey, err, rollbackErr, cleanupErr)
		}
		return auth.PublicUser{}, fmt.Errorf("%w: delete old avatar key=%s: %v", ErrDeleteFailed, oldObjectKey, err)
	}

	return toPublicUser(updatedUser), nil
}

func (s *Service) RemoveAvatar(ctx context.Context, userID string) (auth.PublicUser, error) {
	if s == nil || s.store == nil || s.objectStore == nil {
		return auth.PublicUser{}, ErrServiceNotReady
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return auth.PublicUser{}, &auth.ValidationError{Message: "user_id is required"}
	}

	currentUser, err := s.store.GetUserByID(ctx, trimmedUserID)
	if err != nil {
		return auth.PublicUser{}, err
	}
	if strings.TrimSpace(currentUser.AvatarURL) == "" {
		return toPublicUser(currentUser), nil
	}

	updatedUser, err := s.store.UpdateUserAvatarURL(ctx, trimmedUserID, "", s.now())
	if err != nil {
		return auth.PublicUser{}, err
	}

	oldObjectKey, managed := s.extractManagedObjectKey(currentUser.AvatarURL)
	if !managed || oldObjectKey == "" {
		return toPublicUser(updatedUser), nil
	}

	if err := s.deleteObjectStrict(ctx, oldObjectKey); err != nil {
		rollbackErr := s.rollbackAvatarURL(ctx, trimmedUserID, currentUser.AvatarURL)
		if rollbackErr != nil {
			return auth.PublicUser{}, fmt.Errorf("%w: delete avatar key=%s err=%v rollback_err=%v", ErrRollbackFailed, oldObjectKey, err, rollbackErr)
		}
		return auth.PublicUser{}, fmt.Errorf("%w: delete avatar key=%s: %v", ErrDeleteFailed, oldObjectKey, err)
	}

	return toPublicUser(updatedUser), nil
}

func (s *Service) rollbackAvatarURL(ctx context.Context, userID, avatarURL string) error {
	_, err := s.store.UpdateUserAvatarURL(ctx, userID, strings.TrimSpace(avatarURL), s.now())
	return err
}

func (s *Service) deleteObjectStrict(ctx context.Context, objectKey string) error {
	trimmedKey := strings.TrimSpace(objectKey)
	if trimmedKey == "" {
		return fmt.Errorf("%w: object key is required", ErrDeleteFailed)
	}

	var lastErr error
	for attempt := 1; attempt <= s.deleteAttempts; attempt++ {
		err := s.objectStore.DeleteObject(ctx, trimmedKey)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt >= s.deleteAttempts {
			break
		}

		timer := time.NewTimer(s.deleteBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("%w: key=%s attempts=%d last_error=%v", ErrDeleteFailed, trimmedKey, s.deleteAttempts, lastErr)
}

func (s *Service) generateObjectKey(userID string) (string, error) {
	randomPart := make([]byte, 8)
	if _, err := readRandomBytes(randomPart); err != nil {
		return "", err
	}
	stamp := s.now().UTC().Format("20060102T150405.000000000")
	safeUserID := sanitizeKeyPart(userID)
	objectName := fmt.Sprintf("%s-%s.webp", stamp, hex.EncodeToString(randomPart))
	return strings.Trim(s.keyPrefix+"/"+safeUserID+"/"+objectName, "/"), nil
}

func sanitizeKeyPart(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown"
	}
	builder := strings.Builder{}
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	output := strings.Trim(builder.String(), "_")
	if output == "" {
		return "unknown"
	}
	return output
}

func (s *Service) buildObjectURL(objectKey string) string {
	base := *s.publicBaseURL
	basePath := strings.Trim(base.Path, "/")
	trimmedKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if basePath == "" {
		base.Path = "/" + trimmedKey
	} else {
		base.Path = "/" + strings.Trim(basePath+"/"+trimmedKey, "/")
	}
	return base.String()
}

func (s *Service) extractManagedObjectKey(rawURL string) (string, bool) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmedURL)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, s.publicBaseURL.Scheme) || !strings.EqualFold(parsed.Host, s.publicBaseURL.Host) {
		return "", false
	}

	basePath := strings.Trim(s.publicBaseURL.Path, "/")
	path := strings.Trim(parsed.Path, "/")
	if basePath != "" {
		if path == basePath {
			return "", false
		}
		prefix := basePath + "/"
		if !strings.HasPrefix(path, prefix) {
			return "", false
		}
		path = strings.TrimPrefix(path, prefix)
	}

	path = strings.Trim(path, "/")
	if path == "" {
		return "", false
	}
	if !strings.HasPrefix(path, s.keyPrefix+"/") {
		return "", false
	}

	return path, true
}

func toPublicUser(user auth.User) auth.PublicUser {
	return auth.PublicUser{
		ID:        user.ID,
		FullName:  user.FullName,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		CreatedAt: user.CreatedAt,
	}
}
