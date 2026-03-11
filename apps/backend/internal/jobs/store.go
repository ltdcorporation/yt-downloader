package jobs

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

var ErrNotFound = errors.New("job not found")

type Record struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	InputURL    string     `json:"input_url"`
	OutputKind  string     `json:"output_kind"`
	OutputKey   string     `json:"output_key,omitempty"`
	Title       string     `json:"title,omitempty"`
	Error       string     `json:"error,omitempty"`
	DownloadURL string     `json:"download_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type backend interface {
	Put(ctx context.Context, record Record) error
	Get(ctx context.Context, jobID string) (Record, error)
	ListRecent(ctx context.Context, limit int) ([]Record, error)
	Close() error
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("jobs store engine=postgres")
		}
		return &Store{
			backend: newPostgresBackend(cfg.PostgresDSN, cfg.JobRetentionDays),
		}
	}

	if logger != nil {
		logger.Printf("jobs store engine=redis (POSTGRES_DSN empty)")
	}
	return &Store{
		backend: newRedisBackend(cfg),
	}
}

func (s *Store) Close() error {
	return s.backend.Close()
}

func (s *Store) Put(ctx context.Context, record Record) error {
	if record.ID == "" {
		return errors.New("job id is required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	return s.backend.Put(ctx, record)
}

func (s *Store) Get(ctx context.Context, jobID string) (Record, error) {
	if strings.TrimSpace(jobID) == "" {
		return Record{}, errors.New("job id is required")
	}
	return s.backend.Get(ctx, strings.TrimSpace(jobID))
}

func (s *Store) Update(ctx context.Context, jobID string, mutate func(*Record)) (Record, error) {
	record, err := s.Get(ctx, jobID)
	if err != nil {
		return Record{}, err
	}

	if mutate != nil {
		mutate(&record)
	}
	record.UpdatedAt = time.Now().UTC()

	if err := s.Put(ctx, record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) ListRecent(ctx context.Context, limit int) ([]Record, error) {
	return s.backend.ListRecent(ctx, limit)
}
