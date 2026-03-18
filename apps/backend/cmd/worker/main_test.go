package main

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/jobs"
)

type fakeWorkerStore struct {
	closeErr   error
	closeCalls int
}

func (f *fakeWorkerStore) Update(_ context.Context, _ string, _ func(*jobs.Record)) (jobs.Record, error) {
	return jobs.Record{}, nil
}

func (f *fakeWorkerStore) Close() error {
	f.closeCalls++
	return f.closeErr
}

type fakeWorkerR2 struct{}

func (fakeWorkerR2) UploadFile(context.Context, string, string) error {
	return nil
}

func (fakeWorkerR2) PresignDownloadURL(context.Context, string, time.Duration) (string, time.Time, error) {
	return "", time.Time{}, nil
}

type fakeRunner struct {
	runErr   error
	runCalls int
}

func (f *fakeRunner) Run(context.Context) error {
	f.runCalls++
	return f.runErr
}

func withWorkerTestOverrides(t *testing.T) {
	t.Helper()

	oldLookPath := workerLookPath
	oldStoreFactory := newJobStore
	oldR2Factory := newR2Client
	oldWorkerFactory := newWorker

	t.Cleanup(func() {
		workerLookPath = oldLookPath
		newJobStore = oldStoreFactory
		newR2Client = oldR2Factory
		newWorker = oldWorkerFactory
	})
}

func TestRun_YTDLPLookPathError(t *testing.T) {
	withWorkerTestOverrides(t)

	workerLookPath = func(string) (string, error) {
		return "", errors.New("missing binary")
	}

	cfg := config.Config{YTDLPBinary: "yt-dlp"}
	err := run(cfg, nil)
	if err == nil {
		t.Fatal("expected lookpath error")
	}
	if !strings.Contains(err.Error(), "yt-dlp binary not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_SuccessWithFFmpegWarningAndR2Warning(t *testing.T) {
	withWorkerTestOverrides(t)

	workerLookPath = func(name string) (string, error) {
		switch name {
		case "yt-dlp":
			return "/usr/bin/yt-dlp", nil
		case "ffmpeg":
			return "", errors.New("ffmpeg missing")
		default:
			return "", errors.New("unexpected lookup")
		}
	}

	store := &fakeWorkerStore{closeErr: errors.New("close failed")}
	runner := &fakeRunner{}
	capturedBinary := ""
	capturedR2Nil := false
	newJobStore = func(cfg config.Config, _ *log.Logger) workerStore {
		capturedBinary = cfg.YTDLPBinary
		return store
	}
	newR2Client = func(context.Context, config.Config) (workerR2Client, error) {
		return nil, errors.New("r2 not configured")
	}
	newWorker = func(_ config.Config, _ *log.Logger, _ workerStore, r2 workerR2Client) workerRunner {
		capturedR2Nil = r2 == nil
		return runner
	}

	cfg := config.Config{YTDLPBinary: "yt-dlp", RedisAddr: "127.0.0.1:6382"}
	err := run(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("expected run success, got %v", err)
	}
	if capturedBinary != "/usr/bin/yt-dlp" {
		t.Fatalf("expected resolved ytdlp binary passed to store, got %s", capturedBinary)
	}
	if !capturedR2Nil {
		t.Fatalf("expected nil r2 passed to worker when r2 init fails")
	}
	if runner.runCalls != 1 {
		t.Fatalf("expected worker run called once, got %d", runner.runCalls)
	}
	if store.closeCalls != 1 {
		t.Fatalf("expected store close called once, got %d", store.closeCalls)
	}
}

func TestRun_WorkerErrorWrapped(t *testing.T) {
	withWorkerTestOverrides(t)

	workerLookPath = func(name string) (string, error) {
		switch name {
		case "yt-dlp":
			return "/usr/local/bin/yt-dlp", nil
		case "ffmpeg":
			return "/usr/bin/ffmpeg", nil
		default:
			return "", errors.New("unexpected lookup")
		}
	}

	store := &fakeWorkerStore{}
	runner := &fakeRunner{runErr: errors.New("worker crashed")}
	newJobStore = func(config.Config, *log.Logger) workerStore {
		return store
	}
	newR2Client = func(context.Context, config.Config) (workerR2Client, error) {
		return fakeWorkerR2{}, nil
	}
	newWorker = func(_ config.Config, _ *log.Logger, _ workerStore, _ workerR2Client) workerRunner {
		return runner
	}

	cfg := config.Config{YTDLPBinary: "yt-dlp", RedisAddr: "127.0.0.1:6382"}
	err := run(cfg, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("expected worker error")
	}
	if !strings.Contains(err.Error(), "worker stopped") || !strings.Contains(err.Error(), "worker crashed") {
		t.Fatalf("unexpected wrapped worker error: %v", err)
	}
	if store.closeCalls != 1 {
		t.Fatalf("expected store close called even on worker error, got %d", store.closeCalls)
	}
}
