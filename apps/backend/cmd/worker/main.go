package main

import (
	"context"
	"log"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/queue"
)

func main() {
	cfg := config.Load()
	logger := log.Default()
	worker := queue.NewWorker(cfg, logger)

	logger.Printf("worker starting (redis=%s)", cfg.RedisAddr)
	if err := worker.Run(context.Background()); err != nil {
		logger.Fatalf("worker stopped: %v", err)
	}
}
