package youtube

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
	Stdout            string
	Stderr            string
	ExitCode          int
	ExpectedJS        string
	ExpectedHeader    string
	ExpectedUserAgent string
	ExpectedURL       string
}

func makeFakeYTDLPScript(t *testing.T, opts fakeYTDLPScriptOptions) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

expected_js=%q
expected_header=%q
expected_ua=%q
expected_url=%q
stderr_text=%q
exit_code=%d

stdout_payload=$(cat <<'__JSON__'
%s
__JSON__
)

captured_js=""
captured_header=""
captured_ua=""
captured_url=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --js-runtimes)
      captured_js="$2"
      shift 2
      ;;
    --add-header)
      captured_header="$2"
      shift 2
      ;;
    --user-agent)
      captured_ua="$2"
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

if [[ -n "$expected_js" && "$captured_js" != "$expected_js" ]]; then
  echo "unexpected --js-runtimes value: $captured_js" >&2
  exit 91
fi

if [[ -n "$expected_header" && "$captured_header" != "$expected_header" ]]; then
  echo "unexpected --add-header value: $captured_header" >&2
  exit 92
fi

if [[ -n "$expected_ua" && "$captured_ua" != "$expected_ua" ]]; then
  echo "unexpected --user-agent value: $captured_ua" >&2
  exit 93
fi

if [[ -n "$expected_url" && "$captured_url" != "$expected_url" ]]; then
  echo "unexpected target URL: $captured_url" >&2
  exit 94
fi

if [[ "$exit_code" -ne 0 ]]; then
  if [[ -n "$stderr_text" ]]; then
    echo "$stderr_text" >&2
  fi
  exit "$exit_code"
fi

printf '%%s' "$stdout_payload"
`,
		opts.ExpectedJS,
		opts.ExpectedHeader,
		opts.ExpectedUserAgent,
		opts.ExpectedURL,
		opts.Stderr,
		opts.ExitCode,
		opts.Stdout,
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

func progressiveFormat(id string, height int, url string, filesize int64, filesizeApprox int64) ytdlpFormat {
	return ytdlpFormat{
		FormatID:       id,
		Ext:            "mp4",
		VideoCodec:     "avc1",
		AudioCodec:     "mp4a",
		URL:            url,
		Height:         height,
		Filesize:       filesize,
		FilesizeApprox: filesizeApprox,
	}
}

func heatmapBin(start float64, end float64, value float64) ytdlpHeatmapPoint {
	return ytdlpHeatmapPoint{StartTime: start, EndTime: end, Value: value}
}

func TestNewResolver_DefaultsAndTrim(t *testing.T) {
	resolver := NewResolver("yt-dlp", "  node  ", 0, 0, 999)

	if resolver.ytdlpBinary != "yt-dlp" {
		t.Fatalf("unexpected ytdlpBinary: %s", resolver.ytdlpBinary)
	}
	if resolver.ytdlpJSRuntimes != "node" {
		t.Fatalf("expected trimmed js runtimes, got %q", resolver.ytdlpJSRuntimes)
	}
	if resolver.maxDurationSecs != 3600 {
		t.Fatalf("expected default maxDurationSecs=3600, got %d", resolver.maxDurationSecs)
	}
	if resolver.maxQuality != 1080 {
		t.Fatalf("expected default maxQuality=1080, got %d", resolver.maxQuality)
	}
	if resolver.maxFileSizeBytes != 999 {
		t.Fatalf("unexpected maxFileSizeBytes: %d", resolver.maxFileSizeBytes)
	}
}

func TestParseInput(t *testing.T) {
	t.Run("plain URL", func(t *testing.T) {
		plainURL, plainHeaders, plainUA, err := ParseInput("https://www.youtube.com/watch?v=abc123")
		if err != nil {
			t.Fatalf("plain URL should be parsed, got err: %v", err)
		}
		if plainURL != "https://www.youtube.com/watch?v=abc123" {
			t.Fatalf("unexpected plain URL: %s", plainURL)
		}
		if len(plainHeaders) != 0 || plainUA != "" {
			t.Fatalf("plain URL should not return headers/UA")
		}
	})

	t.Run("curl input with header and user-agent", func(t *testing.T) {
		curlInput := `curl "https://www.youtube.com/watch?v=abc123" -H "Referer: https://www.youtube.com/" -A "Mozilla/5.0"`
		curlURL, curlHeaders, curlUA, err := ParseInput(curlInput)
		if err != nil {
			t.Fatalf("curl input should be parsed, got err: %v", err)
		}
		if curlURL != "https://www.youtube.com/watch?v=abc123" {
			t.Fatalf("unexpected curl URL: %s", curlURL)
		}
		if curlHeaders["Referer"] != "https://www.youtube.com/" {
			t.Fatalf("missing parsed referer header")
		}
		if curlUA != "Mozilla/5.0" {
			t.Fatalf("unexpected parsed UA: %s", curlUA)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, _, _, err := ParseInput("   ")
		if err == nil || !strings.Contains(err.Error(), "url is required") {
			t.Fatalf("expected url is required error, got %v", err)
		}
	})

	t.Run("malformed curl input", func(t *testing.T) {
		_, _, _, err := ParseInput(`curl "https://www.youtube.com/watch?v=abc123`)
		if err == nil || !strings.Contains(err.Error(), "failed to parse cURL input") {
			t.Fatalf("expected parse cURL error, got %v", err)
		}
	})

	t.Run("curl with no URL", func(t *testing.T) {
		_, _, _, err := ParseInput(`curl -H "Referer: https://www.youtube.com/"`)
		if err == nil || !strings.Contains(err.Error(), "failed to parse URL from cURL input") {
			t.Fatalf("expected parse URL from cURL error, got %v", err)
		}
	})

	t.Run("curl ignores malformed header and unknown flags", func(t *testing.T) {
		input := `curl --compressed -H "invalidheader" -H "X-Token: abc" https://www.youtube.com/watch?v=abc123`
		urlValue, headers, ua, err := ParseInput(input)
		if err != nil {
			t.Fatalf("expected parse success, got err: %v", err)
		}
		if urlValue != "https://www.youtube.com/watch?v=abc123" {
			t.Fatalf("unexpected URL: %s", urlValue)
		}
		if headers["X-Token"] != "abc" {
			t.Fatalf("expected valid header parsed, got %#v", headers)
		}
		if _, ok := headers["invalidheader"]; ok {
			t.Fatalf("malformed header should not be parsed")
		}
		if ua != "" {
			t.Fatalf("unexpected user-agent: %q", ua)
		}
	})
}

func TestValidateYouTubeURL(t *testing.T) {
	validURLs := []string{
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/abc123",
		"https://www.youtube.com/shorts/abc123",
		"https://m.youtube.com/watch?v=abc123",
		"https://music.youtube.com/watch?v=abc123",
	}

	for _, input := range validURLs {
		if err := validateYouTubeURL(input); err != nil {
			t.Fatalf("expected valid URL (%s), got error: %v", input, err)
		}
	}

	tests := []struct {
		name string
		url  string
		err  string
	}{
		{
			name: "invalid parse",
			url:  "://not-a-url",
			err:  "invalid URL",
		},
		{
			name: "unsupported scheme",
			url:  "ftp://www.youtube.com/watch?v=abc123",
			err:  "URL must start with http or https",
		},
		{
			name: "short URL missing id",
			url:  "https://youtu.be/",
			err:  "invalid YouTube short URL",
		},
		{
			name: "watch missing v",
			url:  "https://www.youtube.com/watch",
			err:  "missing video id",
		},
		{
			name: "shorts missing id (currently treated as unsupported path)",
			url:  "https://www.youtube.com/shorts/",
			err:  "unsupported YouTube URL path",
		},
		{
			name: "unsupported path",
			url:  "https://www.youtube.com/channel/UC12345",
			err:  "unsupported YouTube URL path",
		},
		{
			name: "non-youtube host",
			url:  "https://example.com/watch?v=abc123",
			err:  "URL must be a YouTube link",
		},
		{
			name: "playlist not supported",
			url:  "https://www.youtube.com/watch?v=abc123&list=PL12345",
			err:  "playlist URL is not supported",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateYouTubeURL(tc.url)
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("expected error containing %q, got %v", tc.err, err)
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
			name: "valid progressive mp4",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: "mp4a"},
			want: true,
		},
		{
			name: "non-mp4 extension",
			in:   ytdlpFormat{Ext: "webm", VideoCodec: "vp9", AudioCodec: "opus"},
			want: false,
		},
		{
			name: "missing video codec",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "", AudioCodec: "mp4a"},
			want: false,
		},
		{
			name: "video codec none",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "none", AudioCodec: "mp4a"},
			want: false,
		},
		{
			name: "missing audio codec",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: ""},
			want: false,
		},
		{
			name: "audio codec none",
			in:   ytdlpFormat{Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isProgressiveMP4(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected isProgressiveMP4 result, got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestSelectFormats(t *testing.T) {
	resolver := NewResolver("/usr/local/bin/yt-dlp", "node", 60, 1080, 1000)

	raw := []ytdlpFormat{
		progressiveFormat("18", 360, "https://cdn.example/18", 100, 0),
		progressiveFormat("18-alt", 360, "https://cdn.example/18-alt", 120, 0),
		{FormatID: "22-no-audio", Ext: "mp4", VideoCodec: "avc1", AudioCodec: "none", URL: "https://cdn.example/22", Height: 720, Filesize: 200},
		progressiveFormat("399", 1080, "https://cdn.example/399", 2000, 0),
		{FormatID: "400", Ext: "webm", VideoCodec: "vp9", AudioCodec: "opus", URL: "https://cdn.example/400", Height: 720, Filesize: 200},
	}

	out := resolver.selectFormats(raw)
	if len(out) != 2 {
		t.Fatalf("expected 2 formats (1 mp4 + 1 mp3 option), got %d", len(out))
	}

	if out[0].ID != "18-alt" || out[0].Quality != "360p" {
		t.Fatalf("unexpected selected MP4 format: %+v", out[0])
	}
	if out[1].Type != "mp3" || out[1].ID != "mp3-128" {
		t.Fatalf("expected mp3 synthetic option, got %+v", out[1])
	}
}

func TestSelectFormats_FiltersOrderingAndApproxFilesize(t *testing.T) {
	resolver := NewResolver("yt-dlp", "", 60, 720, 500)

	raw := []ytdlpFormat{
		progressiveFormat("bad-height-zero", 0, "https://cdn.example/0", 100, 0),
		progressiveFormat("bad-height-too-high", 1080, "https://cdn.example/1080", 100, 0),
		progressiveFormat("bad-empty-url", 480, "", 120, 0),
		progressiveFormat("good-720-a", 720, "https://cdn.example/720a", 0, 300),
		progressiveFormat("good-720-b-smaller", 720, "https://cdn.example/720b", 0, 250),
		progressiveFormat("too-big", 360, "https://cdn.example/too-big", 700, 0),
		progressiveFormat("good-360", 360, "https://cdn.example/360", 200, 0),
	}

	out := resolver.selectFormats(raw)
	if len(out) != 3 {
		t.Fatalf("expected 2 mp4 + 1 mp3 formats, got %d (%+v)", len(out), out)
	}

	if out[0].ID != "good-360" || out[0].Quality != "360p" {
		t.Fatalf("unexpected first format: %+v", out[0])
	}
	if out[1].ID != "good-720-a" || out[1].Filesize != 300 {
		t.Fatalf("unexpected second format (should pick larger approx size): %+v", out[1])
	}
	if out[2].Type != "mp3" {
		t.Fatalf("expected trailing mp3 synthetic option, got %+v", out[2])
	}
}

func TestChooseThumbnail(t *testing.T) {
	t.Run("prefer primary thumbnail", func(t *testing.T) {
		payload := ytdlpOutput{
			Thumbnail: "https://cdn.example/primary.jpg",
			Thumbnails: []thumbnail{
				{URL: "https://cdn.example/fallback1.jpg"},
				{URL: "https://cdn.example/fallback2.jpg"},
			},
		}
		if got := chooseThumbnail(payload); got != "https://cdn.example/primary.jpg" {
			t.Fatalf("expected primary thumbnail, got %s", got)
		}
	})

	t.Run("fallback to last non-empty thumbnail", func(t *testing.T) {
		payload := ytdlpOutput{
			Thumbnails: []thumbnail{
				{URL: ""},
				{URL: "https://cdn.example/fallback1.jpg"},
				{URL: "https://cdn.example/fallback2.jpg"},
			},
		}
		if got := chooseThumbnail(payload); got != "https://cdn.example/fallback2.jpg" {
			t.Fatalf("expected last fallback thumbnail, got %s", got)
		}
	})

	t.Run("empty when none available", func(t *testing.T) {
		payload := ytdlpOutput{}
		if got := chooseThumbnail(payload); got != "" {
			t.Fatalf("expected empty thumbnail, got %q", got)
		}
	})
}

func TestResolve(t *testing.T) {
	t.Run("requires ytdlp binary", func(t *testing.T) {
		resolver := NewResolver("", "node", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "yt-dlp binary is not configured") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("parse input error", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "   ")
		if err == nil || !strings.Contains(err.Error(), "url is required") {
			t.Fatalf("expected url required error, got %v", err)
		}
	})

	t.Run("validate URL error", func(t *testing.T) {
		resolver := NewResolver("yt-dlp", "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://example.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "URL must be a YouTube link") {
			t.Fatalf("expected validate error, got %v", err)
		}
	})

	t.Run("requested format error is remapped", func(t *testing.T) {
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			ExitCode: 23,
			Stderr:   "Requested format is not available",
		})
		resolver := NewResolver(script, "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "update yt-dlp to the latest version") {
			t.Fatalf("expected remapped requested-format error, got %v", err)
		}
	})

	t.Run("generic command stderr error", func(t *testing.T) {
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			ExitCode: 10,
			Stderr:   "network timeout",
		})
		resolver := NewResolver(script, "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "failed to resolve URL: network timeout") {
			t.Fatalf("expected wrapped generic error, got %v", err)
		}
	})

	t.Run("command failure without stderr uses run error", func(t *testing.T) {
		missingBinary := filepath.Join(t.TempDir(), "missing-yt-dlp")
		resolver := NewResolver(missingBinary, "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "failed to resolve URL:") {
			t.Fatalf("expected wrapped run error, got %v", err)
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: "not-json"})
		resolver := NewResolver(script, "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "yt-dlp response is invalid") {
			t.Fatalf("expected invalid JSON error, got %v", err)
		}
	})

	t.Run("live content rejected", func(t *testing.T) {
		cases := []struct {
			name    string
			payload ytdlpOutput
		}{
			{
				name: "is_live true",
				payload: ytdlpOutput{
					Title:    "Live",
					Duration: 100,
					IsLive:   true,
					Formats:  []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 100, 0)},
				},
			},
			{
				name: "live_status is_live",
				payload: ytdlpOutput{
					Title:      "Live",
					Duration:   100,
					LiveStatus: "is_live",
					Formats:    []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 100, 0)},
				},
			},
			{
				name: "live_status is_upcoming",
				payload: ytdlpOutput{
					Title:      "Live",
					Duration:   100,
					LiveStatus: "is_upcoming",
					Formats:    []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 100, 0)},
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, tc.payload)})
				resolver := NewResolver(script, "", 60, 1080, 0)
				_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
				if err == nil || !strings.Contains(err.Error(), "live content is not supported") {
					t.Fatalf("expected live-content error, got %v", err)
				}
			})
		}
	})

	t.Run("duration unavailable", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:    "No Duration",
			Duration: 0,
			Formats:  []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 100, 0)},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 60, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "video duration is unavailable") {
			t.Fatalf("expected duration unavailable error, got %v", err)
		}
	})

	t.Run("duration exceeds max", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:    "Too Long",
			Duration: 121,
			Formats:  []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 100, 0)},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 2, 1080, 0)
		_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err == nil || !strings.Contains(err.Error(), "video exceeds maximum duration") {
			t.Fatalf("expected max duration error, got %v", err)
		}
	})

	t.Run("successful resolve with plain URL", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:    "Sample Video",
			Duration: 61.6,
			Thumbnails: []thumbnail{
				{URL: ""},
				{URL: "https://img.example/thumb-1.jpg"},
				{URL: "https://img.example/thumb-2.jpg"},
			},
			Formats: []ytdlpFormat{
				progressiveFormat("18", 360, "https://cdn.example/18", 220, 0),
				progressiveFormat("18-small", 360, "https://cdn.example/18-small", 180, 0),
				progressiveFormat("22-big", 720, "https://cdn.example/22-big", 900, 0),
				progressiveFormat("22-ok", 720, "https://cdn.example/22-ok", 0, 300),
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			Stdout:      mustJSON(t, payload),
			ExpectedURL: "https://www.youtube.com/watch?v=abc123",
		})
		resolver := NewResolver(script, "", 60, 1080, 500)

		result, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err != nil {
			t.Fatalf("expected resolve success, got err: %v", err)
		}

		if result.Title != "Sample Video" {
			t.Fatalf("unexpected title: %s", result.Title)
		}
		if result.DurationSeconds != 62 {
			t.Fatalf("expected rounded duration 62, got %d", result.DurationSeconds)
		}
		if result.Thumbnail != "https://img.example/thumb-2.jpg" {
			t.Fatalf("unexpected thumbnail: %s", result.Thumbnail)
		}
		if len(result.Formats) != 3 {
			t.Fatalf("expected 2 mp4 + 1 mp3 options, got %d (%+v)", len(result.Formats), result.Formats)
		}
		if result.Formats[0].ID != "18" || result.Formats[0].Quality != "360p" {
			t.Fatalf("unexpected first format: %+v", result.Formats[0])
		}
		if result.Formats[1].ID != "22-ok" || result.Formats[1].Filesize != 300 {
			t.Fatalf("unexpected second format (expected fileszie_approx fallback): %+v", result.Formats[1])
		}
		if result.Formats[2].Type != "mp3" {
			t.Fatalf("expected synthetic mp3 option at tail, got %+v", result.Formats[2])
		}
		if result.HeatmapMeta.AlgorithmVersion == "" {
			t.Fatalf("expected algorithm version in heatmap meta")
		}
		if result.HeatmapMeta.Available {
			t.Fatalf("expected no heatmap data for payload without heatmap")
		}
		if result.HeatmapMeta.Bins != 0 {
			t.Fatalf("expected bins=0 for payload without heatmap, got %d", result.HeatmapMeta.Bins)
		}
		if len(result.Heatmap) != 0 {
			t.Fatalf("expected empty heatmap payload, got %+v", result.Heatmap)
		}
		if len(result.KeyMoments) != 0 {
			t.Fatalf("expected empty key moments, got %+v", result.KeyMoments)
		}
	})

	t.Run("successful resolve includes heatmap and key moments", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:    "Heatmap Video",
			Duration: 100,
			Formats: []ytdlpFormat{
				progressiveFormat("18", 360, "https://cdn.example/18", 200, 0),
			},
			Heatmap: []ytdlpHeatmapPoint{
				heatmapBin(0, 10, 1.0),
				heatmapBin(10, 20, 0.35),
				heatmapBin(20, 30, 0.22),
				heatmapBin(30, 40, 0.20),
				heatmapBin(40, 50, 0.30),
				heatmapBin(50, 60, 0.95),
				heatmapBin(60, 70, 0.25),
				heatmapBin(70, 80, 0.88),
				heatmapBin(80, 90, 0.30),
				heatmapBin(90, 100, 0.20),
			},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{Stdout: mustJSON(t, payload)})
		resolver := NewResolver(script, "", 60, 1080, 0)

		result, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
		if err != nil {
			t.Fatalf("expected resolve success, got err: %v", err)
		}
		if !result.HeatmapMeta.Available {
			t.Fatalf("expected heatmap available=true")
		}
		if result.HeatmapMeta.Bins != 10 {
			t.Fatalf("expected heatmap bins=10, got %d", result.HeatmapMeta.Bins)
		}
		if result.HeatmapMeta.AlgorithmVersion == "" {
			t.Fatalf("expected algorithm version")
		}
		if len(result.Heatmap) != 10 {
			t.Fatalf("expected normalized heatmap points len=10, got %d", len(result.Heatmap))
		}
		if len(result.KeyMoments) == 0 {
			t.Fatalf("expected non-empty key moments")
		}
		if got := result.KeyMoments; len(got) != 2 || got[0] != 55 || got[1] != 75 {
			t.Fatalf("unexpected key moments: %+v", got)
		}
	})

	t.Run("successful resolve with curl input + js runtime + header + user agent", func(t *testing.T) {
		payload := ytdlpOutput{
			Title:     "Curl Video",
			Duration:  30,
			Thumbnail: "https://img.example/curl-thumb.jpg",
			Formats:   []ytdlpFormat{progressiveFormat("18", 360, "https://cdn.example/18", 150, 0)},
		}
		script := makeFakeYTDLPScript(t, fakeYTDLPScriptOptions{
			Stdout:            mustJSON(t, payload),
			ExpectedJS:        "node",
			ExpectedHeader:    "Referer: https://www.youtube.com/",
			ExpectedUserAgent: "Mozilla/5.0 ResolverTest",
			ExpectedURL:       "https://www.youtube.com/watch?v=abc123",
		})

		resolver := NewResolver(script, "node", 60, 1080, 0)
		input := `curl "https://www.youtube.com/watch?v=abc123" -H "Referer: https://www.youtube.com/" -A "Mozilla/5.0 ResolverTest"`
		result, err := resolver.Resolve(context.Background(), input)
		if err != nil {
			t.Fatalf("expected resolve success, got err: %v", err)
		}
		if result.Title != "Curl Video" {
			t.Fatalf("unexpected title: %s", result.Title)
		}
	})
}
