package history

import (
	stdsql "database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type scannerFunc func(dest ...any) error

func (f scannerFunc) Scan(dest ...any) error {
	return f(dest...)
}

func TestPostgresBackend_RuntimeErrorBranchesAfterSchemaReady(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx, cancel := integrationContext(t)
	defer cancel()

	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("unexpected ensure ready error: %v", err)
	}

	if _, err := backend.db.ExecContext(ctx, `DROP TABLE IF EXISTS history_attempts`); err != nil {
		t.Fatalf("failed to drop history_attempts: %v", err)
	}
	if _, err := backend.db.ExecContext(ctx, `DROP TABLE IF EXISTS history_items`); err != nil {
		t.Fatalf("failed to drop history_items: %v", err)
	}

	now := time.Now().UTC()

	tests := []struct {
		name        string
		run         func() error
		expectInErr string
	}{
		{
			name: "UpsertItem",
			run: func() error {
				_, err := backend.UpsertItem(ctx, Item{ID: "his_rt", UserID: "user_1", Platform: PlatformYouTube, SourceURL: "https://youtube.com/watch?v=rt", SourceURLHash: "hash_rt", CreatedAt: now, UpdatedAt: now})
				return err
			},
			expectInErr: "upsert history item",
		},
		{
			name: "GetItemByID",
			run: func() error {
				_, err := backend.GetItemByID(ctx, "user_1", "his_rt")
				return err
			},
			expectInErr: "read history item",
		},
		{
			name: "SoftDeleteItem",
			run: func() error {
				return backend.SoftDeleteItem(ctx, "user_1", "his_rt", now)
			},
			expectInErr: "soft-delete history item",
		},
		{
			name: "MarkItemSuccess",
			run: func() error {
				return backend.MarkItemSuccess(ctx, "user_1", "his_rt", now)
			},
			expectInErr: "mark history item success",
		},
		{
			name: "CreateAttempt",
			run: func() error {
				return backend.CreateAttempt(ctx, Attempt{ID: "hat_rt", HistoryItemID: "his_rt", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued, CreatedAt: now, UpdatedAt: now})
			},
			expectInErr: "create history attempt",
		},
		{
			name: "UpdateAttempt",
			run: func() error {
				return backend.UpdateAttempt(ctx, Attempt{ID: "hat_rt", HistoryItemID: "his_rt", UserID: "user_1", RequestKind: RequestKindMP3, Status: StatusQueued, UpdatedAt: now})
			},
			expectInErr: "update history attempt",
		},
		{
			name: "GetAttemptByID",
			run: func() error {
				_, err := backend.GetAttemptByID(ctx, "user_1", "hat_rt")
				return err
			},
			expectInErr: "read history attempt",
		},
		{
			name: "GetAttemptByJobID",
			run: func() error {
				_, err := backend.GetAttemptByJobID(ctx, "job_rt")
				return err
			},
			expectInErr: "read history attempt by job_id",
		},
		{
			name: "GetLatestAttemptByItem",
			run: func() error {
				_, err := backend.GetLatestAttemptByItem(ctx, "user_1", "his_rt")
				return err
			},
			expectInErr: "read latest history attempt",
		},
		{
			name: "ListItems",
			run: func() error {
				_, err := backend.ListItems(ctx, "user_1", ListFilter{Limit: 10})
				return err
			},
			expectInErr: "list history items",
		},
		{
			name: "GetStats",
			run: func() error {
				_, err := backend.GetStats(ctx, "user_1")
				return err
			},
			expectInErr: "query history total items",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.run()
			if err == nil {
				t.Fatalf("expected runtime query error")
			}
			if !strings.Contains(err.Error(), testCase.expectInErr) {
				t.Fatalf("expected error to contain %q, got %v", testCase.expectInErr, err)
			}
		})
	}
}

func TestPostgresBackend_GetStatsSecondQueryErrorBranch(t *testing.T) {
	dsn, cleanup := createTempPostgresDatabase(t)
	defer cleanup()

	backend := newPostgresBackend(dsn)
	defer func() { _ = backend.Close() }()

	ctx, cancel := integrationContext(t)
	defer cancel()

	if err := backend.EnsureReady(ctx); err != nil {
		t.Fatalf("unexpected ensure ready error: %v", err)
	}

	if _, err := backend.db.ExecContext(ctx, `DROP TABLE IF EXISTS history_attempts`); err != nil {
		t.Fatalf("failed to drop history_attempts: %v", err)
	}

	_, err := backend.GetStats(ctx, "user_1")
	if err == nil {
		t.Fatalf("expected second-query stats error")
	}
	if !strings.Contains(err.Error(), "query history stats aggregates") {
		t.Fatalf("unexpected stats second-query error: %v", err)
	}
}

func TestScanHelpersBranches(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)

	if _, err := scanItem(scannerFunc(func(dest ...any) error { return errors.New("scan failed") })); err == nil {
		t.Fatalf("expected scanItem to return scan error")
	}

	itemDeletedAt := now.Add(2 * time.Minute)
	item, err := scanItem(scannerFunc(func(dest ...any) error {
		if len(dest) != 13 {
			return fmt.Errorf("unexpected scanItem dest count: %d", len(dest))
		}
		*dest[0].(*string) = "his_scan"
		*dest[1].(*string) = "user_1"
		*dest[2].(*string) = "youtube"
		*dest[3].(*string) = "https://youtube.com/watch?v=scan"
		*dest[4].(*string) = "hash_scan"
		*dest[5].(*stdsql.NullString) = stdsql.NullString{String: "Scan Title", Valid: true}
		*dest[6].(*stdsql.NullString) = stdsql.NullString{String: "https://img.example.com/scan.jpg", Valid: true}
		*dest[7].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(time.Minute), Valid: true}
		*dest[8].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(90 * time.Second), Valid: true}
		*dest[9].(*int) = 3
		*dest[10].(*stdsql.NullTime) = stdsql.NullTime{Time: itemDeletedAt, Valid: true}
		*dest[11].(*time.Time) = now
		*dest[12].(*time.Time) = now.Add(3 * time.Minute)
		return nil
	}))
	if err != nil {
		t.Fatalf("unexpected scanItem success error: %v", err)
	}
	if item.DeletedAt == nil || !item.DeletedAt.Equal(itemDeletedAt) {
		t.Fatalf("expected deleted_at to be set from scanItem, got %+v", item.DeletedAt)
	}

	if _, err := scanListEntry(scannerFunc(func(dest ...any) error { return errors.New("scan list failed") })); err == nil {
		t.Fatalf("expected scanListEntry to return scan error")
	}

	entryExpiresAt := now.Add(10 * time.Minute)
	entryCompletedAt := now.Add(11 * time.Minute)
	entryDeletedAt := now.Add(12 * time.Minute)
	entry, err := scanListEntry(scannerFunc(func(dest ...any) error {
		if len(dest) != 30 {
			return fmt.Errorf("unexpected scanListEntry dest count: %d", len(dest))
		}

		*dest[0].(*string) = "his_entry"
		*dest[1].(*string) = "user_1"
		*dest[2].(*string) = "youtube"
		*dest[3].(*string) = "https://youtube.com/watch?v=entry"
		*dest[4].(*string) = "hash_entry"
		*dest[5].(*stdsql.NullString) = stdsql.NullString{String: "Entry Title", Valid: true}
		*dest[6].(*stdsql.NullString) = stdsql.NullString{String: "https://img.example.com/entry.jpg", Valid: true}
		*dest[7].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(time.Minute), Valid: true}
		*dest[8].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(2 * time.Minute), Valid: true}
		*dest[9].(*int) = 7
		*dest[10].(*stdsql.NullTime) = stdsql.NullTime{Time: entryDeletedAt, Valid: true}
		*dest[11].(*time.Time) = now
		*dest[12].(*time.Time) = now.Add(3 * time.Minute)

		*dest[13].(*stdsql.NullString) = stdsql.NullString{String: "hat_entry", Valid: true}
		*dest[14].(*stdsql.NullString) = stdsql.NullString{String: "his_entry", Valid: true}
		*dest[15].(*stdsql.NullString) = stdsql.NullString{String: "user_1", Valid: true}
		*dest[16].(*stdsql.NullString) = stdsql.NullString{String: "mp4", Valid: true}
		*dest[17].(*stdsql.NullString) = stdsql.NullString{String: "done", Valid: true}
		*dest[18].(*stdsql.NullString) = stdsql.NullString{String: "18", Valid: true}
		*dest[19].(*stdsql.NullString) = stdsql.NullString{String: "1080p", Valid: true}
		*dest[20].(*stdsql.NullInt64) = stdsql.NullInt64{Int64: 12345, Valid: true}
		*dest[21].(*stdsql.NullString) = stdsql.NullString{String: "job_entry", Valid: true}
		*dest[22].(*stdsql.NullString) = stdsql.NullString{String: "out/key", Valid: true}
		*dest[23].(*stdsql.NullString) = stdsql.NullString{String: "https://signed.example.com/entry.mp4", Valid: true}
		*dest[24].(*stdsql.NullTime) = stdsql.NullTime{Time: entryExpiresAt, Valid: true}
		*dest[25].(*stdsql.NullString) = stdsql.NullString{String: "", Valid: false}
		*dest[26].(*stdsql.NullString) = stdsql.NullString{String: "", Valid: false}
		*dest[27].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(4 * time.Minute), Valid: true}
		*dest[28].(*stdsql.NullTime) = stdsql.NullTime{Time: now.Add(5 * time.Minute), Valid: true}
		*dest[29].(*stdsql.NullTime) = stdsql.NullTime{Time: entryCompletedAt, Valid: true}
		return nil
	}))
	if err != nil {
		t.Fatalf("unexpected scanListEntry success error: %v", err)
	}
	if entry.Item.LastSuccessAt == nil || entry.Item.DeletedAt == nil {
		t.Fatalf("expected scanListEntry to set item nullable times, got %+v", entry.Item)
	}
	if entry.LatestAttempt == nil {
		t.Fatalf("expected scanListEntry latest attempt to be set")
	}
	if entry.LatestAttempt.ExpiresAt == nil || !entry.LatestAttempt.ExpiresAt.Equal(entryExpiresAt) {
		t.Fatalf("expected attempt expires_at from scanListEntry, got %+v", entry.LatestAttempt.ExpiresAt)
	}
	if entry.LatestAttempt.CompletedAt == nil || !entry.LatestAttempt.CompletedAt.Equal(entryCompletedAt) {
		t.Fatalf("expected attempt completed_at from scanListEntry, got %+v", entry.LatestAttempt.CompletedAt)
	}
}
