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

func TestNewResolver_DefaultAndExplicitQuality(t *testing.T) {
	fallback := NewResolver("yt-dlp", "  node  ", 0, 777, "  /tmp/x-cookies  ", "  /tmp/a.txt,/tmp/b.txt  ", true)
	if fallback.maxQuality != 1080 {
		t.Fatalf("expected fallback maxQuality=1080, got %d", fallback.maxQuality)
	}
	if fallback.ytdlpJSRuntimes != "node" {
		t.Fatalf("expected trimmed js runtime, got %q", fallback.ytdlpJSRuntimes)
	}
	if fallback.cookiesDir != "/tmp/x-cookies" {
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
		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/1")
		if err == nil || !strings.Contains(err.Error(), "yt-dlp binary is not configured") {
			t.Fatalf("expected ytdlp binary error, got %v", err)
		}
	})

	t.Run("parse input error", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)
		_, err := resolver.Resolve(context.Background(), `curl "https://x.com/user/status/1`)
		if err == nil || !strings.Contains(err.Error(), "failed to parse cURL input") {
			t.Fatalf("expected parse input error, got %v", err)
		}
	})

	t.Run("invalid host", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 1080, 0, "", "", true)
		_, err := resolver.Resolve(context.Background(), "https://example.com/user/status/1")
		if err == nil || !strings.Contains(err.Error(), "URL must be an X/Twitter link") {
			t.Fatalf("expected x host validation error, got %v", err)
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
		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/123")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), `cookie profile "acc-single" failed: geo blocked`) {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(err.Error(), "failed to resolve X URL with") {
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
			Stderr:   "sensitive media locked",
			ExitCode: 1,
		})

		resolver := NewResolver(script, "", 1080, 0, "", fileA+","+fileB, false)
		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/123")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to resolve X URL with 2 cookie profiles") {
			t.Fatalf("expected aggregate error, got %v", err)
		}
		if !strings.Contains(err.Error(), "sensitive media locked") {
			t.Fatalf("expected root failure reason in aggregate error, got %v", err)
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

		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/1")
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

		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/1")
		if err == nil || !strings.Contains(err.Error(), "live content is not supported") {
			t.Fatalf("expected live content error, got %v", err)
		}
	})

	t.Run("no downloadable format", func(t *testing.T) {
		payload := ytdlpOutput{
			Title: "No Progressive",
			Formats: []ytdlpFormat{
				{FormatID: "137", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", URL: "https://video-cdn.example/137.mp4", Height: 1080},
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 1080, 0, "", "", true)

		_, err := resolver.Resolve(context.Background(), "https://x.com/user/status/1")
		if err == nil || !strings.Contains(err.Error(), "no downloadable MP4 format is available") {
			t.Fatalf("expected no format error, got %v", err)
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

		result, err := resolver.Resolve(context.Background(), "https://x.com/user/status/1")
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
