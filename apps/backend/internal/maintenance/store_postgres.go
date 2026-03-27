package maintenance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const maintenanceScopeID = "global"

type postgresBackend struct {
	db *sql.DB

	schemaMu    sync.Mutex
	schemaReady bool
}

func newPostgresBackend(dsn string) *postgresBackend {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(fmt.Sprintf("maintenance postgres open failed: %v", err))
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

func (p *postgresBackend) GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Snapshot{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	defaults := DefaultData()
	_, err := p.db.ExecContext(
		ctx,
		`INSERT INTO maintenance_settings (
			id,
			enabled,
			estimated_downtime,
			public_message,
			version,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,1,$5,$6)
		ON CONFLICT (id) DO NOTHING`,
		maintenanceScopeID,
		defaults.Enabled,
		defaults.EstimatedDowntime,
		defaults.PublicMessage,
		now.UTC(),
		now.UTC(),
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("create default maintenance snapshot: %w", err)
	}

	for _, service := range defaultServiceOverrides() {
		_, err := p.db.ExecContext(
			ctx,
			`INSERT INTO maintenance_service_overrides (
				scope_id,
				service_key,
				display_name,
				status,
				enabled,
				updated_at
			) VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (scope_id, service_key) DO NOTHING`,
			maintenanceScopeID,
			service.Key,
			service.Name,
			service.Status,
			service.Enabled,
			now.UTC(),
		)
		if err != nil {
			return Snapshot{}, fmt.Errorf("create default maintenance services: %w", err)
		}
	}

	settingsRow := p.db.QueryRowContext(
		ctx,
		`SELECT enabled, estimated_downtime, public_message, version, created_at, updated_at, COALESCE(updated_by_user_id, '')
		 FROM maintenance_settings
		 WHERE id = $1`,
		maintenanceScopeID,
	)

	snapshot := Snapshot{}
	if err := settingsRow.Scan(
		&snapshot.Data.Enabled,
		&snapshot.Data.EstimatedDowntime,
		&snapshot.Data.PublicMessage,
		&snapshot.Version,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
		&snapshot.UpdatedByUserID,
	); err != nil {
		return Snapshot{}, fmt.Errorf("select maintenance settings: %w", err)
	}

	servicesRows, err := p.db.QueryContext(
		ctx,
		`SELECT service_key, display_name, status, enabled
		 FROM maintenance_service_overrides
		 WHERE scope_id = $1
		 ORDER BY CASE service_key
		 	WHEN 'api_gateway' THEN 1
		 	WHEN 'primary_database' THEN 2
		 	WHEN 'worker_nodes' THEN 3
		 	ELSE 100 END`,
		maintenanceScopeID,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("select maintenance services: %w", err)
	}
	defer servicesRows.Close()

	services := make([]ServiceOverride, 0, 3)
	for servicesRows.Next() {
		var key string
		var status string
		service := ServiceOverride{}
		if err := servicesRows.Scan(&key, &service.Name, &status, &service.Enabled); err != nil {
			return Snapshot{}, fmt.Errorf("scan maintenance service: %w", err)
		}
		service.Key = ServiceKey(key)
		service.Status = ServiceStatus(status)
		services = append(services, service)
	}
	if err := servicesRows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate maintenance services: %w", err)
	}
	snapshot.Data.Services = services

	return normalizeSnapshot(snapshot)
}

func (p *postgresBackend) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if err := p.ensureSchema(ctx); err != nil {
		return Snapshot{}, err
	}

	beforeJSON, err := json.Marshal(params.Before)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal maintenance before snapshot: %w", err)
	}
	afterJSON, err := json.Marshal(params.After)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal maintenance after snapshot: %w", err)
	}
	changedFieldsJSON, err := json.Marshal(params.ChangedFields)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal maintenance changed fields: %w", err)
	}

	auditID := params.AuditID
	if auditID == "" {
		auditID = "mna_" + uuid.NewString()
	}
	changedAt := params.ChangedAt
	if changedAt.IsZero() {
		changedAt = time.Now().UTC()
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin maintenance patch tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(
		ctx,
		`UPDATE maintenance_settings
		 SET enabled = $3,
		     estimated_downtime = $4,
		     public_message = $5,
		     version = $6,
		     updated_at = $7,
		     updated_by_user_id = $8
		 WHERE id = $1
		   AND version = $2
		 RETURNING enabled, estimated_downtime, public_message, version, created_at, updated_at, COALESCE(updated_by_user_id, '')`,
		maintenanceScopeID,
		params.Before.Version,
		params.After.Data.Enabled,
		params.After.Data.EstimatedDowntime,
		params.After.Data.PublicMessage,
		params.After.Version,
		params.After.UpdatedAt.UTC(),
		nullableString(params.ActorUserID),
	)

	updatedSnapshot := Snapshot{}
	if err := row.Scan(
		&updatedSnapshot.Data.Enabled,
		&updatedSnapshot.Data.EstimatedDowntime,
		&updatedSnapshot.Data.PublicMessage,
		&updatedSnapshot.Version,
		&updatedSnapshot.CreatedAt,
		&updatedSnapshot.UpdatedAt,
		&updatedSnapshot.UpdatedByUserID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, ErrVersionConflict
		}
		return Snapshot{}, fmt.Errorf("update maintenance snapshot: %w", err)
	}

	for _, service := range params.After.Data.Services {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO maintenance_service_overrides (
				scope_id,
				service_key,
				display_name,
				status,
				enabled,
				updated_at
			) VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (scope_id, service_key) DO UPDATE
			SET display_name = EXCLUDED.display_name,
			    status = EXCLUDED.status,
			    enabled = EXCLUDED.enabled,
			    updated_at = EXCLUDED.updated_at`,
			maintenanceScopeID,
			service.Key,
			service.Name,
			service.Status,
			service.Enabled,
			params.After.UpdatedAt.UTC(),
		)
		if err != nil {
			return Snapshot{}, fmt.Errorf("upsert maintenance service override: %w", err)
		}
	}
	updatedSnapshot.Data.Services = params.After.Data.Services

	if len(params.ChangedFields) > 0 {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO maintenance_audit (
				id,
				scope_id,
				actor_user_id,
				request_id,
				source,
				before_json,
				after_json,
				changed_fields,
				created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			auditID,
			maintenanceScopeID,
			nullableString(params.ActorUserID),
			nullableString(params.RequestID),
			params.Source,
			beforeJSON,
			afterJSON,
			changedFieldsJSON,
			changedAt.UTC(),
		)
		if err != nil {
			return Snapshot{}, fmt.Errorf("insert maintenance audit: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit maintenance patch tx: %w", err)
	}

	return normalizeSnapshot(updatedSnapshot)
}

func (p *postgresBackend) ensureSchema(ctx context.Context) error {
	p.schemaMu.Lock()
	defer p.schemaMu.Unlock()

	if p.schemaReady {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS maintenance_settings (
			id TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT FALSE,
			estimated_downtime TEXT NOT NULL,
			public_message TEXT NOT NULL,
			version BIGINT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			updated_by_user_id TEXT REFERENCES auth_users(id) ON DELETE SET NULL,
			CONSTRAINT chk_maintenance_version CHECK (version >= 1),
			CONSTRAINT chk_maintenance_estimated_downtime_length CHECK (char_length(estimated_downtime) <= 120),
			CONSTRAINT chk_maintenance_public_message_length CHECK (char_length(public_message) <= 1000)
		)`,
		`CREATE TABLE IF NOT EXISTS maintenance_service_overrides (
			scope_id TEXT NOT NULL REFERENCES maintenance_settings(id) ON DELETE CASCADE,
			service_key TEXT NOT NULL,
			display_name TEXT NOT NULL,
			status TEXT NOT NULL,
			enabled BOOLEAN NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (scope_id, service_key),
			CONSTRAINT chk_maintenance_service_key CHECK (service_key IN ('api_gateway','primary_database','worker_nodes')),
			CONSTRAINT chk_maintenance_service_status CHECK (status IN ('active','maintenance','scaling','refreshing'))
		)`,
		`CREATE TABLE IF NOT EXISTS maintenance_audit (
			id TEXT PRIMARY KEY,
			scope_id TEXT NOT NULL REFERENCES maintenance_settings(id) ON DELETE CASCADE,
			actor_user_id TEXT REFERENCES auth_users(id) ON DELETE SET NULL,
			request_id TEXT,
			source TEXT NOT NULL,
			before_json JSONB NOT NULL,
			after_json JSONB NOT NULL,
			changed_fields JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_audit_scope_created_at ON maintenance_audit (scope_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_audit_actor_created_at ON maintenance_audit (actor_user_id, created_at DESC)`,
	}

	for _, stmt := range statements {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("maintenance ensure schema failed: %w", err)
		}
	}

	p.schemaReady = true
	return nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
