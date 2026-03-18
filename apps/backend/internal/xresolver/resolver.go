package xresolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"yt-downloader/backend/internal/youtube"
)

type Resolver struct {
	ytdlpBinary          string
	ytdlpJSRuntimes      string
	maxQuality           int
	maxFileSizeBytes     int64
	cookiesDir           string
	cookiesFiles         string
	tryWithoutCookieFile bool
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
	DurationSeconds int      `json:"duration_seconds,omitempty"`
	Formats         []Format `json:"formats"`
	CookieProfile   string   `json:"cookie_profile,omitempty"`
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
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
}

type cookieCandidate struct {
	profile string
	path    string
}

func NewResolver(
	ytdlpBinary string,
	ytdlpJSRuntimes string,
	maxQuality int,
	maxFileSizeBytes int64,
	cookiesDir string,
	cookiesFiles string,
	tryWithoutCookieFile bool,
) *Resolver {
	if maxQuality <= 0 {
		maxQuality = 1080
	}

	return &Resolver{
		ytdlpBinary:          ytdlpBinary,
		ytdlpJSRuntimes:      strings.TrimSpace(ytdlpJSRuntimes),
		maxQuality:           maxQuality,
		maxFileSizeBytes:     maxFileSizeBytes,
		cookiesDir:           strings.TrimSpace(cookiesDir),
		cookiesFiles:         strings.TrimSpace(cookiesFiles),
		tryWithoutCookieFile: tryWithoutCookieFile,
	}
}

func (r *Resolver) Resolve(ctx context.Context, rawURL string) (ResolveResult, error) {
	if r.ytdlpBinary == "" {
		return ResolveResult{}, errors.New("yt-dlp binary is not configured")
	}

	targetURL, headers, userAgent, err := youtube.ParseInput(rawURL)
	if err != nil {
		return ResolveResult{}, err
	}
	if err := validateXURL(targetURL); err != nil {
		return ResolveResult{}, err
	}

	candidates := r.buildCookieCandidates()
	if len(candidates) == 0 {
		return ResolveResult{}, errors.New("x resolver has no cookie profile configured")
	}

	var lastErr error
	for _, candidate := range candidates {
		result, err := r.resolveWithCandidate(ctx, targetURL, headers, userAgent, candidate)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	if len(candidates) == 1 {
		return ResolveResult{}, lastErr
	}
	return ResolveResult{}, fmt.Errorf("failed to resolve X URL with %d cookie profiles: %v", len(candidates), lastErr)
}

func (r *Resolver) resolveWithCandidate(ctx context.Context, targetURL string, headers map[string]string, userAgent string, candidate cookieCandidate) (ResolveResult, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
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
	if candidate.path != "" {
		args = append(args, "--cookies", candidate.path)
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

	cmd := exec.CommandContext(cmdCtx, r.ytdlpBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		if candidate.profile != "" {
			return ResolveResult{}, fmt.Errorf("cookie profile %q failed: %s", candidate.profile, errText)
		}
		return ResolveResult{}, fmt.Errorf("public resolve failed: %s", errText)
	}

	var payload ytdlpOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return ResolveResult{}, errors.New("yt-dlp response is invalid")
	}

	if payload.IsLive || payload.LiveStatus == "is_live" || payload.LiveStatus == "is_upcoming" {
		return ResolveResult{}, errors.New("live content is not supported")
	}

	formats := r.selectFormats(payload.Formats)
	if len(formats) == 0 {
		return ResolveResult{}, errors.New("no downloadable MP4 format is available")
	}

	durationSeconds := int(math.Round(payload.Duration))
	if durationSeconds < 0 {
		durationSeconds = 0
	}

	result := ResolveResult{
		Title:           payload.Title,
		Thumbnail:       chooseThumbnail(payload),
		DurationSeconds: durationSeconds,
		Formats:         formats,
	}
	if candidate.profile != "" {
		result.CookieProfile = candidate.profile
	}

	return result, nil
}

func validateXURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must start with http or https")
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "x.com", "www.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com":
	default:
		return errors.New("URL must be an X/Twitter link")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 3 && parts[1] == "status" && parts[0] != "" && parts[2] != "" {
		return nil
	}
	if len(parts) >= 3 && parts[0] == "i" && parts[1] == "status" && parts[2] != "" {
		return nil
	}

	return errors.New("unsupported X URL path")
}

func (r *Resolver) buildCookieCandidates() []cookieCandidate {
	seen := make(map[string]struct{})
	out := make([]cookieCandidate, 0)

	if r.tryWithoutCookieFile {
		out = append(out, cookieCandidate{})
	}

	appendCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		resolvedPath := path
		if absPath, err := filepath.Abs(path); err == nil {
			resolvedPath = absPath
		}
		if _, ok := seen[resolvedPath]; ok {
			return
		}
		info, err := os.Stat(resolvedPath)
		if err != nil || info.IsDir() {
			return
		}
		seen[resolvedPath] = struct{}{}

		profile := strings.TrimSuffix(filepath.Base(resolvedPath), filepath.Ext(resolvedPath))
		if strings.TrimSpace(profile) == "" {
			profile = "cookie"
		}
		out = append(out, cookieCandidate{profile: profile, path: resolvedPath})
	}

	for _, part := range strings.Split(r.cookiesFiles, ",") {
		appendCandidate(part)
	}

	if r.cookiesDir != "" {
		entries, err := os.ReadDir(r.cookiesDir)
		if err == nil {
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Name() < entries[j].Name()
			})
			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				appendCandidate(filepath.Join(r.cookiesDir, entry.Name()))
			}
		}
	}

	return out
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
		if !exists || candidate.Filesize > current.Filesize {
			bestByHeight[item.Height] = candidate
		}
	}

	heights := make([]int, 0, len(bestByHeight))
	for height := range bestByHeight {
		heights = append(heights, height)
	}
	sort.Ints(heights)

	formats := make([]Format, 0, len(heights))
	for _, height := range heights {
		formats = append(formats, bestByHeight[height])
	}
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
