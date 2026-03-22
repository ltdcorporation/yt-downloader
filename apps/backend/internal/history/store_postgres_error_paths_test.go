package history

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresBackend_EnsureSchemaErrorBranches(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	// Force ensureSchema to execute against a closed DB so each method exercises
	// the early error-return path.
	_ = backend.db.Close()
	backend.schemaReady = false

	ctx := context.Background()

	checks := []struct {
		name string
		run  func() error
	}{
		{
			name: "UpsertItem",
			run: func() error {
				_, err := backend.UpsertItem(ctx, Item{ID: "his_x", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=x", SourceURLHash: "hash_x", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
				return err
			},
		},
		{
			name: "GetItemByID",
			run: func() error {
				_, err := backend.GetItemByID(ctx, "user_1", "his_x")
				return err
			},
		},
		{
			name: "SoftDeleteItem",
			run: func() error {
				return backend.SoftDeleteItem(ctx, "user_1", "his_x", time.Now().UTC())
			},
		},
		{
			name: "MarkItemSuccess",
			run: func() error {
				return backend.MarkItemSuccess(ctx, "user_1", "his_x", time.Now().UTC())
			},
		},
		{
			name: "CreateAttempt",
			run: func() error {
				return backend.CreateAttempt(ctx, Attempt{ID: "hat_x", HistoryItemID: "his_x", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
			},
		},
		{
			name: "UpdateAttempt",
			run: func() error {
				return backend.UpdateAttempt(ctx, Attempt{ID: "hat_x", HistoryItemID: "his_x", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued, UpdatedAt: time.Now().UTC()})
			},
		},
		{
			name: "GetAttemptByID",
			run: func() error {
				_, err := backend.GetAttemptByID(ctx, "user_1", "hat_x")
				return err
			},
		},
		{
			name: "GetAttemptByJobID",
			run: func() error {
				_, err := backend.GetAttemptByJobID(ctx, "job_x")
				return err
			},
		},
		{
			name: "GetLatestAttemptByItem",
			run: func() error {
				_, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_x")
				return err
			},
		},
		{
			name: "ListItems",
			run: func() error {
				_, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10})
				return err
			},
		},
		{
			name: "GetStats",
			run: func() error {
				_, err := backend.GetStats(ctx, "user_1")
				return err
			},
		},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.run()
			if err == nil {
				t.Fatalf("expected ensure schema error")
			}
			if !strings.Contains(err.Error(), "sql:") && !strings.Contains(err.Error(), "database is closed") && !strings.Contains(err.Error(), "ensure history schema") {
				t.Fatalf("unexpected error for %s: %v", check.name, err)
			}
		})
	}
}
