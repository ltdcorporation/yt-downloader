package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv          string
	HTTPPort        string
	HTTPAddr        string
	YTDLPBinary     string
	YTDLPJSRuntimes string

	RateLimitRPS            float64
	MaxVideoDurationMinutes int
	MaxFileSizeBytes        int64
	YouTubeMaxQuality       int

	MP3Bitrate          int
	MP3OutputTTLMinutes int
	JobRetentionDays    int

	RedisAddr     string
	RedisPassword string
	PostgresDSN   string

	AdminBasicAuthUser string
	AdminBasicAuthPass string

	R2Endpoint        string
	R2Region          string
	R2Bucket          string
	R2KeyPrefix       string
	R2AccessKeyID     string
	R2SecretAccessKey string

	CORSAllowedOrigins string
}

func Load() Config {
	return Config{
		AppEnv:                  getenv("APP_ENV", "development"),
		HTTPPort:                getenv("HTTP_PORT", "8080"),
		HTTPAddr:                getenv("HTTP_ADDR", ""),
		YTDLPBinary:             getenv("YTDLP_BINARY", "yt-dlp"),
		YTDLPJSRuntimes:         getenv("YTDLP_JS_RUNTIMES", "node"),
		RateLimitRPS:            getenvFloat("RATE_LIMIT_RPS", 3),
		MaxVideoDurationMinutes: getenvInt("MAX_VIDEO_DURATION_MINUTES", 60),
		MaxFileSizeBytes:        getenvInt64("MAX_FILE_SIZE_BYTES", 1073741824),
		YouTubeMaxQuality:       getenvInt("YOUTUBE_MAX_QUALITY", 1080),
		MP3Bitrate:              getenvInt("MP3_BITRATE", 128),
		MP3OutputTTLMinutes:     getenvInt("MP3_OUTPUT_TTL_MINUTES", 60),
		JobRetentionDays:        getenvInt("JOB_RETENTION_DAYS", 14),
		RedisAddr:               getenv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:           getenv("REDIS_PASSWORD", ""),
		PostgresDSN:             getenv("POSTGRES_DSN", ""),
		AdminBasicAuthUser:      getenv("ADMIN_BASIC_AUTH_USER", "admin"),
		AdminBasicAuthPass:      getenv("ADMIN_BASIC_AUTH_PASS", "change-me"),
		R2Endpoint:              getenv("R2_ENDPOINT", ""),
		R2Region:                getenv("R2_REGION", "auto"),
		R2Bucket:                getenv("R2_BUCKET", ""),
		R2KeyPrefix:             getenv("R2_KEY_PREFIX", ""),
		R2AccessKeyID:           getenv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey:       getenv("R2_SECRET_ACCESS_KEY", ""),
		CORSAllowedOrigins:      getenv("CORS_ALLOWED_ORIGINS", "http://127.0.0.1:3000,http://localhost:3000"),
	}
}

func getenv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func getenvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvInt64(key string, fallback int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
