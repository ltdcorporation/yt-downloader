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
	t.Setenv("IG_MAX_QUALITY", "")
	t.Setenv("IG_COOKIES_DIR", "")
	t.Setenv("IG_COOKIES_FILES", "")
	t.Setenv("IG_RESOLVE_TRY_WITHOUT_COOKIES", "")
	t.Setenv("TT_MAX_QUALITY", "")
	t.Setenv("TT_COOKIES_DIR", "")
	t.Setenv("TT_COOKIES_FILES", "")
	t.Setenv("TT_RESOLVE_TRY_WITHOUT_COOKIES", "")
	t.Setenv("MP3_BITRATE", "")
	t.Setenv("MP3_OUTPUT_TTL_MINUTES", "")
	t.Setenv("JOB_RETENTION_DAYS", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("ADMIN_BASIC_AUTH_USER", "")
	t.Setenv("ADMIN_BASIC_AUTH_PASS", "")
	t.Setenv("AUTH_SESSION_COOKIE_NAME", "")
	t.Setenv("AUTH_SESSION_COOKIE_DOMAIN", "")
	t.Setenv("AUTH_SESSION_COOKIE_SECURE", "")
	t.Setenv("AUTH_SESSION_TTL_HOURS", "")
	t.Setenv("AUTH_REMEMBER_SESSION_TTL_HOURS", "")
	t.Setenv("AUTH_BCRYPT_COST", "")
	t.Setenv("GOOGLE_CLIENT_IDS", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
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
	if cfg.IGMaxQuality != 1080 {
		t.Fatalf("unexpected default IGMaxQuality: %d", cfg.IGMaxQuality)
	}
	if cfg.IGResolveTryWithoutCookies != true {
		t.Fatalf("unexpected default IGResolveTryWithoutCookies: %v", cfg.IGResolveTryWithoutCookies)
	}
	if cfg.TTMaxQuality != 1080 {
		t.Fatalf("unexpected default TTMaxQuality: %d", cfg.TTMaxQuality)
	}
	if cfg.TTResolveTryWithoutCookies != true {
		t.Fatalf("unexpected default TTResolveTryWithoutCookies: %v", cfg.TTResolveTryWithoutCookies)
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("unexpected default RedisAddr: %s", cfg.RedisAddr)
	}
	if cfg.AdminBasicAuthUser != "admin" || cfg.AdminBasicAuthPass != "change-me" {
		t.Fatalf("unexpected default admin credentials")
	}
	if cfg.AuthSessionCookieName != "qs_session" {
		t.Fatalf("unexpected default AUTH_SESSION_COOKIE_NAME: %s", cfg.AuthSessionCookieName)
	}
	if cfg.AuthSessionCookieDomain != "" {
		t.Fatalf("unexpected default AUTH_SESSION_COOKIE_DOMAIN: %s", cfg.AuthSessionCookieDomain)
	}
	if cfg.AuthSessionCookieSecure {
		t.Fatalf("expected AUTH_SESSION_COOKIE_SECURE=false in non-production defaults")
	}
	if cfg.AuthSessionTTLHours != 24 || cfg.AuthRememberSessionTTLHours != 720 {
		t.Fatalf("unexpected auth TTL defaults: ttl=%d remember=%d", cfg.AuthSessionTTLHours, cfg.AuthRememberSessionTTLHours)
	}
	if cfg.AuthBcryptCost != 12 {
		t.Fatalf("unexpected AUTH_BCRYPT_COST default: %d", cfg.AuthBcryptCost)
	}
	if cfg.GoogleClientIDs != "" {
		t.Fatalf("unexpected GOOGLE_CLIENT_IDS default: %q", cfg.GoogleClientIDs)
	}
	if cfg.CORSAllowedOrigins != "http://127.0.0.1:3000,http://localhost:3000" {
		t.Fatalf("unexpected default CORS origins: %s", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_UsesSingleGoogleClientIDFallback(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_IDS", "")
	t.Setenv("GOOGLE_CLIENT_ID", "single-client.apps.googleusercontent.com")

	cfg := Load()
	if cfg.GoogleClientIDs != "single-client.apps.googleusercontent.com" {
		t.Fatalf("expected GOOGLE_CLIENT_ID fallback, got %q", cfg.GoogleClientIDs)
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
	t.Setenv("IG_MAX_QUALITY", "1440")
	t.Setenv("IG_COOKIES_DIR", "/tmp/ig-cookies")
	t.Setenv("IG_COOKIES_FILES", "/tmp/ig-a.txt,/tmp/ig-b.txt")
	t.Setenv("IG_RESOLVE_TRY_WITHOUT_COOKIES", "false")
	t.Setenv("TT_MAX_QUALITY", "540")
	t.Setenv("TT_COOKIES_DIR", "/tmp/tt-cookies")
	t.Setenv("TT_COOKIES_FILES", "/tmp/tt-a.txt,/tmp/tt-b.txt")
	t.Setenv("TT_RESOLVE_TRY_WITHOUT_COOKIES", "false")
	t.Setenv("MP3_BITRATE", "256")
	t.Setenv("MP3_OUTPUT_TTL_MINUTES", "120")
	t.Setenv("JOB_RETENTION_DAYS", "30")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6382")
	t.Setenv("POSTGRES_DSN", "postgres://postgres:pass@127.0.0.1:5435/ytd?sslmode=disable")
	t.Setenv("AUTH_SESSION_COOKIE_NAME", "quicksnap_session")
	t.Setenv("AUTH_SESSION_COOKIE_DOMAIN", ".example.com")
	t.Setenv("AUTH_SESSION_COOKIE_SECURE", "not-bool")
	t.Setenv("AUTH_SESSION_TTL_HOURS", "12")
	t.Setenv("AUTH_REMEMBER_SESSION_TTL_HOURS", "480")
	t.Setenv("AUTH_BCRYPT_COST", "14")
	t.Setenv("GOOGLE_CLIENT_IDS", "web-client-id.apps.googleusercontent.com,mobile-client-id.apps.googleusercontent.com")
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
	if cfg.IGMaxQuality != 1440 {
		t.Fatalf("unexpected IG_MAX_QUALITY: %d", cfg.IGMaxQuality)
	}
	if cfg.IGCookiesDir != "/tmp/ig-cookies" {
		t.Fatalf("unexpected IG_COOKIES_DIR: %s", cfg.IGCookiesDir)
	}
	if cfg.IGCookiesFiles != "/tmp/ig-a.txt,/tmp/ig-b.txt" {
		t.Fatalf("unexpected IG_COOKIES_FILES: %s", cfg.IGCookiesFiles)
	}
	if cfg.IGResolveTryWithoutCookies != false {
		t.Fatalf("unexpected IG_RESOLVE_TRY_WITHOUT_COOKIES: %v", cfg.IGResolveTryWithoutCookies)
	}
	if cfg.TTMaxQuality != 540 {
		t.Fatalf("unexpected TT_MAX_QUALITY: %d", cfg.TTMaxQuality)
	}
	if cfg.TTCookiesDir != "/tmp/tt-cookies" {
		t.Fatalf("unexpected TT_COOKIES_DIR: %s", cfg.TTCookiesDir)
	}
	if cfg.TTCookiesFiles != "/tmp/tt-a.txt,/tmp/tt-b.txt" {
		t.Fatalf("unexpected TT_COOKIES_FILES: %s", cfg.TTCookiesFiles)
	}
	if cfg.TTResolveTryWithoutCookies != false {
		t.Fatalf("unexpected TT_RESOLVE_TRY_WITHOUT_COOKIES: %v", cfg.TTResolveTryWithoutCookies)
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
	if cfg.AuthSessionCookieName != "quicksnap_session" {
		t.Fatalf("unexpected AUTH_SESSION_COOKIE_NAME override: %s", cfg.AuthSessionCookieName)
	}
	if cfg.AuthSessionCookieDomain != ".example.com" {
		t.Fatalf("unexpected AUTH_SESSION_COOKIE_DOMAIN override: %s", cfg.AuthSessionCookieDomain)
	}
	if !cfg.AuthSessionCookieSecure {
		t.Fatalf("invalid AUTH_SESSION_COOKIE_SECURE should fallback to true for production")
	}
	if cfg.AuthSessionTTLHours != 12 || cfg.AuthRememberSessionTTLHours != 480 {
		t.Fatalf("unexpected auth ttl overrides: ttl=%d remember=%d", cfg.AuthSessionTTLHours, cfg.AuthRememberSessionTTLHours)
	}
	if cfg.AuthBcryptCost != 14 {
		t.Fatalf("unexpected AUTH_BCRYPT_COST override: %d", cfg.AuthBcryptCost)
	}
	if cfg.GoogleClientIDs != "web-client-id.apps.googleusercontent.com,mobile-client-id.apps.googleusercontent.com" {
		t.Fatalf("unexpected GOOGLE_CLIENT_IDS override: %q", cfg.GoogleClientIDs)
	}
	if cfg.R2KeyPrefix != "yt-downloader/prod" {
		t.Fatalf("unexpected R2_KEY_PREFIX override: %s", cfg.R2KeyPrefix)
	}
}
