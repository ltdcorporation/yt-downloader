package main

import (
	"log"
	"net/http"

	"yt-downloader/backend/internal/config"
	httplayer "yt-downloader/backend/internal/http"
	"yt-downloader/backend/internal/youtube"
)

func main() {
	cfg := config.Load()
	logger := log.Default()
	resolver := youtube.NewResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.MaxVideoDurationMinutes,
		cfg.YouTubeMaxQuality,
		cfg.MaxFileSizeBytes,
	)
	server := httplayer.NewServer(cfg, logger, resolver)
	defer server.Close()

	logger.Printf("api server starting on :%s (env=%s)", cfg.HTTPPort, cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, server.Handler()); err != nil {
		logger.Fatalf("api server stopped: %v", err)
	}
}
