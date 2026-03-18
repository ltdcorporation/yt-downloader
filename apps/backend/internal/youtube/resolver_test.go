package youtube

import (
	"context"
	"strings"
	"testing"
)

func TestParseCurlOrURL(t *testing.T) {
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
}

func TestValidateYouTubeURL(t *testing.T) {
	validURLs := []string{
		"https://www.youtube.com/watch?v=abc123",
		"https://youtu.be/abc123",
		"https://www.youtube.com/shorts/abc123",
		"https://m.youtube.com/watch?v=abc123",
	}

	for _, input := range validURLs {
		if err := validateYouTubeURL(input); err != nil {
			t.Fatalf("expected valid URL (%s), got error: %v", input, err)
		}
	}

	invalidURLs := []string{
		"https://www.youtube.com/watch?v=abc123&list=PL12345",
		"https://www.youtube.com/channel/UC12345",
		"https://example.com/watch?v=abc123",
		"ftp://www.youtube.com/watch?v=abc123",
		"https://youtu.be/",
	}

	for _, input := range invalidURLs {
		if err := validateYouTubeURL(input); err == nil {
			t.Fatalf("expected invalid URL (%s), got nil error", input)
		}
	}
}

func TestSelectFormats(t *testing.T) {
	resolver := NewResolver("/usr/local/bin/yt-dlp", "node", 60, 1080, 1000)

	raw := []ytdlpFormat{
		{
			FormatID:   "18",
			Ext:        "mp4",
			VideoCodec: "avc1",
			AudioCodec: "mp4a",
			URL:        "https://cdn.example/18",
			Height:     360,
			Filesize:   100,
		},
		{
			FormatID:   "18-alt",
			Ext:        "mp4",
			VideoCodec: "avc1",
			AudioCodec: "mp4a",
			URL:        "https://cdn.example/18-alt",
			Height:     360,
			Filesize:   120,
		},
		{
			FormatID:   "22",
			Ext:        "mp4",
			VideoCodec: "avc1",
			AudioCodec: "none",
			URL:        "https://cdn.example/22",
			Height:     720,
			Filesize:   200,
		},
		{
			FormatID:   "399",
			Ext:        "mp4",
			VideoCodec: "avc1",
			AudioCodec: "mp4a",
			URL:        "https://cdn.example/399",
			Height:     1080,
			Filesize:   2000,
		},
		{
			FormatID:   "400",
			Ext:        "webm",
			VideoCodec: "vp9",
			AudioCodec: "opus",
			URL:        "https://cdn.example/400",
			Height:     720,
			Filesize:   200,
		},
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

func TestResolveRequiresYTDLPBinary(t *testing.T) {
	resolver := NewResolver("", "node", 60, 1080, 0)
	_, err := resolver.Resolve(context.Background(), "https://www.youtube.com/watch?v=abc123")
	if err == nil {
		t.Fatal("expected error when yt-dlp binary is missing")
	}
	if !strings.Contains(err.Error(), "yt-dlp binary is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
