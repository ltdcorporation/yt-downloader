package history

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

func (p *postgresBackend) EnsureReady(ctx context.Context) error {
	return p.ensureSchema(ctx)
}

func (p *postgresBackend) UpsertItem(ctx context.Context, item Item) (Item, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Item{}, err
	}

	var lastAttemptAt any
	if item.LastAttemptAt != nil {
		lastAttemptAt = item.LastAttemptAt.UTC()
	}

	var lastSuccessAt any
	if item.LastSuccessAt != nil {
		lastSuccessAt = item.LastSuccessAt.UTC()
	}

	row := p.db.QueryRowContext(ctx, `
		INSERT INTO history_items (
			id, user_id, platform, source_url, source_url_hash,
			title, thumbnail_url,
			last_attempt_at, last_success_at, attempt_count,
			deleted_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9, $10,
			NULL, $11, $12
		)
		ON CONFLICT (user_id, source_url_hash) WHERE deleted_at IS NULL
		DO UPDATE SET
			platform = EXCLUDED.platform,
			source_url = EXCLUDED.source_url,
			title = CASE
				WHEN EXCLUDED.title IS NOT NULL AND EXCLUDED.title <> '' THEN EXCLUDED.title
				ELSE history_items.title
			END,
			thumbnail_url = CASE
				WHEN EXCLUDED.thumbnail_url IS NOT NULL AND EXCLUDED.thumbnail_url <> '' THEN EXCLUDED.thumbnail_url
				ELSE history_items.thumbnail_url
			END,
			last_attempt_at = COALESCE(EXCLUDED.last_attempt_at, history_items.last_attempt_at),
			last_success_at = COALESCE(EXCLUDED.last_success_at, history_items.last_success_at),
			attempt_count = history_items.attempt_count + CASE WHEN EXCLUDED.attempt_count > 0 THEN EXCLUDED.attempt_count ELSE 1 END,
			updated_at = EXCLUDED.updated_at
		RETURNING
			id, user_id, platform, source_url, source_url_hash,
			title, thumbnail_url,
			last_attempt_at, last_success_at, attempt_count,
			deleted_at, created_at, updated_at
	`,
		item.ID,
		item.UserID,
		item.Platform,
		item.SourceURL,
		item.SourceURLHash,
		item.Title,
		item.ThumbnailURL,
		lastAttemptAt,
		lastSuccessAt,
		item.AttemptCount,
		item.CreatedAt.UTC(),
		item.UpdatedAt.UTC(),
	)

	result, err := scanItem(row)
	if err != nil {
		return Item{}, fmt.Errorf("upsert history item: %w", err)
	}
	return result, nil
}

func (p *postgresBackend) GetItemByID(ctx context.Context, userID, itemID string) (Item, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Item{}, err
	}

	row := p.db.QueryRowContext(ctx, `
		SELECT
			id, user_id, platform, source_url, source_url_hash,
			title, thumbnail_url,
			last_attempt_at, last_success_at, attempt_count,
			deleted_at, created_at, updated_at
		FROM history_items
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, itemID, userID)

	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("read history item: %w", err)
	}
	return item, nil
}

func (p *postgresBackend) SoftDeleteItem(ctx context.Context, userID, itemID string, deletedAt time.Time) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	result, err := p.db.ExecContext(ctx, `
		UPDATE history_items
		SET deleted_at = $3, updated_at = $3
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, itemID, userID, deletedAt.UTC())
	if err != nil {
		return fmt.Errorf("soft-delete history item: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (p *postgresBackend) MarkItemSuccess(ctx context.Context, userID, itemID string, succeededAt time.Time) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	result, err := p.db.ExecContext(ctx, `
		UPDATE history_items
		SET
			last_success_at = CASE
				WHEN last_success_at IS NULL OR last_success_at < $3 THEN $3
				ELSE last_success_at
			END,
			updated_at = $3
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, itemID, userID, succeededAt.UTC())
	if err != nil {
		return fmt.Errorf("mark history item success: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrItemNotFound
	}
	return nil
}

func (p *postgresBackend) CreateAttempt(ctx context.Context, attempt Attempt) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	var sizeBytes any
	if attempt.SizeBytes != nil {
		sizeBytes = *attempt.SizeBytes
	}
	var expiresAt any
	if attempt.ExpiresAt != nil {
		expiresAt = attempt.ExpiresAt.UTC()
	}
	var completedAt any
	if attempt.CompletedAt != nil {
		completedAt = attempt.CompletedAt.UTC()
	}

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO history_attempts (
			id, history_item_id, user_id,
			request_kind, status,
			format_id, quality_label, size_bytes,
			job_id, output_key, download_url, expires_at,
			error_code, error_text,
			created_at, updated_at, completed_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7, $8,
			$9, $10, $11, $12,
			$13, $14,
			$15, $16, $17
		)
	`,
		attempt.ID,
		attempt.HistoryItemID,
		attempt.UserID,
		attempt.RequestKind,
		attempt.Status,
		attempt.FormatID,
		attempt.QualityLabel,
		sizeBytes,
		nullIfEmpty(attempt.JobID),
		nullIfEmpty(attempt.OutputKey),
		nullIfEmpty(attempt.DownloadURL),
		expiresAt,
		nullIfEmpty(attempt.ErrorCode),
		nullIfEmpty(attempt.ErrorText),
		attempt.CreatedAt.UTC(),
		attempt.UpdatedAt.UTC(),
		completedAt,
	)
	if mappedErr := mapPostgresWriteError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("create history attempt: %w", err)
	}

	return nil
}

func (p *postgresBackend) UpdateAttempt(ctx context.Context, attempt Attempt) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}

	var sizeBytes any
	if attempt.SizeBytes != nil {
		sizeBytes = *attempt.SizeBytes
	}
	var expiresAt any
	if attempt.ExpiresAt != nil {
		expiresAt = attempt.ExpiresAt.UTC()
	}
	var completedAt any
	if attempt.CompletedAt != nil {
		completedAt = attempt.CompletedAt.UTC()
	}

	result, err := p.db.ExecContext(ctx, `
		UPDATE history_attempts
		SET
			request_kind = $3,
			status = $4,
			format_id = $5,
			quality_label = $6,
			size_bytes = $7,
			job_id = $8,
			output_key = $9,
			download_url = $10,
			expires_at = $11,
			error_code = $12,
			error_text = $13,
			updated_at = $14,
			completed_at = $15
		WHERE id = $1 AND user_id = $2
	`,
		attempt.ID,
		attempt.UserID,
		attempt.RequestKind,
		attempt.Status,
		nullIfEmpty(attempt.FormatID),
		nullIfEmpty(attempt.QualityLabel),
		sizeBytes,
		nullIfEmpty(attempt.JobID),
		nullIfEmpty(attempt.OutputKey),
		nullIfEmpty(attempt.DownloadURL),
		expiresAt,
		nullIfEmpty(attempt.ErrorCode),
		nullIfEmpty(attempt.ErrorText),
		attempt.UpdatedAt.UTC(),
		completedAt,
	)
	if mappedErr := mapPostgresWriteError(err); mappedErr != nil {
		return mappedErr
	}
	if err != nil {
		return fmt.Errorf("update history attempt: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrAttemptNotFound
	}
	return nil
}

func (p *postgresBackend) GetAttemptByID(ctx context.Context, userID, attemptID string) (Attempt, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Attempt{}, err
	}

	row := p.db.QueryRowContext(ctx, `
		SELECT
			id, history_item_id, user_id,
			request_kind, status,
			format_id, quality_label, size_bytes,
			job_id, output_key, download_url, expires_at,
			error_code, error_text,
			created_at, updated_at, completed_at
		FROM history_attempts
		WHERE id = $1 AND user_id = $2
	`, attemptID, userID)

	attempt, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, ErrAttemptNotFound
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("read history attempt: %w", err)
	}
	return attempt, nil
}

func (p *postgresBackend) GetAttemptByJobID(ctx context.Context, jobID string) (Attempt, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Attempt{}, err
	}

	row := p.db.QueryRowContext(ctx, `
		SELECT
			id, history_item_id, user_id,
			request_kind, status,
			format_id, quality_label, size_bytes,
			job_id, output_key, download_url, expires_at,
			error_code, error_text,
			created_at, updated_at, completed_at
		FROM history_attempts
		WHERE job_id = $1
	`, jobID)

	attempt, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, ErrAttemptNotFound
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("read history attempt by job_id: %w", err)
	}
	return attempt, nil
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS history_items (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			source_url TEXT NOT NULL,
			source_url_hash TEXT NOT NULL,
			title TEXT,
			thumbnail_url TEXT,
			last_attempt_at TIMESTAMPTZ,
			last_success_at TIMESTAMPTZ,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT chk_history_items_platform CHECK (platform IN ('youtube','tiktok','instagram','x'))
		)
		`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_history_items_id_user ON history_items (id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_history_items_user_last_attempt ON history_items (user_id, last_attempt_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_history_items_user_platform_last_attempt ON history_items (user_id, platform, last_attempt_at DESC, id DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_history_items_user_source_hash_active ON history_items (user_id, source_url_hash) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_history_items_active_by_user ON history_items (user_id, id) WHERE deleted_at IS NULL`,
		`
		CREATE TABLE IF NOT EXISTS history_attempts (
			id TEXT PRIMARY KEY,
			history_item_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			request_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			format_id TEXT,
			quality_label TEXT,
			size_bytes BIGINT,
			job_id TEXT,
			output_key TEXT,
			download_url TEXT,
			expires_at TIMESTAMPTZ,
			error_code TEXT,
			error_text TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			CONSTRAINT chk_history_attempt_request_kind CHECK (request_kind IN ('mp3','mp4','image')),
			CONSTRAINT chk_history_attempt_status CHECK (status IN ('queued','processing','done','failed','expired')),
			CONSTRAINT fk_history_attempt_item_user FOREIGN KEY (history_item_id, user_id)
				REFERENCES history_items(id, user_id)
				ON DELETE CASCADE
		)
		`,
		`CREATE INDEX IF NOT EXISTS idx_history_attempts_user_created ON history_attempts (user_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_history_attempts_item_created ON history_attempts (history_item_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_history_attempts_user_status_created ON history_attempts (user_id, status, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_history_attempts_job_id_unique ON history_attempts (job_id) WHERE job_id IS NOT NULL`,
	}

	for _, statement := range statements {
		if _, err := p.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure history schema: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func scanItem(row interface{ Scan(dest ...any) error }) (Item, error) {
	var item Item
	var platform string
	var title sql.NullString
	var thumbnailURL sql.NullString
	var lastAttemptAt sql.NullTime
	var lastSuccessAt sql.NullTime
	var deletedAt sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.UserID,
		&platform,
		&item.SourceURL,
		&item.SourceURLHash,
		&title,
		&thumbnailURL,
		&lastAttemptAt,
		&lastSuccessAt,
		&item.AttemptCount,
		&deletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Item{}, err
	}

	item.Platform = Platform(platform)
	item.Title = title.String
	item.ThumbnailURL = thumbnailURL.String
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time.UTC()
		item.LastAttemptAt = &t
	}
	if lastSuccessAt.Valid {
		t := lastSuccessAt.Time.UTC()
		item.LastSuccessAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		item.DeletedAt = &t
	}

	return item, nil
}

func scanAttempt(row interface{ Scan(dest ...any) error }) (Attempt, error) {
	var attempt Attempt
	var requestKind string
	var status string
	var formatID sql.NullString
	var qualityLabel sql.NullString
	var sizeBytes sql.NullInt64
	var jobID sql.NullString
	var outputKey sql.NullString
	var downloadURL sql.NullString
	var expiresAt sql.NullTime
	var errorCode sql.NullString
	var errorText sql.NullString
	var completedAt sql.NullTime

	err := row.Scan(
		&attempt.ID,
		&attempt.HistoryItemID,
		&attempt.UserID,
		&requestKind,
		&status,
		&formatID,
		&qualityLabel,
		&sizeBytes,
		&jobID,
		&outputKey,
		&downloadURL,
		&expiresAt,
		&errorCode,
		&errorText,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
		&completedAt,
	)
	if err != nil {
		return Attempt{}, err
	}

	attempt.RequestKind = RequestKind(requestKind)
	attempt.Status = AttemptStatus(status)
	attempt.FormatID = formatID.String
	attempt.QualityLabel = qualityLabel.String
	if sizeBytes.Valid {
		value := sizeBytes.Int64
		attempt.SizeBytes = &value
	}
	attempt.JobID = jobID.String
	attempt.OutputKey = outputKey.String
	attempt.DownloadURL = downloadURL.String
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		attempt.ExpiresAt = &t
	}
	attempt.ErrorCode = errorCode.String
	attempt.ErrorText = errorText.String
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		attempt.CompletedAt = &t
	}

	return attempt, nil
}

func mapPostgresWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}

	switch pgErr.Code {
	case "23505":
		return ErrConflict
	case "23503":
		if strings.Contains(strings.ToLower(pgErr.ConstraintName), "fk_history_attempt_item_user") {
			return ErrItemNotFound
		}
	}

	return nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
