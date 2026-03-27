package history

import (
	"context"
	"testing"
	"time"
)

func TestPostgresBackendIntegration_EnsureReadyUpgradesResolvedStatusConstraint(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	legacySchema := []string{
		`
		CREATE TABLE history_items (
			id TEXT NOT NULL,
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
			PRIMARY KEY (id),
			CONSTRAINT chk_history_platform CHECK (platform IN ('youtube','tiktok','instagram','x'))
		)
		`,
		`CREATE UNIQUE INDEX idx_history_items_id_user_unique ON history_items (id, user_id)`,
		`CREATE UNIQUE INDEX idx_history_items_user_hash_active ON history_items (user_id, source_url_hash) WHERE deleted_at IS NULL`,
		`
		CREATE TABLE history_attempts (
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
	}

	for _, statement := range legacySchema {
		if _, err := backend.db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("failed to seed legacy schema statement: %v", err)
		}
	}

	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("ensure ready should upgrade legacy history_attempts status constraint: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	item, err := backend.UpsertItem(ctx, Item{
		ID:            "his_legacy_upgrade",
		UserID:        "user_legacy",
		Platform:      PlatformYouTube,
		SourceURL:     "https://www.youtube.com/watch?v=legacy",
		SourceURLHash: "hash_legacy_upgrade",
		Title:         "Legacy Upgrade",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("unexpected upsert item error after schema upgrade: %v", err)
	}

	err = backend.CreateAttempt(ctx, Attempt{
		ID:            "hat_legacy_upgrade",
		HistoryItemID: item.ID,
		UserID:        item.UserID,
		RequestKind:   RequestKindMP4,
		Status:        StatusResolved,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("expected resolved attempt to be accepted after schema upgrade, got %v", err)
	}
}
