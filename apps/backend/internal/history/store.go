package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

const (
	PlatformYouTube   Platform = "youtube"
	PlatformTikTok    Platform = "tiktok"
	PlatformInstagram Platform = "instagram"
	PlatformX         Platform = "x"
)

const (
	RequestKindMP3   RequestKind = "mp3"
	RequestKindMP4   RequestKind = "mp4"
	RequestKindImage RequestKind = "image"
)

const (
	StatusQueued     AttemptStatus = "queued"
	StatusProcessing AttemptStatus = "processing"
	StatusDone       AttemptStatus = "done"
	StatusFailed     AttemptStatus = "failed"
	StatusExpired    AttemptStatus = "expired"
)

var (
	ErrItemNotFound    = errors.New("history item not found")
	ErrAttemptNotFound = errors.New("history attempt not found")
	ErrConflict        = errors.New("history conflict")
	ErrInvalidInput    = errors.New("invalid history input")
)

type Platform string

type RequestKind string

type AttemptStatus string

type Item struct {
	ID            string
	UserID        string
	Platform      Platform
	SourceURL     string
	SourceURLHash string
	Title         string
	ThumbnailURL  string
	LastAttemptAt *time.Time
	LastSuccessAt *time.Time
	AttemptCount  int
	DeletedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Attempt struct {
	ID            string
	HistoryItemID string
	UserID        string
	RequestKind   RequestKind
	Status        AttemptStatus
	FormatID      string
	QualityLabel  string
	SizeBytes     *int64
	JobID         string
	OutputKey     string
	DownloadURL   string
	ExpiresAt     *time.Time
	ErrorCode     string
	ErrorText     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time
}

type backend interface {
	Close() error
	EnsureReady(ctx context.Context) error
	UpsertItem(ctx context.Context, item Item) (Item, error)
	GetItemByID(ctx context.Context, userID, itemID string) (Item, error)
	SoftDeleteItem(ctx context.Context, userID, itemID string, deletedAt time.Time) error
	MarkItemSuccess(ctx context.Context, userID, itemID string, succeededAt time.Time) error
	CreateAttempt(ctx context.Context, attempt Attempt) error
	UpdateAttempt(ctx context.Context, attempt Attempt) error
	GetAttemptByID(ctx context.Context, userID, attemptID string) (Attempt, error)
	GetAttemptByJobID(ctx context.Context, jobID string) (Attempt, error)
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("history store engine=postgres")
		}
		return &Store{backend: newPostgresBackend(cfg.PostgresDSN)}
	}

	if logger != nil {
		logger.Printf("history store engine=memory (POSTGRES_DSN empty)")
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
		return errors.New("history store is not initialized")
	}
	return s.backend.EnsureReady(ctx)
}

func (s *Store) UpsertItem(ctx context.Context, item Item) (Item, error) {
	if s == nil || s.backend == nil {
		return Item{}, errors.New("history store is not initialized")
	}

	item.ID = strings.TrimSpace(item.ID)
	item.UserID = strings.TrimSpace(item.UserID)
	item.SourceURL = normalizeSourceURL(item.SourceURL)
	item.SourceURLHash = strings.TrimSpace(strings.ToLower(item.SourceURLHash))
	item.Title = strings.TrimSpace(item.Title)
	item.ThumbnailURL = strings.TrimSpace(item.ThumbnailURL)

	if item.ID == "" {
		return Item{}, fmt.Errorf("%w: item id is required", ErrInvalidInput)
	}
	if item.UserID == "" {
		return Item{}, fmt.Errorf("%w: user_id is required", ErrInvalidInput)
	}
	if !isValidPlatform(item.Platform) {
		return Item{}, fmt.Errorf("%w: platform is invalid", ErrInvalidInput)
	}
	if item.SourceURL == "" {
		return Item{}, fmt.Errorf("%w: source_url is required", ErrInvalidInput)
	}
	if item.SourceURLHash == "" {
		item.SourceURLHash = hashSourceURL(item.SourceURL)
	}

	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}

	return s.backend.UpsertItem(ctx, item)
}

func (s *Store) GetItemByID(ctx context.Context, userID, itemID string) (Item, error) {
	if s == nil || s.backend == nil {
		return Item{}, errors.New("history store is not initialized")
	}
	trimmedUserID := strings.TrimSpace(userID)
	trimmedItemID := strings.TrimSpace(itemID)
	if trimmedUserID == "" || trimmedItemID == "" {
		return Item{}, fmt.Errorf("%w: user_id and item_id are required", ErrInvalidInput)
	}
	return s.backend.GetItemByID(ctx, trimmedUserID, trimmedItemID)
}

func (s *Store) SoftDeleteItem(ctx context.Context, userID, itemID string, deletedAt time.Time) error {
	if s == nil || s.backend == nil {
		return errors.New("history store is not initialized")
	}
	trimmedUserID := strings.TrimSpace(userID)
	trimmedItemID := strings.TrimSpace(itemID)
	if trimmedUserID == "" || trimmedItemID == "" {
		return fmt.Errorf("%w: user_id and item_id are required", ErrInvalidInput)
	}
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}
	return s.backend.SoftDeleteItem(ctx, trimmedUserID, trimmedItemID, deletedAt.UTC())
}

func (s *Store) MarkItemSuccess(ctx context.Context, userID, itemID string, succeededAt time.Time) error {
	if s == nil || s.backend == nil {
		return errors.New("history store is not initialized")
	}
	trimmedUserID := strings.TrimSpace(userID)
	trimmedItemID := strings.TrimSpace(itemID)
	if trimmedUserID == "" || trimmedItemID == "" {
		return fmt.Errorf("%w: user_id and item_id are required", ErrInvalidInput)
	}
	if succeededAt.IsZero() {
		succeededAt = time.Now().UTC()
	}
	return s.backend.MarkItemSuccess(ctx, trimmedUserID, trimmedItemID, succeededAt.UTC())
}

func (s *Store) CreateAttempt(ctx context.Context, attempt Attempt) (Attempt, error) {
	if s == nil || s.backend == nil {
		return Attempt{}, errors.New("history store is not initialized")
	}

	attempt.ID = strings.TrimSpace(attempt.ID)
	attempt.HistoryItemID = strings.TrimSpace(attempt.HistoryItemID)
	attempt.UserID = strings.TrimSpace(attempt.UserID)
	attempt.FormatID = strings.TrimSpace(attempt.FormatID)
	attempt.QualityLabel = strings.TrimSpace(attempt.QualityLabel)
	attempt.JobID = strings.TrimSpace(attempt.JobID)
	attempt.OutputKey = strings.TrimSpace(attempt.OutputKey)
	attempt.DownloadURL = strings.TrimSpace(attempt.DownloadURL)
	attempt.ErrorCode = strings.TrimSpace(attempt.ErrorCode)
	attempt.ErrorText = strings.TrimSpace(attempt.ErrorText)

	if attempt.ID == "" {
		return Attempt{}, fmt.Errorf("%w: attempt id is required", ErrInvalidInput)
	}
	if attempt.HistoryItemID == "" {
		return Attempt{}, fmt.Errorf("%w: history_item_id is required", ErrInvalidInput)
	}
	if attempt.UserID == "" {
		return Attempt{}, fmt.Errorf("%w: user_id is required", ErrInvalidInput)
	}
	if !isValidRequestKind(attempt.RequestKind) {
		return Attempt{}, fmt.Errorf("%w: request_kind is invalid", ErrInvalidInput)
	}
	if !isValidAttemptStatus(attempt.Status) {
		return Attempt{}, fmt.Errorf("%w: status is invalid", ErrInvalidInput)
	}

	now := time.Now().UTC()
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = now
	}
	if attempt.UpdatedAt.IsZero() {
		attempt.UpdatedAt = now
	}

	if err := s.backend.CreateAttempt(ctx, attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

func (s *Store) GetAttemptByID(ctx context.Context, userID, attemptID string) (Attempt, error) {
	if s == nil || s.backend == nil {
		return Attempt{}, errors.New("history store is not initialized")
	}
	trimmedUserID := strings.TrimSpace(userID)
	trimmedAttemptID := strings.TrimSpace(attemptID)
	if trimmedUserID == "" || trimmedAttemptID == "" {
		return Attempt{}, fmt.Errorf("%w: user_id and attempt_id are required", ErrInvalidInput)
	}
	return s.backend.GetAttemptByID(ctx, trimmedUserID, trimmedAttemptID)
}

func (s *Store) GetAttemptByJobID(ctx context.Context, jobID string) (Attempt, error) {
	if s == nil || s.backend == nil {
		return Attempt{}, errors.New("history store is not initialized")
	}
	trimmedJobID := strings.TrimSpace(jobID)
	if trimmedJobID == "" {
		return Attempt{}, fmt.Errorf("%w: job_id is required", ErrInvalidInput)
	}
	return s.backend.GetAttemptByJobID(ctx, trimmedJobID)
}

func (s *Store) UpdateAttempt(ctx context.Context, userID, attemptID string, mutate func(*Attempt)) (Attempt, error) {
	if s == nil || s.backend == nil {
		return Attempt{}, errors.New("history store is not initialized")
	}

	current, err := s.GetAttemptByID(ctx, userID, attemptID)
	if err != nil {
		return Attempt{}, err
	}

	if mutate != nil {
		mutate(&current)
	}

	if !isValidRequestKind(current.RequestKind) {
		return Attempt{}, fmt.Errorf("%w: request_kind is invalid", ErrInvalidInput)
	}
	if !isValidAttemptStatus(current.Status) {
		return Attempt{}, fmt.Errorf("%w: status is invalid", ErrInvalidInput)
	}

	current.FormatID = strings.TrimSpace(current.FormatID)
	current.QualityLabel = strings.TrimSpace(current.QualityLabel)
	current.JobID = strings.TrimSpace(current.JobID)
	current.OutputKey = strings.TrimSpace(current.OutputKey)
	current.DownloadURL = strings.TrimSpace(current.DownloadURL)
	current.ErrorCode = strings.TrimSpace(current.ErrorCode)
	current.ErrorText = strings.TrimSpace(current.ErrorText)
	current.UpdatedAt = time.Now().UTC()

	if err := s.backend.UpdateAttempt(ctx, current); err != nil {
		return Attempt{}, err
	}
	return current, nil
}

func normalizeSourceURL(raw string) string {
	input := strings.TrimSpace(raw)
	if input == "" {
		return ""
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return input
	}

	if parsed.Scheme != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
	}
	if parsed.Host != "" {
		parsed.Host = strings.ToLower(parsed.Host)
	}
	parsed.Fragment = ""

	if parsed.RawQuery != "" {
		query, err := url.ParseQuery(parsed.RawQuery)
		if err == nil {
			parsed.RawQuery = query.Encode()
		}
	}

	return parsed.String()
}

func hashSourceURL(sourceURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceURL)))
	return hex.EncodeToString(sum[:])
}

func isValidPlatform(platform Platform) bool {
	switch platform {
	case PlatformYouTube, PlatformTikTok, PlatformInstagram, PlatformX:
		return true
	default:
		return false
	}
}

func isValidRequestKind(kind RequestKind) bool {
	switch kind {
	case RequestKindMP3, RequestKindMP4, RequestKindImage:
		return true
	default:
		return false
	}
}

func isValidAttemptStatus(status AttemptStatus) bool {
	switch status {
	case StatusQueued, StatusProcessing, StatusDone, StatusFailed, StatusExpired:
		return true
	default:
		return false
	}
}
