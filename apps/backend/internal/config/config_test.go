package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("YTDLP_BINARY", "")
	t.Setenv("YTDLP_JS_RUNTIMES", "")
	t.Setenv("RATE_LIMIT_RPS", "")
	t.Setenv("MAX_VIDEO_DURATION_MINUTES", "")
	t.Setenv("MAX_FILE_SIZE_BYTES", "")
	t.Setenv("YOUTUBE_MAX_QUALITY", "")
	t.Setenv("X_MAX_QUALITY", "")
	t.Setenv("X_COOKIES_DIR", "")
	t.Setenv("X_COOKIES_FILES", "")
	t.Setenv("X_RESOLVE_TRY_WITHOUT_COOKIES", "")
	t.Setenv("MP3_BITRATE", "")
	t.Setenv("MP3_OUTPUT_TTL_MINUTES", "")
	t.Setenv("JOB_RETENTION_DAYS", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "")
	t.Setenv("R2_ENDPOINT", "")
	t.Setenv("R2_REGION", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_KEY_PREFIX", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg := Load()

	if cfg.AppEnv != "development" {
		t.Fatalf("unexpected default AppEnv: %s", cfg.AppEnv)
	}
	if cfg.HTTPPort != "8080" {
		t.Fatalf("unexpected default HTTPPort: %s", cfg.HTTPPort)
	}
	if cfg.YTDLPBinary != "yt-dlp" {
		t.Fatalf("unexpected default YTDLPBinary: %s", cfg.YTDLPBinary)
	}
	if cfg.RateLimitRPS != 3 {
		t.Fatalf("unexpected default RateLimitRPS: %v", cfg.RateLimitRPS)
	}
	if cfg.XMaxQuality != 1080 {
		t.Fatalf("unexpected default XMaxQuality: %d", cfg.XMaxQuality)
	}
	if cfg.XResolveTryWithoutCookies != true {
		t.Fatalf("unexpected default XResolveTryWithoutCookies: %v", cfg.XResolveTryWithoutCookies)
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("unexpected default RedisAddr: %s", cfg.RedisAddr)
	}
	if cfg.AdminBasicAuthUser != "admin" || cfg.AdminBasicAuthPass != "change-me" {
		t.Fatalf("unexpected default admin credentials")
	}
	if cfg.CORSAllowedOrigins != "http://127.0.0.1:3000,http://localhost:3000" {
		t.Fatalf("unexpected default CORS origins: %s", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_OverridesAndInvalidFallback(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_PORT", "18080")
	t.Setenv("HTTP_ADDR", "127.0.0.1:18080")
	t.Setenv("YTDLP_BINARY", "/usr/local/bin/yt-dlp")
	t.Setenv("RATE_LIMIT_RPS", "not-a-float")
	t.Setenv("MAX_VIDEO_DURATION_MINUTES", "90")
	t.Setenv("MAX_FILE_SIZE_BYTES", "not-int64")
	t.Setenv("YOUTUBE_MAX_QUALITY", "2160")
	t.Setenv("X_MAX_QUALITY", "720")
	t.Setenv("X_COOKIES_DIR", "/tmp/x-cookies")
	t.Setenv("X_COOKIES_FILES", "/tmp/a.txt,/tmp/b.txt")
	t.Setenv("X_RESOLVE_TRY_WITHOUT_COOKIES", "false")
	t.Setenv("MP3_BITRATE", "256")
	t.Setenv("MP3_OUTPUT_TTL_MINUTES", "120")
	t.Setenv("JOB_RETENTION_DAYS", "30")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6382")
	t.Setenv("POSTGRES_DSN", "postgres://postgres:pass@127.0.0.1:5435/ytd?sslmode=disable")
	t.Setenv("R2_KEY_PREFIX", "yt-downloader/prod")

	cfg := Load()

	if cfg.AppEnv != "production" {
		t.Fatalf("expected override AppEnv, got %s", cfg.AppEnv)
	}
	if cfg.HTTPPort != "18080" || cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("unexpected http bind config: port=%s addr=%s", cfg.HTTPPort, cfg.HTTPAddr)
	}
	if cfg.YTDLPBinary != "/usr/local/bin/yt-dlp" {
		t.Fatalf("expected YTDLP_BINARY override, got %s", cfg.YTDLPBinary)
	}
	if cfg.RateLimitRPS != 3 {
		t.Fatalf("invalid RATE_LIMIT_RPS should fallback to 3, got %v", cfg.RateLimitRPS)
	}
	if cfg.MaxVideoDurationMinutes != 90 {
		t.Fatalf("unexpected MAX_VIDEO_DURATION_MINUTES: %d", cfg.MaxVideoDurationMinutes)
	}
	if cfg.MaxFileSizeBytes != 1073741824 {
		t.Fatalf("invalid MAX_FILE_SIZE_BYTES should fallback, got %d", cfg.MaxFileSizeBytes)
	}
	if cfg.YouTubeMaxQuality != 2160 {
		t.Fatalf("unexpected YOUTUBE_MAX_QUALITY: %d", cfg.YouTubeMaxQuality)
	}
	if cfg.XMaxQuality != 720 {
		t.Fatalf("unexpected X_MAX_QUALITY: %d", cfg.XMaxQuality)
	}
	if cfg.XCookiesDir != "/tmp/x-cookies" {
		t.Fatalf("unexpected X_COOKIES_DIR: %s", cfg.XCookiesDir)
	}
	if cfg.XCookiesFiles != "/tmp/a.txt,/tmp/b.txt" {
		t.Fatalf("unexpected X_COOKIES_FILES: %s", cfg.XCookiesFiles)
	}
	if cfg.XResolveTryWithoutCookies != false {
		t.Fatalf("unexpected X_RESOLVE_TRY_WITHOUT_COOKIES: %v", cfg.XResolveTryWithoutCookies)
	}
	if cfg.MP3Bitrate != 256 || cfg.MP3OutputTTLMinutes != 120 || cfg.JobRetentionDays != 30 {
		t.Fatalf("unexpected MP3/job retention overrides: bitrate=%d ttl=%d retention=%d", cfg.MP3Bitrate, cfg.MP3OutputTTLMinutes, cfg.JobRetentionDays)
	}
	if cfg.RedisAddr != "127.0.0.1:6382" {
		t.Fatalf("unexpected REDIS_ADDR override: %s", cfg.RedisAddr)
	}
	if cfg.PostgresDSN == "" {
		t.Fatalf("expected POSTGRES_DSN override")
	}
	if cfg.R2KeyPrefix != "yt-downloader/prod" {
		t.Fatalf("unexpected R2_KEY_PREFIX override: %s", cfg.R2KeyPrefix)
	}
}
