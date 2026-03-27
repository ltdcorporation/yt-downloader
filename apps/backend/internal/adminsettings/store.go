package adminsettings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

var (
	ErrInvalidInput    = errors.New("invalid admin settings input")
	ErrVersionConflict = errors.New("admin settings version conflict")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "invalid admin settings input"
	}
	return e.Message
}

type Quality string

const (
	Quality4K    Quality = "4k"
	Quality1080p Quality = "1080p"
	Quality720p  Quality = "720p"
	Quality480p  Quality = "480p"
)

type Preferences struct {
	DefaultQuality      Quality
	AutoTrimSilence     bool
	ThumbnailGeneration bool
}

type EmailNotifications struct {
	Processing bool
	Storage    bool
	Summary    bool
}

type Notifications struct {
	Email EmailNotifications
}

type Data struct {
	Preferences   Preferences
	Notifications Notifications
}

type Snapshot struct {
	Data            Data
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	UpdatedByUserID string
}

type PreferencesPatch struct {
	DefaultQuality      *Quality
	AutoTrimSilence     *bool
	ThumbnailGeneration *bool
}

type EmailNotificationsPatch struct {
	Processing *bool
	Storage    *bool
	Summary    *bool
}

type NotificationsPatch struct {
	Email *EmailNotificationsPatch
}

type Patch struct {
	Preferences   *PreferencesPatch
	Notifications *NotificationsPatch
}

type ApplyPatchParams struct {
	Before        Snapshot
	After         Snapshot
	ChangedFields []string
	ActorUserID   string
	RequestID     string
	Source        string
	ChangedAt     time.Time
	AuditID       string
}

type backend interface {
	Close() error
	EnsureReady(ctx context.Context) error
	GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error)
	ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error)
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("admin settings store engine=postgres")
		}
		return &Store{backend: newPostgresBackend(cfg.PostgresDSN)}
	}

	if logger != nil {
		logger.Printf("admin settings store engine=memory (POSTGRES_DSN empty)")
	}
	return &Store{backend: newMemoryBackend()}
}

func (s *Store) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.Close()
}

func (s *Store) EnsureReady(ctx context.Context) error {
	if s == nil || s.backend == nil {
		return errors.New("admin settings store is not initialized")
	}
	return s.backend.EnsureReady(ctx)
}

func (s *Store) GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error) {
	if s == nil || s.backend == nil {
		return Snapshot{}, errors.New("admin settings store is not initialized")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	snapshot, err := s.backend.GetOrCreateSnapshot(ctx, now.UTC())
	if err != nil {
		return Snapshot{}, err
	}
	return normalizeSnapshot(snapshot)
}

func (s *Store) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if s == nil || s.backend == nil {
		return Snapshot{}, errors.New("admin settings store is not initialized")
	}

	before, err := normalizeSnapshot(params.Before)
	if err != nil {
		return Snapshot{}, err
	}
	after, err := normalizeSnapshot(params.After)
	if err != nil {
		return Snapshot{}, err
	}

	if after.Version != before.Version+1 {
		return Snapshot{}, &ValidationError{Message: "after version must be before version + 1"}
	}
	if after.UpdatedAt.IsZero() {
		return Snapshot{}, &ValidationError{Message: "after updated_at is required"}
	}
	if after.CreatedAt.IsZero() {
		after.CreatedAt = before.CreatedAt
	}
	if after.CreatedAt.IsZero() {
		return Snapshot{}, &ValidationError{Message: "after created_at is required"}
	}

	params.Before = before
	params.After = after
	params.ChangedFields = normalizeChangedFields(params.ChangedFields)
	params.ActorUserID = strings.TrimSpace(params.ActorUserID)
	params.RequestID = strings.TrimSpace(params.RequestID)
	params.Source = strings.TrimSpace(strings.ToLower(params.Source))
	if params.Source == "" {
		params.Source = "admin_web"
	}
	if params.ChangedAt.IsZero() {
		params.ChangedAt = time.Now().UTC()
	}

	snapshot, err := s.backend.ApplyPatch(ctx, params)
	if err != nil {
		return Snapshot{}, err
	}
	return normalizeSnapshot(snapshot)
}

func DefaultData() Data {
	return Data{
		Preferences: Preferences{
			DefaultQuality:      Quality1080p,
			AutoTrimSilence:     false,
			ThumbnailGeneration: false,
		},
		Notifications: Notifications{
			Email: EmailNotifications{
				Processing: true,
				Storage:    true,
				Summary:    false,
			},
		},
	}
}

func DefaultSnapshot(now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return Snapshot{
		Data:      DefaultData(),
		Version:   1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
}

func IsValidQuality(raw Quality) bool {
	switch Quality(strings.TrimSpace(strings.ToLower(string(raw)))) {
	case Quality4K, Quality1080p, Quality720p, Quality480p:
		return true
	default:
		return false
	}
}

func normalizeQuality(raw Quality) (Quality, error) {
	quality := Quality(strings.TrimSpace(strings.ToLower(string(raw))))
	if !IsValidQuality(quality) {
		return "", fmt.Errorf("%w: unsupported default_quality", ErrInvalidInput)
	}
	return quality, nil
}

func normalizeSnapshot(snapshot Snapshot) (Snapshot, error) {
	quality, err := normalizeQuality(snapshot.Data.Preferences.DefaultQuality)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Data.Preferences.DefaultQuality = quality

	if snapshot.Version < 1 {
		return Snapshot{}, &ValidationError{Message: "version must be >= 1"}
	}

	snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	snapshot.UpdatedAt = snapshot.UpdatedAt.UTC()
	snapshot.UpdatedByUserID = strings.TrimSpace(snapshot.UpdatedByUserID)

	return snapshot, nil
}

func normalizeChangedFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
