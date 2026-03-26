package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
)

func makeFakeYTDLPVideoScript(t *testing.T, mode string, expectedFormatID string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp-video")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
mode=%q
expected_format=%q

out_tpl=""
format_id=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      out_tpl="$2"
      shift 2
      ;;
    --format)
      format_id="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n "$expected_format" && "$format_id" != "$expected_format" ]]; then
  echo "unexpected format id: $format_id" >&2
  exit 71
fi

case "$mode" in
  success)
    out_file="$(printf '%%s' "$out_tpl" | sed 's/%%(ext)s/mp4/g')"
    mkdir -p "$(dirname "$out_file")"
    printf 'video-data' > "$out_file"
    ;;
  fail)
    echo "simulated yt-dlp video failure" >&2
    exit 66
    ;;
  nooutput)
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 65
    ;;
esac
`, mode, expectedFormatID)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake yt-dlp video script: %v", err)
	}
	return path
}

func makeFakeFFmpegScript(t *testing.T, mode string, expectStart string, expectEnd string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ffmpeg")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
mode=%q
expect_start=%q
expect_end=%q

ss=""
to=""
input=""
out=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -ss)
      ss="$2"
      shift 2
      ;;
    -to)
      to="$2"
      shift 2
      ;;
    -i)
      input="$2"
      shift 2
      ;;
    *)
      out="$1"
      shift
      ;;
  esac
done

if [[ -n "$expect_start" && "$ss" != "$expect_start" ]]; then
  echo "unexpected -ss: $ss" >&2
  exit 81
fi
if [[ -n "$expect_end" && "$to" != "$expect_end" ]]; then
  echo "unexpected -to: $to" >&2
  exit 82
fi
if [[ ! -f "$input" ]]; then
  echo "input file missing" >&2
  exit 83
fi

case "$mode" in
  success)
    mkdir -p "$(dirname "$out")"
    printf 'cut-video' > "$out"
    ;;
  fail)
    echo "simulated ffmpeg failure" >&2
    exit 84
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 85
    ;;
esac
`, mode, expectStart, expectEnd)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake ffmpeg script: %v", err)
	}
	return path
}

func makeVideoCutTask(t *testing.T, payload VideoCutPayload) *asynq.Task {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to encode payload: %v", err)
	}
	return asynq.NewTask(TaskVideoCut, encoded)
}

func TestHandleVideoCut_InvalidPayload(t *testing.T) {
	worker := makeWorkerForTest("", nil, nil)

	t.Run("invalid json", func(t *testing.T) {
		task := asynq.NewTask(TaskVideoCut, []byte("{"))
		err := worker.handleVideoCut(context.Background(), task)
		if err == nil {
			t.Fatal("expected invalid json error")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		task := makeVideoCutTask(t, VideoCutPayload{JobID: "job_x"})
		err := worker.handleVideoCut(context.Background(), task)
		if err == nil || !strings.Contains(err.Error(), "invalid payload") {
			t.Fatalf("expected invalid payload error, got %v", err)
		}
	})
}

func TestHandleVideoCut_SuccessUpdatesJobAndHistory(t *testing.T) {
	ytdlpScript := makeFakeYTDLPVideoScript(t, "success", "22")
	ffmpegScript := makeFakeFFmpegScript(t, "success", "12", "48")

	store := newFakeJobStore()
	historyStore := newFakeHistoryStore()
	jobID := "job_video_cut_success"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	historyAttempt := history.Attempt{
		ID:            "hat_video_cut_success",
		HistoryItemID: "his_video_cut_success",
		UserID:        "user_1",
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusQueued,
		JobID:         jobID,
	}
	historyStore.attemptByID[historyAttempt.ID] = historyAttempt
	historyStore.attemptByJobID[jobID] = historyAttempt

	expiresAt := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	r2 := &fakeR2{presignURL: "https://signed.example.com/video-cut.mp4", presignExpiresAt: expiresAt}

	worker := makeWorkerForTestWithHistory(ytdlpScript, store, r2, historyStore)
	worker.cfg.VideoCutFFmpegBinary = ffmpegScript
	worker.cfg.VideoCutOutputTTLMinutes = 45

	task := makeVideoCutTask(t, VideoCutPayload{
		JobID:          jobID,
		SourceURL:      "https://www.youtube.com/watch?v=abc123",
		FormatID:       "22",
		OutputKey:      "yt-downloader/prod/video-cut/" + jobID + ".mp4",
		CutMode:        VideoCutModeManual,
		ManualStartSec: 12,
		ManualEndSec:   48,
	})

	if err := worker.handleVideoCut(context.Background(), task); err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusDone {
		t.Fatalf("expected done status, got %s", record.Status)
	}
	if record.DownloadURL != r2.presignURL {
		t.Fatalf("unexpected download url, got %q want %q", record.DownloadURL, r2.presignURL)
	}
	if record.ExpiresAt == nil || !record.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expires at, got=%+v want=%v", record.ExpiresAt, expiresAt)
	}
	if r2.presignTTL != 45*time.Minute {
		t.Fatalf("unexpected presign ttl, got %v", r2.presignTTL)
	}
	if !r2.uploadedFileFound {
		t.Fatalf("expected uploaded file to exist")
	}

	attempt := historyStore.attemptByID[historyAttempt.ID]
	if attempt.Status != history.StatusDone {
		t.Fatalf("expected history status done, got %s", attempt.Status)
	}
	if attempt.DownloadURL != r2.presignURL {
		t.Fatalf("unexpected history download url, got %q", attempt.DownloadURL)
	}
	if attempt.SizeBytes == nil || *attempt.SizeBytes <= 0 {
		t.Fatalf("expected history size bytes to be set, got %+v", attempt.SizeBytes)
	}
	if attempt.ErrorCode != "" || attempt.ErrorText != "" {
		t.Fatalf("expected history errors cleared, got code=%q text=%q", attempt.ErrorCode, attempt.ErrorText)
	}
}

func TestHandleVideoCut_FFmpegFailureMarksFailed(t *testing.T) {
	ytdlpScript := makeFakeYTDLPVideoScript(t, "success", "22")
	ffmpegScript := makeFakeFFmpegScript(t, "fail", "", "")

	store := newFakeJobStore()
	historyStore := newFakeHistoryStore()
	jobID := "job_video_cut_fail"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	historyAttempt := history.Attempt{
		ID:            "hat_video_cut_fail",
		HistoryItemID: "his_video_cut_fail",
		UserID:        "user_1",
		RequestKind:   history.RequestKindMP4,
		Status:        history.StatusQueued,
		JobID:         jobID,
	}
	historyStore.attemptByID[historyAttempt.ID] = historyAttempt
	historyStore.attemptByJobID[jobID] = historyAttempt

	worker := makeWorkerForTestWithHistory(ytdlpScript, store, &fakeR2{}, historyStore)
	worker.cfg.VideoCutFFmpegBinary = ffmpegScript

	task := makeVideoCutTask(t, VideoCutPayload{
		JobID:          jobID,
		SourceURL:      "https://www.youtube.com/watch?v=abc123",
		FormatID:       "22",
		OutputKey:      "yt-downloader/prod/video-cut/" + jobID + ".mp4",
		CutMode:        VideoCutModeManual,
		ManualStartSec: 12,
		ManualEndSec:   48,
	})

	err := worker.handleVideoCut(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "simulated ffmpeg failure") {
		t.Fatalf("expected ffmpeg failure error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if !strings.Contains(record.Error, "ffmpeg cut failed") {
		t.Fatalf("expected ffmpeg failure in record error, got %q", record.Error)
	}

	attempt := historyStore.attemptByID[historyAttempt.ID]
	if attempt.Status != history.StatusFailed {
		t.Fatalf("expected history status failed, got %s", attempt.Status)
	}
	if attempt.ErrorCode != "video_cut_failed" {
		t.Fatalf("expected history error code video_cut_failed, got %s", attempt.ErrorCode)
	}
}
