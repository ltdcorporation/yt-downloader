package main

import (
	"context"
	"log"
	"os/exec"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/storage"
)

func main() {
	cfg := config.Load()
	logger := log.Default()

	resolvedYTDLPBinary, err := exec.LookPath(cfg.YTDLPBinary)
	if err != nil {
		logger.Fatalf("yt-dlp binary not found (YTDLP_BINARY=%q): %v", cfg.YTDLPBinary, err)
	}
	cfg.YTDLPBinary = resolvedYTDLPBinary
	logger.Printf("yt-dlp binary resolved: %s", cfg.YTDLPBinary)

	if ffmpegBinary, err := exec.LookPath("ffmpeg"); err != nil {
		logger.Printf("warning: ffmpeg binary not found in PATH, MP3 conversion may fail: %v", err)
	} else {
		logger.Printf("ffmpeg binary resolved: %s", ffmpegBinary)
	}

	jobStore := jobs.NewStore(cfg, logger)
	defer func() {
		if err := jobStore.Close(); err != nil {
			logger.Printf("warning: close job store: %v", err)
		}
	}()

	r2Client, err := storage.NewR2Client(context.Background(), cfg)
	if err != nil {
		logger.Printf("warning: r2 is not ready, mp3 jobs will fail until configured (%v)", err)
	}

	worker := queue.NewWorker(cfg, logger, jobStore, r2Client)

	logger.Printf("worker starting (redis=%s)", cfg.RedisAddr)
	if err := worker.Run(context.Background()); err != nil {
		logger.Fatalf("worker stopped: %v", err)
	}
}
