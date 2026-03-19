package igresolver

import (
	"context"
	"encoding/json"
	"errors"
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

func TestValidateInstagramURL(t *testing.T) {
	valid := []string{
		"https://instagram.com/p/ABC123/",
		"https://www.instagram.com/reel/ABC123/",
		"https://m.instagram.com/reel/ABC123/?utm_source=test",
		"https://www.instagr.am/tv/ABC123/",
		"https://instagram.com/reels/ABC123",
	}

	for _, input := range valid {
		if err := validateInstagramURL(input); err != nil {
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
			url:  "ftp://instagram.com/reel/ABC123",
			err:  "URL must start with http or https",
		},
		{
			name: "invalid host",
			url:  "https://example.com/reel/ABC123",
			err:  "URL must be an Instagram link",
		},
		{
			name: "unsupported path",
			url:  "https://instagram.com/stories/user/123",
			err:  "unsupported Instagram URL path",
		},
		{
			name: "missing id",
			url:  "https://instagram.com/reel/",
			err:  "unsupported Instagram URL path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInstagramURL(tc.url)
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
		Title:    "IG Test Video",
		Duration: 12.5,
		Formats: []ytdlpFormat{
			progressiveFormat("18", 360, "https://video-cdn.example/18.mp4", 12345),
		},
	}

	script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
		Stdout:         mustJSON(t, payload),
		ExpectedURL:    "https://instagram.com/reel/ABC123",
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

	result, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}
	if result.Title != "IG Test Video" {
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

	_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
	if err == nil || !strings.Contains(err.Error(), "no cookie profile configured") {
		t.Fatalf("expected no profile error, got %v", err)
	}
}

func TestResolve_PublicModeSuccess(t *testing.T) {
	payload := ytdlpOutput{
		Title:      "Public IG Video",
		Thumbnail:  "https://img.example/thumb.jpg",
		LiveStatus: "not_live",
		Formats: []ytdlpFormat{
			progressiveFormat("22", 720, "https://video-cdn.example/22.mp4", 123),
			{FormatID: "137", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", URL: "https://video-cdn.example/137.mp4", Height: 1080},
		},
	}

	script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
		Stdout:      mustJSON(t, payload),
		ExpectedURL: "https://www.instagram.com/p/ABCD1234",
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

	result, err := resolver.Resolve(context.Background(), "https://www.instagram.com/p/ABCD1234")
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
		ExpectedURL: "https://instagram.com/reel/ZZZ111",
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

	input := `curl "https://instagram.com/reel/ZZZ111" -H "Referer: https://instagram.com/" -A "Mozilla/5.0 Test"`
	result, err := resolver.Resolve(context.Background(), input)
	if err != nil {
		t.Fatalf("expected resolve success, got err: %v", err)
	}
	if result.Title != "Curl Input" {
		t.Fatalf("unexpected title: %s", result.Title)
	}
}

func TestNewResolver_DefaultAndExplicitQuality(t *testing.T) {
	fallback := NewResolver("yt-dlp", "  node  ", 0, 777, "  /tmp/ig-cookies  ", "  /tmp/a.txt,/tmp/b.txt  ", true)
	if fallback.maxQuality != 1080 {
		t.Fatalf("expected fallback maxQuality=1080, got %d", fallback.maxQuality)
	}
	if fallback.ytdlpJSRuntimes != "node" {
		t.Fatalf("expected trimmed js runtime, got %q", fallback.ytdlpJSRuntimes)
	}
	if fallback.cookiesDir != "/tmp/ig-cookies" {
		t.Fatalf("expected trimmed cookiesDir, got %q", fallback.cookiesDir)
	}
	if fallback.cookiesFiles != "/tmp/a.txt,/tmp/b.txt" {
		t.Fatalf("expected trimmed cookiesFiles, got %q", fallback.cookiesFiles)
	}
	if fallback.maxFileSizeBytes != 777 {
		t.Fatalf("unexpected maxFileSizeBytes: %d", fallback.maxFileSizeBytes)
	}
	if fallback.tryWithoutCookieFile != true {
		t.Fatalf("expected tryWithoutCookieFile=true")
	}

	explicit := NewResolver("yt-dlp", "", 720, 0, "", "", false)
	if explicit.maxQuality != 720 {
		t.Fatalf("expected explicit maxQuality=720, got %d", explicit.maxQuality)
	}
	if explicit.tryWithoutCookieFile != false {
		t.Fatalf("expected tryWithoutCookieFile=false")
	}
}

func TestResolve_EarlyValidationErrors(t *testing.T) {
	t.Run("requires ytdlp binary", func(t *testing.T) {
		resolver := NewResolver("", "", 1080, 0, "", "", true)
		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil || !strings.Contains(err.Error(), "yt-dlp binary is not configured") {
			t.Fatalf("expected ytdlp binary error, got %v", err)
		}
	})

	t.Run("parse input error", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)
		_, err := resolver.Resolve(context.Background(), `curl "https://instagram.com/reel/ABC123`)
		if err == nil || !strings.Contains(err.Error(), "failed to parse cURL input") {
			t.Fatalf("expected parse input error, got %v", err)
		}
	})

	t.Run("invalid host", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)
		_, err := resolver.Resolve(context.Background(), "https://example.com/reel/ABC123")
		if err == nil || !strings.Contains(err.Error(), "URL must be an Instagram link") {
			t.Fatalf("expected instagram host validation error, got %v", err)
		}
	})
}

func TestResolve_FailureWrappingByCandidateCount(t *testing.T) {
	t.Run("single candidate returns direct error", func(t *testing.T) {
		cookieFile := filepath.Join(t.TempDir(), "acc-single.txt")
		if err := os.WriteFile(cookieFile, []byte("# cookie"), 0o600); err != nil {
			t.Fatalf("failed to write cookie file: %v", err)
		}

		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			Stdout:   "{}",
			Stderr:   "geo blocked",
			ExitCode: 1,
		})

		resolver := NewResolver(script, "", 1080, 0, "", cookieFile, false)
		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), `cookie profile "acc-single" failed: geo blocked`) {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(err.Error(), "failed to resolve Instagram URL with") {
			t.Fatalf("single candidate should not be wrapped with aggregate message: %v", err)
		}
	})

	t.Run("multiple candidates returns aggregate error", func(t *testing.T) {
		dir := t.TempDir()
		fileA := filepath.Join(dir, "acc-a.txt")
		fileB := filepath.Join(dir, "acc-b.txt")
		if err := os.WriteFile(fileA, []byte("# a"), 0o600); err != nil {
			t.Fatalf("failed to write fileA: %v", err)
		}
		if err := os.WriteFile(fileB, []byte("# b"), 0o600); err != nil {
			t.Fatalf("failed to write fileB: %v", err)
		}

		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			Stdout:   "{}",
			Stderr:   "restricted",
			ExitCode: 1,
		})

		resolver := NewResolver(script, "", 1080, 0, "", fileA+","+fileB, false)
		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to resolve Instagram URL with 2 cookie profiles") {
			t.Fatalf("expected aggregate error, got %v", err)
		}
		if !strings.Contains(err.Error(), "restricted") {
			t.Fatalf("expected root failure reason in aggregate error, got %v", err)
		}
	})

	t.Run("non-retryable typed error bypasses aggregate wrapping", func(t *testing.T) {
		dir := t.TempDir()
		fileA := filepath.Join(dir, "acc-a.txt")
		fileB := filepath.Join(dir, "acc-b.txt")
		if err := os.WriteFile(fileA, []byte("# a"), 0o600); err != nil {
			t.Fatalf("failed to write fileA: %v", err)
		}
		if err := os.WriteFile(fileB, []byte("# b"), 0o600); err != nil {
			t.Fatalf("failed to write fileB: %v", err)
		}

		payload := ytdlpOutput{
			Title: "HLS Only",
			Formats: []ytdlpFormat{
				{FormatID: "hls-250", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", Protocol: "m3u8_native", URL: "https://video-cdn.example/360.m3u8", Height: 360},
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})

		resolver := NewResolver(script, "", 1080, 0, "", fileA+","+fileB, false)
		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil {
			t.Fatal("expected typed error")
		}
		if strings.Contains(err.Error(), "failed to resolve Instagram URL with") {
			t.Fatalf("typed non-retryable error should not be wrapped, got %v", err)
		}
		var resolveErr *ResolveError
		if !errors.As(err, &resolveErr) {
			t.Fatalf("expected typed resolve error, got %T: %v", err, err)
		}
		if resolveErr.Code != ErrCodeIGHLSOnlyNotSupported {
			t.Fatalf("expected code %q, got %q", ErrCodeIGHLSOnlyNotSupported, resolveErr.Code)
		}
	})
}

func TestResolve_ResponseEdgeCases(t *testing.T) {
	t.Run("invalid json response", func(t *testing.T) {
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			Stdout:   "not-json",
			ExitCode: 0,
		})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil || !strings.Contains(err.Error(), "yt-dlp response is invalid") {
			t.Fatalf("expected invalid json error, got %v", err)
		}
	})

	t.Run("live content rejected", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:      "Live Post",
			IsLive:     true,
			LiveStatus: "is_live",
			Formats: []ytdlpFormat{
				progressiveFormat("18", 360, "https://video-cdn.example/18.mp4", 100),
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil || !strings.Contains(err.Error(), "live content is not supported") {
			t.Fatalf("expected live content error, got %v", err)
		}
	})

	t.Run("no downloadable format", func(t *testing.T) {
		payload := ytdlpOutput{
			Title: "No Progressive",
			Formats: []ytdlpFormat{
				{FormatID: "137", Ext: "webm", VideoCodec: "vp9", AudioCodec: "none", URL: "https://video-cdn.example/137.webm", Height: 1080},
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil || !strings.Contains(err.Error(), "no downloadable MP4 format is available") {
			t.Fatalf("expected no format error, got %v", err)
		}
	})

	t.Run("hls-only source returns typed unsupported error", func(t *testing.T) {
		payload := ytdlpOutput{
			Title: "HLS Only",
			Formats: []ytdlpFormat{
				{FormatID: "hls-audio-64000", Ext: "mp4", VideoCodec: "none", AudioCodec: "aac", Protocol: "m3u8_native", URL: "https://video-cdn.example/audio.m3u8"},
				{FormatID: "hls-250", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", Protocol: "m3u8_native", URL: "https://video-cdn.example/360.m3u8", Height: 360},
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		_, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err == nil {
			t.Fatal("expected hls-only error")
		}
		var resolveErr *ResolveError
		if !errors.As(err, &resolveErr) {
			t.Fatalf("expected typed resolve error, got %T: %v", err, err)
		}
		if resolveErr.Code != ErrCodeIGHLSOnlyNotSupported {
			t.Fatalf("expected code %q, got %q", ErrCodeIGHLSOnlyNotSupported, resolveErr.Code)
		}
		if !strings.Contains(strings.ToLower(resolveErr.Error()), "hls-only") {
			t.Fatalf("expected hls-only message, got %v", resolveErr)
		}
	})

	t.Run("accepts direct http mp4 when codec metadata is empty", func(t *testing.T) {
		payload := ytdlpOutput{
			Title: "Direct MP4",
			Formats: []ytdlpFormat{
				{FormatID: "http-256", Ext: "mp4", VideoCodec: "", AudioCodec: "", Protocol: "https", URL: "https://video-cdn.example/270.mp4", Height: 270, Filesize: 1000},
				{FormatID: "http-832", Ext: "mp4", VideoCodec: "", AudioCodec: "", Protocol: "https", URL: "https://video-cdn.example/360.mp4", Height: 360, Filesize: 2000},
				{FormatID: "hls-250", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", Protocol: "m3u8_native", URL: "https://video-cdn.example/360.m3u8", Height: 360},
				{FormatID: "dash-360", Ext: "mp4", VideoCodec: "", AudioCodec: "", Protocol: "https", URL: "https://video-cdn.example/360-dash.mp4", Height: 360, Filesize: 1500},
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		result, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err != nil {
			t.Fatalf("expected resolve success, got %v", err)
		}
		if len(result.Formats) != 2 {
			t.Fatalf("expected 2 formats from direct http mp4 fallback, got %d (%#v)", len(result.Formats), result.Formats)
		}
		if result.Formats[0].ID != "http-256" || result.Formats[1].ID != "http-832" {
			t.Fatalf("unexpected selected formats: %#v", result.Formats)
		}
	})

	t.Run("negative duration clamped to zero + thumbnail fallback", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:     "Duration Clamp",
			Duration:  -4.8,
			Thumbnail: "",
			Thumbnails: []thumbnail{
				{URL: ""},
				{URL: "https://img.example/first.jpg"},
				{URL: "https://img.example/final.jpg"},
			},
			Formats: []ytdlpFormat{
				progressiveFormat("22", 720, "https://video-cdn.example/22.mp4", 999),
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		result, err := resolver.Resolve(context.Background(), "https://instagram.com/reel/ABC123")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if result.DurationSeconds != 0 {
			t.Fatalf("expected clamped duration 0, got %d", result.DurationSeconds)
		}
		if result.Thumbnail != "https://img.example/final.jpg" {
			t.Fatalf("unexpected thumbnail fallback: %s", result.Thumbnail)
		}
	})
}

func TestSelectFormats_BestByHeightAndLimits(t *testing.T) {
	resolver := NewResolver("yt-dlp", "", 720, 500, "", "", true)

	formats := resolver.selectFormats([]ytdlpFormat{
		progressiveFormat("360-low", 360, "https://video-cdn.example/360-low.mp4", 300),
		progressiveFormat("360-high", 360, "https://video-cdn.example/360-high.mp4", 350),
		progressiveFormat("480-too-big", 480, "https://video-cdn.example/480.mp4", 600),
		{FormatID: "720-approx", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "mp4a", URL: "https://video-cdn.example/720.mp4", Height: 720, Filesize: 0, FilesizeApprox: 450},
		progressiveFormat("1080-over", 1080, "https://video-cdn.example/1080.mp4", 400),
	})

	if len(formats) != 2 {
		t.Fatalf("expected 2 formats, got %d (%#v)", len(formats), formats)
	}
	if formats[0].ID != "360-high" || formats[1].ID != "720-approx" {
		t.Fatalf("unexpected format ordering/content: %#v", formats)
	}
	if formats[1].Filesize != 450 {
		t.Fatalf("expected filesize from filesize_approx, got %d", formats[1].Filesize)
	}
}

func TestSelectFormats_AcceptsInstagramDirectHTTPMP4Fallback(t *testing.T) {
	resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)

	formats := resolver.selectFormats([]ytdlpFormat{
		{FormatID: "http-256", Ext: "mp4", URL: "https://video-cdn.example/270.mp4", Height: 270, Protocol: "https", Filesize: 1000},
		{FormatID: "http-832", Ext: "mp4", URL: "https://video-cdn.example/360.mp4", Height: 360, Protocol: "https", Filesize: 2000},
		{FormatID: "hls-audio-64000", Ext: "mp4", URL: "https://video-cdn.example/audio.m3u8", Height: 360, Protocol: "m3u8_native", FormatNote: "Audio"},
		{FormatID: "dash-360", Ext: "mp4", URL: "https://video-cdn.example/360-dash.mp4", Height: 360, Protocol: "https", FormatNote: "video only"},
	})

	if len(formats) != 2 {
		t.Fatalf("expected 2 direct http mp4 formats, got %d (%#v)", len(formats), formats)
	}
	if formats[0].ID != "http-256" || formats[1].ID != "http-832" {
		t.Fatalf("unexpected format IDs: %#v", formats)
	}
}

func TestIsDashVideoMP4(t *testing.T) {
	tests := []struct {
		name string
		in   ytdlpFormat
		want bool
	}{
		{
			name: "valid dash video mp4 with codec",
			in: ytdlpFormat{
				FormatID:   "dash-video-1",
				Ext:        "mp4",
				VideoCodec: "avc1.64001F",
				AudioCodec: "none",
				Protocol:   "https",
				Height:     720,
				URL:        "https://scontent.cdninstagram.com/video.mp4",
			},
			want: true,
		},
		{
			name: "valid dash video mp4 without codec metadata",
			in: ytdlpFormat{
				FormatID: "dash-video-2",
				Ext:      "mp4",
				Protocol: "https",
				Height:   360,
				URL:      "https://scontent.cdninstagram.com/video360.mp4",
			},
			want: true,
		},
		{
			name: "dash audio segment rejected",
			in: ytdlpFormat{
				FormatID:   "dash-audio-128000",
				Ext:        "mp4",
				VideoCodec: "none",
				AudioCodec: "mp4a.40.2",
				Protocol:   "https",
				Height:     0,
				URL:        "https://scontent.cdninstagram.com/audio.mp4",
			},
			want: false,
		},
		{
			name: "dash format with m3u8 protocol rejected",
			in: ytdlpFormat{
				FormatID:   "dash-video-3",
				Ext:        "mp4",
				VideoCodec: "avc1",
				AudioCodec: "none",
				Protocol:   "m3u8_native",
				Height:     720,
				URL:        "https://scontent.cdninstagram.com/video.m3u8",
			},
			want: false,
		},
		{
			name: "non-dash format id",
			in: ytdlpFormat{
				FormatID:   "http-832",
				Ext:        "mp4",
				VideoCodec: "avc1",
				AudioCodec: "none",
				Protocol:   "https",
				Height:     720,
				URL:        "https://scontent.cdninstagram.com/video.mp4",
			},
			want: false,
		},
		{
			name: "dash format with non-mp4 ext",
			in: ytdlpFormat{
				FormatID:   "dash-video-1",
				Ext:        "webm",
				VideoCodec: "vp9",
				AudioCodec: "none",
				Protocol:   "https",
				Height:     720,
				URL:        "https://scontent.cdninstagram.com/video.webm",
			},
			want: false,
		},
		{
			name: "dash format with missing url",
			in: ytdlpFormat{
				FormatID:   "dash-video-1",
				Ext:        "mp4",
				VideoCodec: "avc1",
				AudioCodec: "none",
				Protocol:   "https",
				Height:     720,
			},
			want: false,
		},
		{
			name: "dash audio-only by format note",
			in: ytdlpFormat{
				FormatID:   "dash-audio-64",
				Ext:        "mp4",
				Protocol:   "https",
				Height:     360,
				URL:        "https://scontent.cdninstagram.com/audio.mp4",
				FormatNote: "Audio",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isDashVideoMP4(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected result, got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestSelectFormats_AcceptsDashVideoMP4(t *testing.T) {
	resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)

	formats := resolver.selectFormats([]ytdlpFormat{
		{FormatID: "dash-video-1", Ext: "mp4", VideoCodec: "avc1.64001F", AudioCodec: "none", Protocol: "https", URL: "https://scontent.cdninstagram.com/360.mp4", Height: 360, Filesize: 1500},
		{FormatID: "dash-video-2", Ext: "mp4", VideoCodec: "avc1.64001F", AudioCodec: "none", Protocol: "https", URL: "https://scontent.cdninstagram.com/720.mp4", Height: 720, Filesize: 3000},
		{FormatID: "dash-audio-128000", Ext: "mp4", VideoCodec: "none", AudioCodec: "mp4a.40.2", Protocol: "https", URL: "https://scontent.cdninstagram.com/audio.mp4", Height: 0},
		{FormatID: "hls-250", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", Protocol: "m3u8_native", URL: "https://scontent.cdninstagram.com/360.m3u8", Height: 360},
	})

	if len(formats) != 2 {
		t.Fatalf("expected 2 dash video formats, got %d (%#v)", len(formats), formats)
	}
	if formats[0].ID != "dash-video-1" || formats[1].ID != "dash-video-2" {
		t.Fatalf("unexpected format IDs: %#v", formats)
	}
	if formats[0].Quality != "360p" || formats[1].Quality != "720p" {
		t.Fatalf("unexpected qualities: %s, %s", formats[0].Quality, formats[1].Quality)
	}
}

func TestIsLikelyInstagramDirectMP4(t *testing.T) {
	tests := []struct {
		name string
		in   ytdlpFormat
		want bool
	}{
		{
			name: "valid direct http mp4",
			in: ytdlpFormat{
				FormatID: "http-832",
				Ext:      "mp4",
				Protocol: "https",
				Height:   360,
				URL:      "https://instagram.cdn/video.mp4",
			},
			want: true,
		},
		{
			name: "dash format id",
			in: ytdlpFormat{
				FormatID: "dash-360",
				Ext:      "mp4",
				Protocol: "https",
				Height:   360,
				URL:      "https://instagram.cdn/dash.mp4",
			},
			want: false,
		},
		{
			name: "known codec metadata should not use fallback",
			in: ytdlpFormat{
				FormatID:   "0",
				Ext:        "mp4",
				VideoCodec: "avc1",
				AudioCodec: "none",
				Protocol:   "https",
				Height:     360,
				URL:        "https://instagram.cdn/video-only.mp4",
			},
			want: false,
		},
		{
			name: "audio only note",
			in: ytdlpFormat{
				FormatID:   "http-64",
				Ext:        "mp4",
				Protocol:   "https",
				Height:     360,
				URL:        "https://instagram.cdn/audio.mp4",
				FormatNote: "Audio",
			},
			want: false,
		},
		{
			name: "video only note",
			in: ytdlpFormat{
				FormatID:   "http-500",
				Ext:        "mp4",
				Protocol:   "https",
				Height:     360,
				URL:        "https://instagram.cdn/video-only.mp4",
				FormatNote: "video only",
			},
			want: false,
		},
		{
			name: "missing url",
			in: ytdlpFormat{
				FormatID: "http-832",
				Ext:      "mp4",
				Protocol: "https",
				Height:   360,
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isLikelyInstagramDirectMP4(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected result, got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestIsLikelyHLSOnlySource(t *testing.T) {
	tests := []struct {
		name string
		raw  []ytdlpFormat
		want bool
	}{
		{
			name: "pure hls video source",
			raw: []ytdlpFormat{
				{FormatID: "hls-audio-64000", Protocol: "m3u8_native", VideoCodec: "none", AudioCodec: "aac"},
				{FormatID: "hls-250", Protocol: "m3u8_native", VideoCodec: "avc1", AudioCodec: "none", Height: 360},
			},
			want: true,
		},
		{
			name: "mixed hls and direct video",
			raw: []ytdlpFormat{
				{FormatID: "hls-250", Protocol: "m3u8_native", VideoCodec: "avc1", AudioCodec: "none", Height: 360},
				{FormatID: "http-832", Protocol: "https", Height: 360, URL: "https://video-cdn.example/360.mp4"},
			},
			want: false,
		},
		{
			name: "no hls video",
			raw: []ytdlpFormat{
				{FormatID: "http-832", Protocol: "https", Height: 360, URL: "https://video-cdn.example/360.mp4"},
			},
			want: false,
		},
		{
			name: "hls audio only",
			raw: []ytdlpFormat{
				{FormatID: "hls-audio-64000", Protocol: "m3u8_native", VideoCodec: "none", AudioCodec: "aac"},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isLikelyHLSOnlySource(tc.raw)
			if got != tc.want {
				t.Fatalf("unexpected result, got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestIsProgressiveMP4(t *testing.T) {
	tests := []struct {
		name string
		in   ytdlpFormat
		want bool
	}{
		{
			name: "valid progressive",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: "mp4a"},
			want: true,
		},
		{
			name: "invalid ext",
			in:   ytdlpFormat{Ext: "webm", VideoCodec: "vp9", AudioCodec: "opus"},
			want: false,
		},
		{
			name: "missing video",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "", AudioCodec: "mp4a"},
			want: false,
		},
		{
			name: "video none",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "none", AudioCodec: "mp4a"},
			want: false,
		},
		{
			name: "missing audio",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: ""},
			want: false,
		},
		{
			name: "audio none",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isProgressiveMP4(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected result, got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestChooseThumbnail(t *testing.T) {
	tests := []struct {
		name string
		in   ytdlpOutput
		want string
	}{
		{
			name: "prefer direct thumbnail",
			in: ytdlpOutput{
				Thumbnail:  "https://img.example/direct.jpg",
				Thumbnails: []thumbnail{{URL: "https://img.example/fallback.jpg"}},
			},
			want: "https://img.example/direct.jpg",
		},
		{
			name: "fallback from thumbnails reverse order",
			in: ytdlpOutput{
				Thumbnail:  "",
				Thumbnails: []thumbnail{{URL: ""}, {URL: "https://img.example/first.jpg"}, {URL: "https://img.example/final.jpg"}},
			},
			want: "https://img.example/final.jpg",
		},
		{
			name: "no thumbnail available",
			in:   ytdlpOutput{},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := chooseThumbnail(tc.in); got != tc.want {
				t.Fatalf("unexpected thumbnail, got=%q want=%q", got, tc.want)
			}
		})
	}
}
