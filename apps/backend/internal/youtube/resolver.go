package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	shlex "github.com/google/shlex"
)

type Resolver struct {
	ytdlpBinary      string
	ytdlpJSRuntimes  string
	maxDurationSecs  int
	maxQuality       int
	maxFileSizeBytes int64
}

type Format struct {
	ID          string `json:"id"`
	Quality     string `json:"quality"`
	Container   string `json:"container"`
	Type        string `json:"type"`
	Progressive bool   `json:"progressive"`
	URL         string `json:"url,omitempty"`
	Filesize    int64  `json:"filesize,omitempty"`
}

type ResolveResult struct {
	Title           string   `json:"title"`
	Thumbnail       string   `json:"thumbnail"`
	DurationSeconds int      `json:"duration_seconds"`
	Formats         []Format `json:"formats"`
}

type ytdlpOutput struct {
	Title      string        `json:"title"`
	Thumbnail  string        `json:"thumbnail"`
	Thumbnails []thumbnail   `json:"thumbnails"`
	Duration   float64       `json:"duration"`
	IsLive     bool          `json:"is_live"`
	LiveStatus string        `json:"live_status"`
	Formats    []ytdlpFormat `json:"formats"`
}

type thumbnail struct {
	URL string `json:"url"`
}

type ytdlpFormat struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	VideoCodec     string  `json:"vcodec"`
	AudioCodec     string  `json:"acodec"`
	URL            string  `json:"url"`
	Height         int     `json:"height"`
	FormatNote     string  `json:"format_note"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
}

func NewResolver(ytdlpBinary string, ytdlpJSRuntimes string, maxDurationMinutes int, maxQuality int, maxFileSizeBytes int64) *Resolver {
	maxSeconds := 60 * maxDurationMinutes
	if maxSeconds <= 0 {
		maxSeconds = 3600
	}
	if maxQuality <= 0 {
		maxQuality = 1080
	}

	return &Resolver{
		ytdlpBinary:      ytdlpBinary,
		ytdlpJSRuntimes:  strings.TrimSpace(ytdlpJSRuntimes),
		maxDurationSecs:  maxSeconds,
		maxQuality:       maxQuality,
		maxFileSizeBytes: maxFileSizeBytes,
	}
}

func (r *Resolver) Resolve(ctx context.Context, rawURL string) (ResolveResult, error) {
	if r.ytdlpBinary == "" {
		return ResolveResult{}, errors.New("yt-dlp binary is not configured")
	}

	targetURL, headers, userAgent, err := parseCurlOrURL(rawURL)
	if err != nil {
		return ResolveResult{}, err
	}

	if err := validateYouTubeURL(targetURL); err != nil {
		return ResolveResult{}, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	args := []string{
		"--ignore-config",
		"--dump-single-json",
		"--no-playlist",
		"--skip-download",
		"--no-warnings",
	}
	if r.ytdlpJSRuntimes != "" {
		args = append(args, "--js-runtimes", r.ytdlpJSRuntimes)
	}

	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		args = append(args, "--add-header", fmt.Sprintf("%s: %s", key, value))
	}
	if userAgent != "" {
		args = append(args, "--user-agent", userAgent)
	}
	args = append(args, targetURL)

	cmd := exec.CommandContext(
		cmdCtx,
		r.ytdlpBinary,
		args...,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		if strings.Contains(errText, "Requested format is not available") {
			return ResolveResult{}, errors.New("yt-dlp failed to fetch playable formats; update yt-dlp to the latest version")
		}
		return ResolveResult{}, fmt.Errorf("failed to resolve URL: %s", errText)
	}

	var payload ytdlpOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return ResolveResult{}, errors.New("yt-dlp response is invalid")
	}

	if payload.IsLive || payload.LiveStatus == "is_live" || payload.LiveStatus == "is_upcoming" {
		return ResolveResult{}, errors.New("live content is not supported")
	}

	duration := int(math.Round(payload.Duration))
	if duration <= 0 {
		return ResolveResult{}, errors.New("video duration is unavailable")
	}
	if duration > r.maxDurationSecs {
		return ResolveResult{}, fmt.Errorf("video exceeds maximum duration (%d minutes)", r.maxDurationSecs/60)
	}

	formats := r.selectFormats(payload.Formats)
	if len(formats) == 0 {
		return ResolveResult{}, errors.New("no downloadable MP4 format is available")
	}

	return ResolveResult{
		Title:           payload.Title,
		Thumbnail:       chooseThumbnail(payload),
		DurationSeconds: duration,
		Formats:         formats,
	}, nil
}

func validateYouTubeURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must start with http or https")
	}

	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.Path, "/")

	switch host {
	case "youtu.be":
		if path == "" {
			return errors.New("invalid YouTube short URL")
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		switch {
		case path == "watch":
			if parsed.Query().Get("v") == "" {
				return errors.New("missing video id")
			}
		case strings.HasPrefix(path, "shorts/"):
			parts := strings.Split(path, "/")
			if len(parts) < 2 || parts[1] == "" {
				return errors.New("missing shorts id")
			}
		default:
			return errors.New("unsupported YouTube URL path")
		}
	default:
		return errors.New("URL must be a YouTube link")
	}

	if parsed.Query().Get("list") != "" {
		return errors.New("playlist URL is not supported")
	}

	return nil
}

func parseCurlOrURL(rawInput string) (string, map[string]string, string, error) {
	input := strings.TrimSpace(rawInput)
	if input == "" {
		return "", nil, "", errors.New("url is required")
	}

	if !strings.HasPrefix(strings.ToLower(input), "curl") {
		return input, nil, "", nil
	}

	tokens, err := shlex.Split(input)
	if err != nil {
		return "", nil, "", errors.New("failed to parse cURL input")
	}
	if len(tokens) == 0 {
		return "", nil, "", errors.New("failed to parse cURL input")
	}
	if strings.EqualFold(tokens[0], "curl") {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return "", nil, "", errors.New("failed to parse cURL input")
	}

	headers := make(map[string]string)
	userAgent := ""
	targetURL := ""

	for idx := 0; idx < len(tokens); idx++ {
		token := tokens[idx]
		switch token {
		case "-H", "--header":
			if idx+1 < len(tokens) {
				header := tokens[idx+1]
				colonIndex := strings.Index(header, ":")
				if colonIndex > 0 {
					name := strings.TrimSpace(header[:colonIndex])
					value := strings.TrimSpace(header[colonIndex+1:])
					if name != "" {
						headers[name] = value
					}
				}
				idx++
			}
		case "-A", "--user-agent":
			if idx+1 < len(tokens) {
				userAgent = tokens[idx+1]
				idx++
			}
		default:
			if strings.HasPrefix(token, "-") {
				continue
			}
			if targetURL == "" {
				targetURL = token
			}
		}
	}

	if targetURL == "" {
		return "", nil, "", errors.New("failed to parse URL from cURL input")
	}

	return strings.TrimSpace(targetURL), headers, userAgent, nil
}

func (r *Resolver) selectFormats(raw []ytdlpFormat) []Format {
	bestByHeight := make(map[int]Format)

	for _, item := range raw {
		if !isProgressiveMP4(item) {
			continue
		}
		if item.Height <= 0 || item.Height > r.maxQuality {
			continue
		}
		size := item.Filesize
		if size <= 0 {
			size = item.FilesizeApprox
		}
		if r.maxFileSizeBytes > 0 && size > r.maxFileSizeBytes {
			continue
		}
		if item.URL == "" {
			continue
		}

		candidate := Format{
			ID:          item.FormatID,
			Quality:     strconv.Itoa(item.Height) + "p",
			Container:   "mp4",
			Type:        "mp4",
			Progressive: true,
			URL:         item.URL,
			Filesize:    size,
		}

		current, exists := bestByHeight[item.Height]
		if !exists {
			bestByHeight[item.Height] = candidate
			continue
		}

		// Prefer larger known size as a rough proxy for better bitrate.
		if candidate.Filesize > current.Filesize {
			bestByHeight[item.Height] = candidate
		}
	}

	heights := make([]int, 0, len(bestByHeight))
	for height := range bestByHeight {
		heights = append(heights, height)
	}
	sort.Ints(heights)

	formats := make([]Format, 0, len(heights)+1)
	for _, height := range heights {
		formats = append(formats, bestByHeight[height])
	}

	// MP3 option is queue-based and always available as product-level choice.
	formats = append(formats, Format{
		ID:          "mp3-128",
		Quality:     "audio",
		Container:   "mp3 128kbps",
		Type:        "mp3",
		Progressive: false,
	})

	return formats
}

func isProgressiveMP4(item ytdlpFormat) bool {
	if strings.ToLower(item.Ext) != "mp4" {
		return false
	}
	if item.VideoCodec == "" || item.VideoCodec == "none" {
		return false
	}
	if item.AudioCodec == "" || item.AudioCodec == "none" {
		return false
	}
	return true
}

func chooseThumbnail(payload ytdlpOutput) string {
	if payload.Thumbnail != "" {
		return payload.Thumbnail
	}
	for idx := len(payload.Thumbnails) - 1; idx >= 0; idx-- {
		if payload.Thumbnails[idx].URL != "" {
			return payload.Thumbnails[idx].URL
		}
	}
	return ""
}
