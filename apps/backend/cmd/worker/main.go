package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/storage"
)

type workerStore interface {
	Update(ctx context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error)
	Close() error
}

type workerR2Client interface {
	UploadFile(ctx context.Context, key string, localPath string) error
	PresignDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, time.Time, error)
}

type workerRunner interface {
	Run(ctx context.Context) error
}

var (
	workerLookPath = exec.LookPath
	newJobStore    = func(cfg config.Config, logger *log.Logger) workerStore {
		return jobs.NewStore(cfg, logger)
	}
	newR2Client = func(ctx context.Context, cfg config.Config) (workerR2Client, error) {
		return storage.NewR2Client(ctx, cfg)
	}
	newWorker = func(cfg config.Config, logger *log.Logger, store workerStore, r2 workerR2Client) workerRunner {
		return queue.NewWorker(cfg, logger, store, r2)
	}
)

func run(cfg config.Config, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	resolvedYTDLPBinary, err := workerLookPath(cfg.YTDLPBinary)
	if err != nil {
		return fmt.Errorf("yt-dlp binary not found (YTDLP_BINARY=%q): %w", cfg.YTDLPBinary, err)
	}
	cfg.YTDLPBinary = resolvedYTDLPBinary
	logger.Printf("yt-dlp binary resolved: %s", cfg.YTDLPBinary)

	if ffmpegBinary, err := workerLookPath("ffmpeg"); err != nil {
		logger.Printf("warning: ffmpeg binary not found in PATH, MP3 conversion may fail: %v", err)
	} else {
		logger.Printf("ffmpeg binary resolved: %s", ffmpegBinary)
	}

	jobStore := newJobStore(cfg, logger)
	defer func() {
		if err := jobStore.Close(); err != nil {
			logger.Printf("warning: close job store: %v", err)
		}
	}()

	r2Client, err := newR2Client(context.Background(), cfg)
	if err != nil {
		logger.Printf("warning: r2 is not ready, mp3 jobs will fail until configured (%v)", err)
	}

	worker := newWorker(cfg, logger, jobStore, r2Client)

	logger.Printf("worker starting (redis=%s)", cfg.RedisAddr)
	if err := worker.Run(context.Background()); err != nil {
		return fmt.Errorf("worker stopped: %w", err)
	}

	return nil
}

func main() {
	cfg := config.Load()
	logger := log.Default()
	if err := run(cfg, logger); err != nil {
		logger.Fatalf("%v", err)
	}
}
