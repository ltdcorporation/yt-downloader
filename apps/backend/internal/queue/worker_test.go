package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/jobs"
)

type fakeJobStore struct {
	records          map[string]jobs.Record
	updateErr        error
	failOnUpdateCall int
	updateCalls      int
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{records: make(map[string]jobs.Record)}
}

func (f *fakeJobStore) Update(_ context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error) {
	f.updateCalls++
	if f.updateErr != nil && (f.failOnUpdateCall == 0 || f.updateCalls == f.failOnUpdateCall) {
		return jobs.Record{}, f.updateErr
	}

	record, ok := f.records[jobID]
	if !ok {
		record = jobs.Record{ID: jobID}
	}
	if mutate != nil {
		mutate(&record)
	}
	record.UpdatedAt = time.Now().UTC()
	f.records[jobID] = record

	return record, nil
}

type fakeR2 struct {
	uploadErr         error
	presignErr        error
	presignURL        string
	presignExpiresAt  time.Time
	uploadCalls       int
	presignCalls      int
	uploadedKey       string
	uploadedPath      string
	uploadedFileFound bool
	presignTTL        time.Duration
}

func (f *fakeR2) UploadFile(_ context.Context, key string, localPath string) error {
	f.uploadCalls++
	f.uploadedKey = key
	f.uploadedPath = localPath
	if _, err := os.Stat(localPath); err == nil {
		f.uploadedFileFound = true
	}
	if f.uploadErr != nil {
		return f.uploadErr
	}
	return nil
}

func (f *fakeR2) PresignDownloadURL(_ context.Context, _ string, expiresIn time.Duration) (string, time.Time, error) {
	f.presignCalls++
	f.presignTTL = expiresIn
	if f.presignErr != nil {
		return "", time.Time{}, f.presignErr
	}

	urlValue := f.presignURL
	if urlValue == "" {
		urlValue = "https://example.com/download.mp3"
	}
	expiresAt := f.presignExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(expiresIn)
	}
	return urlValue, expiresAt, nil
}

type fakeAsynqServer struct {
	runErr        error
	runCalls      int
	runHandlerNil bool
}

func (f *fakeAsynqServer) Run(handler asynq.Handler) error {
	f.runCalls++
	f.runHandlerNil = handler == nil
	return f.runErr
}

func makeFakeYTDLPScript(t *testing.T, mode string, expectedQuality string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
mode=%q
expected_quality=%q

out_tpl=""
audio_quality=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      out_tpl="$2"
      shift 2
      ;;
    --audio-quality)
      audio_quality="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n "$expected_quality" && "$audio_quality" != "$expected_quality" ]]; then
  echo "unexpected audio quality: $audio_quality" >&2
  exit 77
fi

case "$mode" in
  success)
    out_file="$(printf '%%s' "$out_tpl" | sed 's/%%(ext)s/mp3/g')"
    mkdir -p "$(dirname "$out_file")"
    printf 'audio-data' > "$out_file"
    ;;
  altname)
    out_file="$(printf '%%s' "$out_tpl" | sed 's/%%(ext)s/custom/g')"
    mkdir -p "$(dirname "$out_file")"
    printf 'audio-data' > "$(dirname "$out_file")/alt-output.mp3"
    ;;
  nooutput)
    ;;
  fail)
    echo "simulated yt-dlp failure" >&2
    exit 66
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 65
    ;;
esac
`, mode, expectedQuality)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake yt-dlp script: %v", err)
	}
	return path
}

func makeFakeYTDLPScriptAssertOptions(t *testing.T, expectedJSRuntime string, expectedHeader string, expectedUserAgent string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
expected_js=%q
expected_header=%q
expected_ua=%q

out_tpl=""
seen_js=""
seen_header=""
seen_ua=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      out_tpl="$2"
      shift 2
      ;;
    --js-runtimes)
      seen_js="$2"
      shift 2
      ;;
    --add-header)
      seen_header="$2"
      shift 2
      ;;
    --user-agent)
      seen_ua="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ "$seen_js" != "$expected_js" ]]; then
  echo "unexpected js runtime: $seen_js" >&2
  exit 71
fi
if [[ "$seen_header" != "$expected_header" ]]; then
  echo "unexpected header: $seen_header" >&2
  exit 72
fi
if [[ "$seen_ua" != "$expected_ua" ]]; then
  echo "unexpected user-agent: $seen_ua" >&2
  exit 73
fi

out_file="$(printf '%%s' "$out_tpl" | sed 's/%%(ext)s/mp3/g')"
mkdir -p "$(dirname "$out_file")"
printf 'audio-data' > "$out_file"
`, expectedJSRuntime, expectedHeader, expectedUserAgent)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write option-assert yt-dlp script: %v", err)
	}

	return path
}

func makeWorkerForTest(ytdlpBinary string, store jobStoreUpdater, r2 r2Storage) *Worker {
	cfg := config.Config{
		YTDLPBinary:         ytdlpBinary,
		YTDLPJSRuntimes:     "",
		MP3OutputTTLMinutes: 60,
	}
	return NewWorker(cfg, log.New(io.Discard, "", 0), store, r2)
}

func makeTask(t *testing.T, payload ConvertMP3Payload) *asynq.Task {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to encode payload: %v", err)
	}
	return asynq.NewTask(TaskConvertMP3, encoded)
}

func TestNewWorker_NilLoggerUsesDefault(t *testing.T) {
	worker := NewWorker(config.Config{}, nil, nil, nil)
	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
	if worker.logger == nil {
		t.Fatal("expected logger to be initialized")
	}
	if worker.serverFactory == nil {
		t.Fatal("expected serverFactory to be initialized")
	}
	if worker.mkTempDir == nil {
		t.Fatal("expected mkTempDir to be initialized")
	}
}

func TestRun_UsesFactoryAndReturnsServerError(t *testing.T) {
	store := newFakeJobStore()
	worker := makeWorkerForTest("", store, nil)
	worker.cfg.RedisAddr = "127.0.0.1:6382"

	server := &fakeAsynqServer{runErr: errors.New("server run failed")}
	factoryCalls := 0
	worker.serverFactory = func(_ asynq.RedisClientOpt, _ asynq.Config) asynqServerRunner {
		factoryCalls++
		return server
	}

	err := worker.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "server run failed") {
		t.Fatalf("expected server run error, got %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("expected one factory call, got %d", factoryCalls)
	}
	if server.runCalls != 1 {
		t.Fatalf("expected one server run call, got %d", server.runCalls)
	}
	if server.runHandlerNil {
		t.Fatalf("expected non-nil handler passed to server")
	}
}

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

func TestHandleConvertMP3_InvalidPayloadCases(t *testing.T) {
	worker := makeWorkerForTest("", nil, nil)

	t.Run("invalid json", func(t *testing.T) {
		task := asynq.NewTask(TaskConvertMP3, []byte("{"))
		err := worker.handleConvertMP3(context.Background(), task)
		if err == nil {
			t.Fatal("expected invalid json error")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		task := makeTask(t, ConvertMP3Payload{JobID: "job_x"})
		err := worker.handleConvertMP3(context.Background(), task)
		if err == nil || !strings.Contains(err.Error(), "invalid payload") {
			t.Fatalf("expected invalid payload error, got %v", err)
		}
	})
}

func TestHandleConvertMP3_DefaultBitrateAndR2NotConfigured(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "128k")
	store := newFakeJobStore()
	jobID := "job_default_bitrate"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	worker := makeWorkerForTest(script, store, nil)
	task := makeTask(t, ConvertMP3Payload{
		JobID:     jobID,
		SourceURL: "https://www.youtube.com/watch?v=abc123",
		OutputKey: "yt-downloader/prod/mp3/" + jobID + ".mp3",
		// BitrateKbps intentionally zero -> must fallback to 128
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "r2 client is not configured") {
		t.Fatalf("expected r2-not-configured error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if !strings.Contains(record.Error, "r2 client is not configured") {
		t.Fatalf("expected failure reason in record error, got %q", record.Error)
	}
	if store.updateCalls < 2 {
		t.Fatalf("expected at least two updates (processing + failed), got %d", store.updateCalls)
	}
}

func TestHandleConvertMP3_ConvertFailureMarksFailed(t *testing.T) {
	script := makeFakeYTDLPScript(t, "fail", "192k")
	store := newFakeJobStore()
	jobID := "job_convert_fail"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	worker := makeWorkerForTest(script, store, &fakeR2{})
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 192,
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "yt-dlp convert failed") {
		t.Fatalf("expected convert failure error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if !strings.Contains(record.Error, "yt-dlp convert failed") {
		t.Fatalf("expected convert failure reason in record error, got %q", record.Error)
	}
}

func TestHandleConvertMP3_ProcessingUpdateErrorContinues(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "128k")
	store := newFakeJobStore()
	store.updateErr = errors.New("mark processing failed")
	store.failOnUpdateCall = 1

	jobID := "job_processing_update_error"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	worker := makeWorkerForTest(script, store, nil)
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 128,
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "r2 client is not configured") {
		t.Fatalf("expected r2-not-configured error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected final failed status, got %s", record.Status)
	}
	if store.updateCalls < 2 {
		t.Fatalf("expected multiple update attempts despite first update error, got %d", store.updateCalls)
	}
}

func TestHandleConvertMP3_SuccessMarksDoneAndUploads(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "192k")
	store := newFakeJobStore()
	jobID := "job_success"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	expiresAt := time.Now().UTC().Add(70 * time.Minute).Truncate(time.Second)
	r2 := &fakeR2{
		presignURL:       "https://signed.example.com/job_success.mp3",
		presignExpiresAt: expiresAt,
	}

	worker := makeWorkerForTest(script, store, r2)
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 192,
	})

	if err := worker.handleConvertMP3(context.Background(), task); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusDone {
		t.Fatalf("expected done status, got %s", record.Status)
	}
	if record.Error != "" {
		t.Fatalf("expected empty error on done status, got %q", record.Error)
	}
	if record.DownloadURL != r2.presignURL {
		t.Fatalf("unexpected download url, got %q want %q", record.DownloadURL, r2.presignURL)
	}
	if record.ExpiresAt == nil || !record.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expiresAt, got %+v want %v", record.ExpiresAt, expiresAt)
	}

	if r2.uploadCalls != 1 {
		t.Fatalf("expected one upload call, got %d", r2.uploadCalls)
	}
	if !r2.uploadedFileFound {
		t.Fatalf("expected uploaded file to exist at upload time")
	}
	if r2.presignCalls != 1 {
		t.Fatalf("expected one presign call, got %d", r2.presignCalls)
	}
	if r2.presignTTL != 60*time.Minute {
		t.Fatalf("unexpected presign ttl, got %v", r2.presignTTL)
	}
}

func TestHandleConvertMP3_DoneUpdateFailureReturnsError(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "192k")
	store := newFakeJobStore()
	store.updateErr = errors.New("mark done failed")
	store.failOnUpdateCall = 2

	jobID := "job_done_update_fail"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	r2 := &fakeR2{presignURL: "https://signed.example.com/done-fail.mp3"}
	worker := makeWorkerForTest(script, store, r2)
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 192,
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "mark done failed") {
		t.Fatalf("expected done update failure error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusProcessing {
		t.Fatalf("expected status to remain processing when done update fails, got %s", record.Status)
	}
	if r2.uploadCalls != 1 || r2.presignCalls != 1 {
		t.Fatalf("expected upload and presign to run before done update failure, upload=%d presign=%d", r2.uploadCalls, r2.presignCalls)
	}
}

func TestHandleConvertMP3_R2UploadFailureMarksFailed(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "160k")
	store := newFakeJobStore()
	jobID := "job_upload_fail"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	r2 := &fakeR2{uploadErr: errors.New("upload failed")}
	worker := makeWorkerForTest(script, store, r2)
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 160,
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "upload failed") {
		t.Fatalf("expected upload failure error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if !strings.Contains(record.Error, "upload failed") {
		t.Fatalf("expected upload failure in record error, got %q", record.Error)
	}
	if r2.presignCalls != 0 {
		t.Fatalf("expected no presign call when upload fails, got %d", r2.presignCalls)
	}
}

func TestHandleConvertMP3_PresignFailureMarksFailed(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "192k")
	store := newFakeJobStore()
	jobID := "job_presign_fail"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	r2 := &fakeR2{presignErr: errors.New("presign failed")}
	worker := makeWorkerForTest(script, store, r2)
	task := makeTask(t, ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/" + jobID + ".mp3",
		BitrateKbps: 192,
	})

	err := worker.handleConvertMP3(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "presign failed") {
		t.Fatalf("expected presign failure error, got %v", err)
	}

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if !strings.Contains(record.Error, "presign failed") {
		t.Fatalf("expected presign failure in record error, got %q", record.Error)
	}
}

func TestConvertMP3_RequiresYTDLPBinary(t *testing.T) {
	worker := makeWorkerForTest("", nil, nil)

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

func TestConvertMP3_MkTempDirError(t *testing.T) {
	script := makeFakeYTDLPScript(t, "success", "128k")
	worker := makeWorkerForTest(script, nil, nil)
	worker.mkTempDir = func(string, string) (string, error) {
		return "", errors.New("mktemp failed")
	}

	_, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_mktemp_fail",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_mktemp_fail.mp3",
		BitrateKbps: 128,
	})
	if cleanup != nil {
		t.Fatalf("expected nil cleanup when mktemp fails")
	}
	if err == nil || !strings.Contains(err.Error(), "create temp dir") {
		t.Fatalf("expected create temp dir error, got %v", err)
	}
}

func TestConvertMP3_CommandFailureIncludesStderr(t *testing.T) {
	script := makeFakeYTDLPScript(t, "fail", "128k")
	worker := makeWorkerForTest(script, nil, nil)

	_, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_fail",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_fail.mp3",
		BitrateKbps: 128,
	})
	if cleanup != nil {
		t.Fatalf("expected cleanup to be nil after command failure")
	}
	if err == nil || !strings.Contains(err.Error(), "simulated yt-dlp failure") {
		t.Fatalf("expected wrapped yt-dlp stderr error, got %v", err)
	}
}

func TestConvertMP3_CommandStartFailureUsesRunErrorText(t *testing.T) {
	missingBinary := filepath.Join(t.TempDir(), "missing-yt-dlp")
	worker := makeWorkerForTest(missingBinary, nil, nil)

	_, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_start_fail",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_start_fail.mp3",
		BitrateKbps: 128,
	})
	if cleanup != nil {
		t.Fatalf("expected cleanup to be nil after command start failure")
	}
	if err == nil || !strings.Contains(err.Error(), "yt-dlp convert failed") {
		t.Fatalf("expected wrapped convert failure, got %v", err)
	}
	if strings.Contains(err.Error(), "simulated yt-dlp failure") {
		t.Fatalf("expected fallback to run error text, got stderr-based error: %v", err)
	}
}

func TestConvertMP3_UsesJSRuntimeHeadersAndUserAgent(t *testing.T) {
	script := makeFakeYTDLPScriptAssertOptions(
		t,
		"node",
		"Referer: https://www.youtube.com/",
		"Mozilla/5.0 QueueTest",
	)

	worker := makeWorkerForTest(script, nil, nil)
	worker.cfg.YTDLPJSRuntimes = "node"

	outputPath, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_opts",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_opts.mp3",
		BitrateKbps: 128,
		Headers: map[string]string{
			" ":       "ignore-me",
			"Referer": "https://www.youtube.com/",
		},
		UserAgent: "Mozilla/5.0 QueueTest",
	})
	if err != nil {
		t.Fatalf("expected option-assert conversion success, got %v", err)
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup function")
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	cleanup()
}

func TestConvertMP3_GlobFallback(t *testing.T) {
	script := makeFakeYTDLPScript(t, "altname", "128k")
	worker := makeWorkerForTest(script, nil, nil)

	outputPath, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_glob",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_glob.mp3",
		BitrateKbps: 128,
	})
	if err != nil {
		t.Fatalf("expected glob fallback success, got err: %v", err)
	}
	if !strings.HasSuffix(outputPath, "alt-output.mp3") {
		t.Fatalf("expected glob fallback file path, got %s", outputPath)
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup function")
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist before cleanup: %v", err)
	}

	dir := filepath.Dir(outputPath)
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected temp dir to be removed by cleanup, stat err=%v", err)
	}
}

func TestConvertMP3_NoOutputFile(t *testing.T) {
	script := makeFakeYTDLPScript(t, "nooutput", "128k")
	worker := makeWorkerForTest(script, nil, nil)

	_, cleanup, err := worker.convertMP3(context.Background(), ConvertMP3Payload{
		JobID:       "job_no_output",
		SourceURL:   "https://www.youtube.com/watch?v=abc123",
		OutputKey:   "yt-downloader/prod/mp3/job_no_output.mp3",
		BitrateKbps: 128,
	})
	if cleanup != nil {
		t.Fatalf("expected cleanup to be nil when no output was produced")
	}
	if err == nil || !strings.Contains(err.Error(), "mp3 output not found") {
		t.Fatalf("expected no-output error, got %v", err)
	}
}

func TestFailJob_NoStoreReturnsOriginalError(t *testing.T) {
	worker := makeWorkerForTest("", nil, nil)
	errIn := errors.New("boom")
	errOut := worker.failJob(context.Background(), "job_test", errIn)
	if errOut == nil || errOut.Error() != "boom" {
		t.Fatalf("expected original error, got %v", errOut)
	}
}

func TestFailJob_UpdateErrorStillReturnsOriginalError(t *testing.T) {
	store := newFakeJobStore()
	store.updateErr = errors.New("store update failed")
	store.failOnUpdateCall = 1

	worker := makeWorkerForTest("", store, nil)
	errIn := errors.New("boom")
	errOut := worker.failJob(context.Background(), "job_fail_update", errIn)
	if errOut == nil || errOut.Error() != "boom" {
		t.Fatalf("expected original error even when store update fails, got %v", errOut)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected one update attempt, got %d", store.updateCalls)
	}
}

func TestFailJob_ClipsErrorWhenStoreAvailable(t *testing.T) {
	store := newFakeJobStore()
	jobID := "job_clip"
	store.records[jobID] = jobs.Record{ID: jobID, Status: jobs.StatusQueued}

	worker := makeWorkerForTest("", store, nil)
	longErr := errors.New(strings.Repeat("x", 450))
	_ = worker.failJob(context.Background(), jobID, longErr)

	record := store.records[jobID]
	if record.Status != jobs.StatusFailed {
		t.Fatalf("expected failed status, got %s", record.Status)
	}
	if len(record.Error) != 400 {
		t.Fatalf("expected clipped error length 400, got %d", len(record.Error))
	}
}
