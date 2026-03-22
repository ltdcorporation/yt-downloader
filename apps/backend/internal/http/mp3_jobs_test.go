package http

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
)

func TestEnqueueMP3Job_GuardsAndErrors(t *testing.T) {
	if _, err := (*Server)(nil).enqueueMP3Job(context.Background(), enqueueMP3Params{}); err == nil {
		t.Fatalf("expected error for nil server")
	}

	cfg := baseTestConfig()
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, &fakeResolver{}, queue, store)

	if _, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{SourceURL: "   "}); err == nil {
		t.Fatalf("expected error for empty source URL")
	}

	store.putErr = errors.New("put failed")
	if _, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{SourceURL: "https://youtube.com/watch?v=abc"}); err == nil || !strings.Contains(err.Error(), "persist queued job") {
		t.Fatalf("expected persist error, got %v", err)
	}
	store.putErr = nil

	queue.err = errors.New("enqueue failed")
	jobID, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{SourceURL: "https://youtube.com/watch?v=abc", Platform: "youtube", UserID: "user_1"})
	if err == nil || !strings.Contains(err.Error(), "enqueue mp3 job") {
		t.Fatalf("expected enqueue error, got id=%q err=%v", jobID, err)
	}

	if len(store.records) != 1 {
		t.Fatalf("expected one failed record after enqueue error, got %d", len(store.records))
	}
	for _, record := range store.records {
		if record.Status != jobs.StatusFailed {
			t.Fatalf("expected failed record status after enqueue error, got %s", record.Status)
		}
	}
}

func TestEnqueueMP3Job_SuccessAndHistoryAttemptCreation(t *testing.T) {
	cfg := baseTestConfig()
	cfg.MP3Bitrate = 0 // force fallback bitrate branch to 128
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, &fakeResolver{}, queue, store)
	token, userID := registerUserAndGetToken(t, server)

	if strings.TrimSpace(token) == "" {
		t.Fatalf("expected valid auth token from registration")
	}

	jobID, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{
		SourceURL: "https://youtube.com/watch?v=abc123",
		Headers: map[string]string{
			"Origin": "https://example.com",
		},
		UserAgent: "TestAgent/1.0",
		Platform:  "youtube",
		Title:     "Test Video",
		Thumbnail: "https://img.example.com/test.jpg",
		UserID:    userID,
	})
	if err != nil {
		t.Fatalf("unexpected enqueue success error: %v", err)
	}
	if !strings.HasPrefix(jobID, "job_") {
		t.Fatalf("unexpected job id format: %s", jobID)
	}

	record, ok := store.records[jobID]
	if !ok {
		t.Fatalf("expected job record for %s", jobID)
	}
	if record.Status != jobs.StatusQueued {
		t.Fatalf("expected queued record status, got %s", record.Status)
	}
	if !strings.Contains(record.OutputKey, jobID) {
		t.Fatalf("expected output key to include job id, got %s", record.OutputKey)
	}

	if queue.enqueueTask == nil {
		t.Fatalf("expected enqueue task to be captured")
	}
	if gotType := queue.enqueueTask.Type(); gotType != "mp3:convert" {
		t.Fatalf("unexpected task type: %s", gotType)
	}

	attempt, err := server.historyStore.GetAttemptByJobID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("expected history attempt for queued job, got err=%v", err)
	}
	if attempt.UserID != userID {
		t.Fatalf("expected attempt user id %s, got %s", userID, attempt.UserID)
	}
	if attempt.RequestKind != history.RequestKindMP3 {
		t.Fatalf("expected request kind mp3, got %s", attempt.RequestKind)
	}
	if attempt.QualityLabel != "128kbps" {
		t.Fatalf("expected fallback quality label 128kbps, got %s", attempt.QualityLabel)
	}
}

func TestEnqueueMP3Job_SkipsHistoryWhenUserMissingOrPlatformUnsupported(t *testing.T) {
	cfg := baseTestConfig()
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, &fakeResolver{}, queue, store)
	_, userID := registerUserAndGetToken(t, server)

	jobWithoutUser, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{
		SourceURL: "https://example.com/no-user",
		Platform:  "youtube",
		UserID:    "",
	})
	if err != nil {
		t.Fatalf("expected enqueue success without user id, got err=%v", err)
	}
	if _, err := server.historyStore.GetAttemptByJobID(context.Background(), jobWithoutUser); !errors.Is(err, history.ErrAttemptNotFound) {
		t.Fatalf("expected no history attempt when user id missing, got %v", err)
	}

	jobID, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{
		SourceURL: "https://example.com/video",
		Platform:  "unknown",
		UserID:    userID,
	})
	if err != nil {
		t.Fatalf("expected enqueue success with unsupported history platform, got err=%v", err)
	}

	if _, err := server.historyStore.GetAttemptByJobID(context.Background(), jobID); !errors.Is(err, history.ErrAttemptNotFound) {
		t.Fatalf("expected no history attempt for unsupported platform, got %v", err)
	}
}

func TestEnqueueMP3Job_SkipsHistoryOnUnsupportedPlatform(t *testing.T) {
	cfg := baseTestConfig()
	queue := &fakeQueue{}
	store := newFakeJobStore()
	server := newTestServer(t, cfg, &fakeResolver{}, queue, store)
	_, userID := registerUserAndGetToken(t, server)

	jobID, err := server.enqueueMP3Job(context.Background(), enqueueMP3Params{
		SourceURL: "https://example.com/video",
		Platform:  "unknown",
		UserID:    userID,
	})
	if err != nil {
		t.Fatalf("expected enqueue success with unsupported history platform, got err=%v", err)
	}

	if _, err := server.historyStore.GetAttemptByJobID(context.Background(), jobID); !errors.Is(err, history.ErrAttemptNotFound) {
		t.Fatalf("expected no history attempt for unsupported platform, got %v", err)
	}
}
