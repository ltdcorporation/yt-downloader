package main

import (
	"log"
	"net/http"
	"os/exec"
	"strings"

	"yt-downloader/backend/internal/config"
	httplayer "yt-downloader/backend/internal/http"
	"yt-downloader/backend/internal/youtube"
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

	resolver := youtube.NewResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.MaxVideoDurationMinutes,
		cfg.YouTubeMaxQuality,
		cfg.MaxFileSizeBytes,
	)
	server := httplayer.NewServer(cfg, logger, resolver)
	defer server.Close()

	listenAddr := strings.TrimSpace(cfg.HTTPAddr)
	if listenAddr == "" {
		listenAddr = ":" + cfg.HTTPPort
	}

	logger.Printf("api server starting on %s (env=%s)", listenAddr, cfg.AppEnv)
	if err := http.ListenAndServe(listenAddr, server.Handler()); err != nil {
		logger.Fatalf("api server stopped: %v", err)
	}
}
