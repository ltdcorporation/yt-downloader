package settings

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func testContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 20*time.Second)
}

func testPostgresAdminDSN() string {
	if v := strings.TrimSpace(os.Getenv("YTD_TEST_POSTGRES_ADMIN_DSN")); v != "" {
		return v
	}
	return "postgres://postgres:123987@127.0.0.1:5435/postgres?sslmode=disable"
}

func createTempPostgresDatabase(t *testing.T) (string, func()) {
	t.Helper()

	adminDSN := testPostgresAdminDSN()
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Skipf("skip postgres integration: cannot open admin dsn (%v)", err)
	}

	ctx, cancel := testContext(t)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Skipf("skip postgres integration: cannot ping admin dsn (%v)", err)
	}

	dbName := fmt.Sprintf("ytd_it_%d_%d", time.Now().UnixNano(), rand.Intn(100000))
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		_ = adminDB.Close()
		t.Skipf("skip postgres integration: cannot create temp db (%v)", err)
	}

	parsed, err := url.Parse(adminDSN)
	if err != nil {
		_ = adminDB.Close()
		t.Fatalf("invalid admin DSN URL: %v", err)
	}
	parsed.Path = "/" + dbName
	testDSN := parsed.String()

	cleanup := func() {
		cleanupCtx, cleanupCancel := testContext(t)
		defer cleanupCancel()

		_, _ = adminDB.ExecContext(
			cleanupCtx,
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`,
			dbName,
		)
		_, _ = adminDB.ExecContext(cleanupCtx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
		_ = adminDB.Close()
	}

	return testDSN, cleanup
}

func ensureAuthUsersTable(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := testContext(t)
	defer cancel()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth_users (
			id TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		t.Fatalf("failed to create auth_users table: %v", err)
	}
}

func seedAuthUser(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	ctx, cancel := testContext(t)
	defer cancel()

	_, err := db.ExecContext(ctx, `INSERT INTO auth_users (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, strings.TrimSpace(userID))
	if err != nil {
		t.Fatalf("failed to seed auth user %q: %v", userID, err)
	}
}
