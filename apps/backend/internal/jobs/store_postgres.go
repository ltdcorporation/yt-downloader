package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresBackend struct {
	db            *sql.DB
	retentionDays int

	schemaMu    sync.Mutex
	schemaReady bool

	cleanupMu   sync.Mutex
	lastCleanup time.Time
}

func newPostgresBackend(dsn string, retentionDays int) *postgresBackend {
	if retentionDays <= 0 {
		retentionDays = 14
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		// Driver registration failure is non-recoverable in this process.
		panic(err)
	}

	return &postgresBackend{
		db:            db,
		retentionDays: retentionDays,
	}
}

func (p *postgresBackend) Close() error {
	return p.db.Close()
}

func (p *postgresBackend) Put(ctx context.Context, record Record) error {
	if err := p.ensureSchema(ctx); err != nil {
		return err
	}
	_ = p.cleanupIfDue(ctx)

	var expiresAt any
	if record.ExpiresAt != nil {
		expiresAt = *record.ExpiresAt
	}

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, status, input_url, output_kind, output_key, title,
			error_text, download_url, created_at, updated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			input_url = EXCLUDED.input_url,
			output_kind = EXCLUDED.output_kind,
			output_key = EXCLUDED.output_key,
			title = EXCLUDED.title,
			error_text = EXCLUDED.error_text,
			download_url = EXCLUDED.download_url,
			updated_at = EXCLUDED.updated_at,
			expires_at = EXCLUDED.expires_at
	`,
		record.ID,
		record.Status,
		record.InputURL,
		record.OutputKind,
		record.OutputKey,
		record.Title,
		record.Error,
		record.DownloadURL,
		record.CreatedAt,
		record.UpdatedAt,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("upsert job record: %w", err)
	}

	if record.Status == StatusFailed && strings.TrimSpace(record.Error) != "" {
		_, _ = p.db.ExecContext(
			ctx,
			`INSERT INTO job_errors (job_id, error_text, created_at) VALUES ($1, $2, $3)`,
			record.ID,
			record.Error,
			record.UpdatedAt,
		)
	}

	return nil
}

func (p *postgresBackend) Get(ctx context.Context, jobID string) (Record, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Record{}, err
	}

	row := p.db.QueryRowContext(ctx, `
		SELECT
			id, status, input_url, output_kind, output_key, title,
			error_text, download_url, created_at, updated_at, expires_at
		FROM jobs
		WHERE id = $1
	`, jobID)

	var record Record
	var outputKey sql.NullString
	var title sql.NullString
	var errorText sql.NullString
	var downloadURL sql.NullString
	var expiresAt sql.NullTime
	err := row.Scan(
		&record.ID,
		&record.Status,
		&record.InputURL,
		&record.OutputKind,
		&outputKey,
		&title,
		&errorText,
		&downloadURL,
		&record.CreatedAt,
		&record.UpdatedAt,
		&expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read job record: %w", err)
	}

	record.OutputKey = outputKey.String
	record.Title = title.String
	record.Error = errorText.String
	record.DownloadURL = downloadURL.String
	if expiresAt.Valid {
		value := expiresAt.Time.UTC()
		record.ExpiresAt = &value
	}
	return record, nil
}

func (p *postgresBackend) ListRecent(ctx context.Context, limit int) ([]Record, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return nil, err
	}
	_ = p.cleanupIfDue(ctx)

	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT
			id, status, input_url, output_kind, output_key, title,
			error_text, download_url, created_at, updated_at, expires_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent jobs: %w", err)
	}
	defer rows.Close()

	items := make([]Record, 0, limit)
	for rows.Next() {
		var record Record
		var outputKey sql.NullString
		var title sql.NullString
		var errorText sql.NullString
		var downloadURL sql.NullString
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&record.ID,
			&record.Status,
			&record.InputURL,
			&record.OutputKind,
			&outputKey,
			&title,
			&errorText,
			&downloadURL,
			&record.CreatedAt,
			&record.UpdatedAt,
			&expiresAt,
		); err != nil {
			return nil, fmt.Errorf("decode job row: %w", err)
		}
		record.OutputKey = outputKey.String
		record.Title = title.String
		record.Error = errorText.String
		record.DownloadURL = downloadURL.String
		if expiresAt.Valid {
			value := expiresAt.Time.UTC()
			record.ExpiresAt = &value
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job rows: %w", err)
	}

	return items, nil
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			input_url TEXT NOT NULL,
			output_kind TEXT NOT NULL,
			output_key TEXT,
			title TEXT,
			error_text TEXT,
			download_url TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			expires_at TIMESTAMPTZ
		)
		`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs (created_at DESC)`,
		`
		CREATE TABLE IF NOT EXISTS job_errors (
			id BIGSERIAL PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			error_text TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`CREATE INDEX IF NOT EXISTS idx_job_errors_created_at ON job_errors (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_job_errors_job_id ON job_errors (job_id)`,
	}

	for _, statement := range statements {
		if _, err := p.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure job schema: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func (p *postgresBackend) cleanupIfDue(ctx context.Context) error {
	p.cleanupMu.Lock()
	if time.Since(p.lastCleanup) < time.Hour {
		p.cleanupMu.Unlock()
		return nil
	}
	p.lastCleanup = time.Now().UTC()
	p.cleanupMu.Unlock()

	_, err := p.db.ExecContext(
		ctx,
		`DELETE FROM job_errors WHERE created_at < NOW() - ($1 * INTERVAL '1 day')`,
		p.retentionDays,
	)
	if err != nil {
		return fmt.Errorf("cleanup job_errors retention: %w", err)
	}

	_, err = p.db.ExecContext(
		ctx,
		`DELETE FROM jobs WHERE created_at < NOW() - ($1 * INTERVAL '1 day')`,
		p.retentionDays,
	)
	if err != nil {
		return fmt.Errorf("cleanup jobs retention: %w", err)
	}

	return nil
}
