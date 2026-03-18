package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
)

func TestNewR2Client_RequiresCompleteConfig(t *testing.T) {
	_, err := NewR2Client(context.Background(), config.Config{})
	if err == nil {
		t.Fatal("expected error for incomplete R2 configuration")
	}
	if !strings.Contains(err.Error(), "r2 configuration is incomplete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewR2Client_InvalidEndpoint(t *testing.T) {
	cfg := config.Config{
		R2Endpoint:        "://bad-url",
		R2Bucket:          "bucket",
		R2AccessKeyID:     "key",
		R2SecretAccessKey: "secret",
	}

	_, err := NewR2Client(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected invalid endpoint error")
	}
	if !strings.Contains(err.Error(), "invalid r2 endpoint") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewR2Client_SuccessAndPresign(t *testing.T) {
	cfg := config.Config{
		R2Endpoint:        "https://example.cloudflare.r2.cloudflarestorage.com",
		R2Region:          "",
		R2Bucket:          "bucket-a",
		R2AccessKeyID:     "key",
		R2SecretAccessKey: "secret",
	}

	client, err := NewR2Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected client creation success, got err: %v", err)
	}
	if client == nil || client.client == nil {
		t.Fatal("expected non-nil R2 client")
	}
	if client.bucket != "bucket-a" {
		t.Fatalf("unexpected bucket: %s", client.bucket)
	}

	urlValue, expiresAt, err := client.PresignDownloadURL(context.Background(), "mp3/job_test.mp3", 0)
	if err != nil {
		t.Fatalf("expected presign success, got err: %v", err)
	}
	if !strings.Contains(urlValue, "X-Amz-Signature=") {
		t.Fatalf("expected signed URL, got: %s", urlValue)
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatalf("expected future expiration time, got: %v", expiresAt)
	}
}
