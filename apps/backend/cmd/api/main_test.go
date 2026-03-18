package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/youtube"
)

type fakeAPIServer struct {
	handler   http.Handler
	closeCall int
}

func (f *fakeAPIServer) Handler() http.Handler {
	if f.handler == nil {
		f.handler = http.NewServeMux()
	}
	return f.handler
}

func (f *fakeAPIServer) Close() {
	f.closeCall++
}

func withAPITestOverrides(t *testing.T) {
	t.Helper()

	oldLookPath := apiLookPath
	oldListen := apiListen
	oldResolver := newResolver
	oldServer := newServer

	t.Cleanup(func() {
		apiLookPath = oldLookPath
		apiListen = oldListen
		newResolver = oldResolver
		newServer = oldServer
	})
}

func TestRun_LookPathError(t *testing.T) {
	withAPITestOverrides(t)

	apiLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	cfg := config.Config{YTDLPBinary: "yt-dlp"}
	err := run(cfg, nil)
	if err == nil {
		t.Fatal("expected lookpath error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "yt-dlp binary not found") || !strings.Contains(got, "YTDLP_BINARY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_UsesHTTPPortFallbackAndClosesServerOnListenError(t *testing.T) {
	withAPITestOverrides(t)

	apiLookPath = func(name string) (string, error) {
		if name == "yt-dlp" {
			return "/usr/bin/yt-dlp", nil
		}
		return "", errors.New("unexpected lookpath")
	}

	server := &fakeAPIServer{}
	capturedAddr := ""
	capturedBinary := ""

	newServer = func(cfg config.Config, _ *log.Logger, _ *youtube.Resolver) apiServer {
		capturedBinary = cfg.YTDLPBinary
		return server
	}

	apiListen = func(addr string, handler http.Handler) error {
		capturedAddr = addr
		if handler == nil {
			t.Fatalf("expected non-nil handler")
		}
		return errors.New("listen failed")
	}

	cfg := config.Config{
		YTDLPBinary:             "yt-dlp",
		HTTPPort:                "18080",
		HTTPAddr:                "",
		YTDLPJSRuntimes:         "node",
		MaxVideoDurationMinutes: 60,
		YouTubeMaxQuality:       1080,
		MaxFileSizeBytes:        0,
	}

	err := run(cfg, log.New(io.Discard, "", 0))
	if err == nil || !strings.Contains(err.Error(), "api server stopped") || !strings.Contains(err.Error(), "listen failed") {
		t.Fatalf("expected wrapped listen error, got %v", err)
	}
	if capturedAddr != ":18080" {
		t.Fatalf("expected fallback listen addr :18080, got %s", capturedAddr)
	}
	if capturedBinary != "/usr/bin/yt-dlp" {
		t.Fatalf("expected resolved ytdlp binary passed to server, got %s", capturedBinary)
	}
	if server.closeCall != 1 {
		t.Fatalf("expected server close called once, got %d", server.closeCall)
	}
}

func TestRun_UsesExplicitHTTPAddrAndReturnsNilOnSuccess(t *testing.T) {
	withAPITestOverrides(t)

	apiLookPath = func(name string) (string, error) {
		if name == "yt-dlp" {
			return "/custom/yt-dlp", nil
		}
		return "", errors.New("unexpected lookpath")
	}

	server := &fakeAPIServer{}
	capturedAddr := ""

	newServer = func(_ config.Config, _ *log.Logger, _ *youtube.Resolver) apiServer {
		return server
	}
	apiListen = func(addr string, handler http.Handler) error {
		capturedAddr = addr
		if handler == nil {
			t.Fatalf("expected non-nil handler")
		}
		return nil
	}

	cfg := config.Config{
		YTDLPBinary:             "yt-dlp",
		HTTPPort:                "9999",
		HTTPAddr:                "127.0.0.1:19090",
		YTDLPJSRuntimes:         "node",
		MaxVideoDurationMinutes: 60,
		YouTubeMaxQuality:       1080,
		MaxFileSizeBytes:        0,
	}

	err := run(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("expected run success, got %v", err)
	}
	if capturedAddr != "127.0.0.1:19090" {
		t.Fatalf("expected explicit listen addr to be used, got %s", capturedAddr)
	}
	if server.closeCall != 1 {
		t.Fatalf("expected server close called once, got %d", server.closeCall)
	}
}

