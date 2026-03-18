package xresolver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeYTDLPScriptOptions struct {
	Stdout         string
	Stderr         string
	ExitCode       int
	ExpectedURL    string
	ExpectedCookie string
	RequireCookie  bool
}

func makeFakeYTDLPScript(t *testing.T, opts fakeYTDLPScriptOptions) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

stdout_payload=$(cat <<'__JSON__'
%s
__JSON__
)

expected_url=%q
expected_cookie=%q
require_cookie=%q
stderr_text=%q
exit_code=%d

captured_url=""
captured_cookie=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cookies)
      captured_cookie="$2"
      shift 2
      ;;
    --js-runtimes|--add-header|--user-agent)
      shift 2
      ;;
    --ignore-config|--dump-single-json|--no-playlist|--skip-download|--no-warnings)
      shift
      ;;
    *)
      captured_url="$1"
      shift
      ;;
  esac
done

if [[ -n "$expected_url" && "$captured_url" != "$expected_url" ]]; then
  echo "unexpected target URL: $captured_url" >&2
  exit 91
fi

if [[ "$require_cookie" == "true" && -z "$captured_cookie" ]]; then
  echo "cookie file is required" >&2
  exit 92
fi

if [[ -n "$expected_cookie" && "$captured_cookie" != "$expected_cookie" ]]; then
  echo "unexpected cookie file: $captured_cookie" >&2
  exit 93
fi

if [[ "$exit_code" -ne 0 ]]; then
  if [[ -n "$stderr_text" ]]; then
    echo "$stderr_text" >&2
  fi
  exit "$exit_code"
fi

printf '%%s' "$stdout_payload"
`,
		opts.Stdout,
		opts.ExpectedURL,
		opts.ExpectedCookie,
		fmt.Sprintf("%v", opts.RequireCookie),
		opts.Stderr,
		opts.ExitCode,
	)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake yt-dlp script: %v", err)
	}

	return path
}

func mustJSON(t *testing.T, payload ytdlpOutput) string {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return string(encoded)
}

func progressiveFormat(id string, height int, mediaURL string, size int64) ytdlpFormat {
	return ytdlpFormat{
		FormatID:       id,
		Ext:            "mp4",
		VideoCodec:     "avc1",
		AudioCodec:     "mp4a",
		URL:            mediaURL,
		Height:         height,
		Filesize:       size,
		FilesizeApprox: 0,
	}
}

func TestValidateXURL(t *testing.T) {
	valid := []string{
		"https://x.com/user/status/12345",
		"https://www.x.com/user/status/12345",
		"https://twitter.com/user/status/12345",
		"https://www.twitter.com/user/status/12345",
		"https://mobile.twitter.com/user/status/12345",
		"https://x.com/i/status/12345",
	}

	for _, input := range valid {
		if err := validateXURL(input); err != nil {
			t.Fatalf("expected valid URL (%s), got err: %v", input, err)
		}
	}

	tests := []struct {
		name string
		url  string
		err  string
	}{
		{
			name: "invalid parse",
			url:  "://bad-url",
			err:  "invalid URL",
		},
		{
			name: "unsupported scheme",
			url:  "ftp://x.com/user/status/1",
			err:  "URL must start with http or https",
		},
		{
			name: "invalid host",
			url:  "https://example.com/user/status/1",
			err:  "URL must be an X/Twitter link",
		},
		{
			name: "unsupported path",
			url:  "https://x.com/user/lists/1",
			err:  "unsupported X URL path",
		},
		{
			name: "missing status id",
			url:  "https://x.com/user/status/",
			err:  "unsupported X URL path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateXURL(tc.url)
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
			}
		})
	}
}

func TestBuildCookieCandidates(t *testing.T) {
	cookiesDir := t.TempDir()
	fileA := filepath.Join(cookiesDir, "akun-a.txt")
	fileB := filepath.Join(cookiesDir, "akun-b.txt")
	hidden := filepath.Join(cookiesDir, ".hidden")

	if err := os.WriteFile(fileA, []byte("# a"), 0o600); err != nil {
		t.Fatalf("failed to write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("# b"), 0o600); err != nil {
		t.Fatalf("failed to write fileB: %v", err)
	}
	if err := os.WriteFile(hidden, []byte("# hidden"), 0o600); err != nil {
		t.Fatalf("failed to write hidden file: %v", err)
	}

	resolver := NewResolver(
		"yt-dlp",
		"node",
		1080,
		0,
		cookiesDir,
		fileA+", "+fileA,
		true,
	)

	candidates := resolver.buildCookieCandidates()
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates (public + 2 cookie files), got %d", len(candidates))
	}
	if candidates[0].profile != "" || candidates[0].path != "" {
		t.Fatalf("expected first candidate to be public fallback, got %#v", candidates[0])
	}

	profiles := []string{candidates[1].profile, candidates[2].profile}
	joined := strings.Join(profiles, ",")
	if !strings.Contains(joined, "akun-a") || !strings.Contains(joined, "akun-b") {
		t.Fatalf("expected akun-a and akun-b profiles, got %#v", profiles)
	}
}

func TestResolve_RotatesToCookieProfile(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), "acc-main.txt")
	if err := os.WriteFile(cookieFile, []byte("# cookie"), 0o600); err != nil {
		t.Fatalf("failed to write cookie file: %v", err)
	}

	payload := ytdlpOutput{
		Title:    "X Test Video",
		Duration: 12.5,
		Formats: []ytdlpFormat{
			progressiveFormat("18", 360, "https://video-cdn.example/18.mp4", 12345),
		},
	}

	script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
		Stdout:         mustJSON(t, payload),
		ExpectedURL:    "https://x.com/user/status/12345",
		ExpectedCookie: cookieFile,
		RequireCookie:  true,
	})

	resolver := NewResolver(
		script,
		"",
		1080,
		0,
		"",
		cookieFile,
		true,
	)

	result, err := resolver.Resolve(context.Background(), "https://x.com/user/status/12345")
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}
	if result.Title != "X Test Video" {
		t.Fatalf("unexpected title: %s", result.Title)
	}
	if result.CookieProfile != "acc-main" {
		t.Fatalf("expected cookie profile acc-main, got %q", result.CookieProfile)
	}
	if result.DurationSeconds != 13 {
		t.Fatalf("expected rounded duration 13, got %d", result.DurationSeconds)
	}
	if len(result.Formats) != 1 || result.Formats[0].ID != "18" {
		t.Fatalf("unexpected formats: %#v", result.Formats)
	}
}

func TestResolve_NoProfilesConfigured(t *testing.T) {
	resolver := NewResolver(
		"yt-dlp",
		"",
		1080,
		0,
		"",
		"",
		false,
	)

	_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/12345")
	if err == nil || !strings.Contains(err.Error(), "no cookie profile configured") {
		t.Fatalf("expected no profile error, got %v", err)
	}
}

func TestResolve_PublicModeSuccess(t *testing.T) {
	payload := ytdlpOutput{
		Title:      "Public X Video",
		Thumbnail:  "https://img.example/thumb.jpg",
		LiveStatus: "not_live",
		Formats: []ytdlpFormat{
			progressiveFormat("22", 720, "https://video-cdn.example/22.mp4", 123),
			{FormatID: "137", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", URL: "https://video-cdn.example/137.mp4", Height: 1080},
		},
	}

	script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
		Stdout:      mustJSON(t, payload),
		ExpectedURL: "https://twitter.com/user/status/987654321",
	})

	resolver := NewResolver(
		script,
		"node",
		1080,
		0,
		"",
		"",
		true,
	)

	result, err := resolver.Resolve(context.Background(), "https://twitter.com/user/status/987654321")
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}
	if result.CookieProfile != "" {
		t.Fatalf("expected empty cookie profile for public mode, got %q", result.CookieProfile)
	}
	if len(result.Formats) != 1 {
		t.Fatalf("expected 1 progressive format, got %d", len(result.Formats))
	}
	if result.Formats[0].ID != "22" || result.Formats[0].Type != "mp4" {
		t.Fatalf("unexpected format payload: %#v", result.Formats[0])
	}
}

func TestResolve_WithCurlInput(t *testing.T) {
	payload := ytdlpOutput{
		Title: "Curl Input",
		Formats: []ytdlpFormat{
			progressiveFormat("18", 360, "https://video-cdn.example/18.mp4", 100),
		},
	}

	script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
		Stdout:      mustJSON(t, payload),
		ExpectedURL: "https://x.com/user/status/1111",
	})

	resolver := NewResolver(
		script,
		"",
		1080,
		0,
		"",
		"",
		true,
	)

	input := `curl "https://x.com/user/status/1111" -H "Referer: https://x.com/" -A "Mozilla/5.0 Test"`
	result, err := resolver.Resolve(context.Background(), input)
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}
	if result.Title != "Curl Input" {
		t.Fatalf("unexpected title: %s", result.Title)
	}
}
