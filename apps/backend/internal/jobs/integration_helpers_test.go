package jobs

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

	"github.com/redis/go-redis/v9"
)

func testContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 20*time.Second)
}

func testRedisAddr() string {
	if v := strings.TrimSpace(os.Getenv("YTD_TEST_REDIS_ADDR")); v != "" {
		return v
	}
	return "127.0.0.1:6382"
}

func testRedisPassword() string {
	return strings.TrimSpace(os.Getenv("YTD_TEST_REDIS_PASSWORD"))
}

func newIntegrationRedisBackend(t *testing.T, ttl time.Duration) *redisBackend {
	t.Helper()
	ctx, cancel := testContext(t)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr(),
		Password: testRedisPassword(),
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("skip redis integration: redis is unavailable (%v)", err)
	}

	keyPrefix := fmt.Sprintf("ytd_it_%d_%d", time.Now().UnixNano(), rand.Intn(100000))
	backend := &redisBackend{
		client:    client,
		keyPrefix: keyPrefix,
		ttl:       ttl,
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := testContext(t)
		defer cleanupCancel()

		keys, err := client.Keys(cleanupCtx, keyPrefix+":*").Result()
		if err == nil && len(keys) > 0 {
			_, _ = client.Del(cleanupCtx, keys...).Result()
		}
		_ = client.Close()
	})

	return backend
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
