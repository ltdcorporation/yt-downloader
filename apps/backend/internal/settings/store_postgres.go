package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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
		panic(fmt.Sprintf("settings postgres open failed: %v", err))
	}
	return &postgresBackend{db: db}
}

func (p *postgresBackend) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *postgresBackend) EnsureReady(ctx context.Context) error {
	return p.ensureSchema(ctx)
}

func (p *postgresBackend) GetOrCreateSnapshot(ctx context.Context, userID string, now time.Time) (Snapshot, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Snapshot{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	defaults := DefaultData()

	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO user_settings (
			user_id,
			default_quality,
			auto_trim_silence,
			thumbnail_generation,
			email_alert_processing,
			email_alert_storage,
			email_alert_summary,
			version,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$9)
		ON CONFLICT (user_id) DO NOTHING`,
		userID,
		defaults.Preferences.DefaultQuality,
		defaults.Preferences.AutoTrimSilence,
		defaults.Preferences.ThumbnailGeneration,
		defaults.Notifications.Email.Processing,
		defaults.Notifications.Email.Storage,
		defaults.Notifications.Email.Summary,
		now.UTC(),
		now.UTC(),
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create default settings: %w", err)
	}

	row := p.db.QueryRowContext(
		ctx,
		`SELECT
			user_id,
			default_quality,
			auto_trim_silence,
			thumbnail_generation,
			email_alert_processing,
			email_alert_storage,
			email_alert_summary,
			version,
			created_at,
			updated_at
		 FROM user_settings
		 WHERE user_id = $1`,
		userID,
	)

	snapshot, err := scanSettingsSnapshot(row)
	if err != nil {
		return Snapshot{}, err
	}

	return snapshot, nil
}

func (p *postgresBackend) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Snapshot{}, err
	}

	beforeJSON, err := json.Marshal(params.Before)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal settings before snapshot: %w", err)
	}
	afterJSON, err := json.Marshal(params.After)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal settings after snapshot: %w", err)
	}
	changedFieldsJSON, err := json.Marshal(params.ChangedFields)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal settings changed fields: %w", err)
	}

	auditID := params.AuditID
	if auditID == "" {
		auditID = "usa_" + uuid.NewString()
	}
	changedAt := params.ChangedAt
	if changedAt.IsZero() {
		changedAt = time.Now().UTC()
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin settings patch tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(
		ctx,
		`UPDATE user_settings
		 SET default_quality = $3,
		     auto_trim_silence = $4,
		     thumbnail_generation = $5,
		     email_alert_processing = $6,
		     email_alert_storage = $7,
		     email_alert_summary = $8,
		     version = $9,
		     updated_at = $10
		 WHERE user_id = $1
		   AND version = $2
		 RETURNING
			user_id,
			default_quality,
			auto_trim_silence,
			thumbnail_generation,
			email_alert_processing,
			email_alert_storage,
			email_alert_summary,
			version,
			created_at,
			updated_at`,
		params.After.UserID,
		params.Before.Version,
		params.After.Data.Preferences.DefaultQuality,
		params.After.Data.Preferences.AutoTrimSilence,
		params.After.Data.Preferences.ThumbnailGeneration,
		params.After.Data.Notifications.Email.Processing,
		params.After.Data.Notifications.Email.Storage,
		params.After.Data.Notifications.Email.Summary,
		params.After.Version,
		params.After.UpdatedAt.UTC(),
	)

	updatedSnapshot, err := scanSettingsSnapshot(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, ErrVersionConflict
		}
		return Snapshot{}, fmt.Errorf("update settings snapshot: %w", err)
	}

	if len(params.ChangedFields) > 0 {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO user_settings_audit (
				id,
				user_id,
				actor_user_id,
				request_id,
				source,
				before_json,
				after_json,
				changed_fields,
				created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			auditID,
			updatedSnapshot.UserID,
			params.ActorUserID,
			nullableString(params.RequestID),
			params.Source,
			beforeJSON,
			afterJSON,
			changedFieldsJSON,
			changedAt.UTC(),
		)
		if err != nil {
			return Snapshot{}, fmt.Errorf("insert settings audit: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit settings patch tx: %w", err)
	}

	return updatedSnapshot, nil
}

func scanSettingsSnapshot(row interface{ Scan(dest ...any) error }) (Snapshot, error) {
	var snapshot Snapshot
	var quality string
	if err := row.Scan(
		&snapshot.UserID,
		&quality,
		&snapshot.Data.Preferences.AutoTrimSilence,
		&snapshot.Data.Preferences.ThumbnailGeneration,
		&snapshot.Data.Notifications.Email.Processing,
		&snapshot.Data.Notifications.Email.Storage,
		&snapshot.Data.Notifications.Email.Summary,
		&snapshot.Version,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	); err != nil {
		return Snapshot{}, err
	}
	snapshot.Data.Preferences.DefaultQuality = Quality(quality)
	return snapshot, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id TEXT PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
			default_quality TEXT NOT NULL DEFAULT '1080p',
			auto_trim_silence BOOLEAN NOT NULL DEFAULT FALSE,
			thumbnail_generation BOOLEAN NOT NULL DEFAULT FALSE,
			email_alert_processing BOOLEAN NOT NULL DEFAULT TRUE,
			email_alert_storage BOOLEAN NOT NULL DEFAULT TRUE,
			email_alert_summary BOOLEAN NOT NULL DEFAULT FALSE,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT chk_user_settings_default_quality CHECK (default_quality IN ('4k','1080p','720p','480p')),
			CONSTRAINT chk_user_settings_version CHECK (version >= 1)
		)`,
		`CREATE TABLE IF NOT EXISTS user_settings_audit (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
			actor_user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE RESTRICT,
			request_id TEXT,
			source TEXT NOT NULL,
			before_json JSONB NOT NULL,
			after_json JSONB NOT NULL,
			changed_fields JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_settings_audit_user_id_created_at ON user_settings_audit (user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_settings_audit_actor_user_id_created_at ON user_settings_audit (actor_user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_settings_audit_request_id ON user_settings_audit (request_id) WHERE request_id IS NOT NULL`,
	}

	for _, stmt := range statements {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("settings ensure schema failed: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}
