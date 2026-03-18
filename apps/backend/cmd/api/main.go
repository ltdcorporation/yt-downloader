package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"yt-downloader/backend/internal/config"
	httplayer "yt-downloader/backend/internal/http"
	"yt-downloader/backend/internal/youtube"
)

type apiServer interface {
	Handler() http.Handler
	Close()
}

var (
	apiLookPath = exec.LookPath
	apiListen   = http.ListenAndServe
	newResolver = youtube.NewResolver
	newServer   = func(cfg config.Config, logger *log.Logger, resolver *youtube.Resolver) apiServer {
		return httplayer.NewServer(cfg, logger, resolver)
	}
)

func run(cfg config.Config, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	resolvedYTDLPBinary, err := apiLookPath(cfg.YTDLPBinary)
	if err != nil {
		return fmt.Errorf("yt-dlp binary not found (YTDLP_BINARY=%q): %w", cfg.YTDLPBinary, err)
	}
	cfg.YTDLPBinary = resolvedYTDLPBinary
	logger.Printf("yt-dlp binary resolved: %s", cfg.YTDLPBinary)

	resolver := newResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.MaxVideoDurationMinutes,
		cfg.YouTubeMaxQuality,
		cfg.MaxFileSizeBytes,
	)
	server := newServer(cfg, logger, resolver)
	defer server.Close()

	listenAddr := strings.TrimSpace(cfg.HTTPAddr)
	if listenAddr == "" {
		listenAddr = ":" + cfg.HTTPPort
	}

	logger.Printf("api server starting on %s (env=%s)", listenAddr, cfg.AppEnv)
	if err := apiListen(listenAddr, server.Handler()); err != nil {
		return fmt.Errorf("api server stopped: %w", err)
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
