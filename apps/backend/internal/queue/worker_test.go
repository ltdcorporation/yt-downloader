package queue

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"

	"yt-downloader/backend/internal/config"
)

func TestClipError(t *testing.T) {
	if got := clipError(nil); got != "" {
		t.Fatalf("expected empty string for nil error, got %q", got)
	}

	shortErr := errors.New("short error")
	if got := clipError(shortErr); got != shortErr.Error() {
		t.Fatalf("expected same short error text, got %q", got)
	}

	longText := strings.Repeat("x", 450)
	got := clipError(errors.New(longText))
	if len(got) != 400 {
		t.Fatalf("expected clipped length 400, got %d", len(got))
	}
}

func TestConvertMP3_RequiresYTDLPBinary(t *testing.T) {
	worker := NewWorker(config.Config{}, log.New(io.Discard, "", 0), nil, nil)

	_, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_test",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_test.mp3",
		BitrateKbps: 128,
	})
	if err == nil {
		t.Fatal("expected error when YTDLP binary is missing")
	}
	if !strings.Contains(err.Error(), "yt-dlp binary is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("expected nil cleanup when convert does not start")
	}
}

func TestFailJob_NoStoreReturnsOriginalError(t *testing.T) {
	worker := NewWorker(config.Config{}, log.New(io.Discard, "", 0), nil, nil)
	errIn := errors.New("boom")
	errOut := worker.failJob(context.Background(), "job_test", errIn)
	if errOut == nil || errOut.Error() != "boom" {
		t.Fatalf("expected original error, got %v", errOut)
	}
}
