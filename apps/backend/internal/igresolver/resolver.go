package igresolver

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

const (
	ErrCodeIGHLSOnlyNotSupported = "ig_hls_only_not_supported"
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

// ResolveError carries machine-readable resolver error code for API response mapping.
type ResolveError struct {
	Code    string
	Message string
}

func (e *ResolveError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return "resolve failed"
	}
	return e.Message
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
	Protocol       string  `json:"protocol"`
	FormatNote     string  `json:"format_note"`
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
	if err := validateInstagramURL(targetURL); err != nil {
		return ResolveResult{}, err
	}

	candidates := r.buildCookieCandidates()
	if len(candidates) == 0 {
		return ResolveResult{}, errors.New("instagram resolver has no cookie profile configured")
	}

	var lastErr error
	for _, candidate := range candidates {
		result, err := r.resolveWithCandidate(ctx, targetURL, headers, userAgent, candidate)
		if err == nil {
			return result, nil
		}
		if isNonRetryableResolveErr(err) {
			return ResolveResult{}, err
		}
		lastErr = err
	}

	if len(candidates) == 1 {
		return ResolveResult{}, lastErr
	}
	return ResolveResult{}, fmt.Errorf("failed to resolve Instagram URL with %d cookie profiles: %v", len(candidates), lastErr)
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
		if isLikelyHLSOnlySource(payload.Formats) {
			return ResolveResult{}, &ResolveError{
				Code:    ErrCodeIGHLSOnlyNotSupported,
				Message: "Instagram video is HLS-only and not supported yet",
			}
		}
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

func validateInstagramURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must start with http or https")
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "instagram.com", "www.instagram.com", "m.instagram.com", "instagr.am", "www.instagr.am":
	default:
		return errors.New("URL must be an Instagram link")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		switch parts[0] {
		case "reel", "reels", "p", "tv":
			return nil
		}
	}

	return errors.New("unsupported Instagram URL path")
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

func isNonRetryableResolveErr(err error) bool {
	var resolveErr *ResolveError
	if !errors.As(err, &resolveErr) {
		return false
	}
	return resolveErr.Code == ErrCodeIGHLSOnlyNotSupported
}

func isLikelyHLSOnlySource(raw []ytdlpFormat) bool {
	hasHLSVideo := false
	hasNonHLSVideo := false

	for _, item := range raw {
		if !hasVideoTrack(item) {
			continue
		}
		if isHLSFormat(item) {
			hasHLSVideo = true
			continue
		}
		hasNonHLSVideo = true
	}

	return hasHLSVideo && !hasNonHLSVideo
}

func hasVideoTrack(item ytdlpFormat) bool {
	if item.Height > 0 {
		return true
	}
	vcodec := strings.ToLower(strings.TrimSpace(item.VideoCodec))
	return vcodec != "" && vcodec != "none"
}

func isHLSFormat(item ytdlpFormat) bool {
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	formatID := strings.ToLower(strings.TrimSpace(item.FormatID))
	return strings.Contains(protocol, "m3u8") || strings.HasPrefix(formatID, "hls-")
}

func (r *Resolver) selectFormats(raw []ytdlpFormat) []Format {
	bestByHeight := make(map[int]Format)

	for _, item := range raw {
		// We prioritize progressive (v+a) or direct MP4s, but also accept video-only MP4s
		// as yt-dlp might fail to find a merged progressive stream for some Reels.
		// Instagram now primarily serves DASH video-only formats (dash-video-xxx),
		// so we also accept those via isDashVideoMP4.
		if !isProgressiveMP4(item) && !isLikelyInstagramDirectMP4(item) && !isVideoOnlyMP4(item) && !isDashVideoMP4(item) {
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

func isLikelyInstagramDirectMP4(item ytdlpFormat) bool {
	if strings.ToLower(item.Ext) != "mp4" {
		return false
	}
	if item.URL == "" {
		return false
	}
	if strings.TrimSpace(item.VideoCodec) != "" || strings.TrimSpace(item.AudioCodec) != "" {
		return false
	}

	formatID := strings.ToLower(strings.TrimSpace(item.FormatID))
	if strings.HasPrefix(formatID, "hls-") || strings.HasPrefix(formatID, "dash-") {
		return false
	}

	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	if protocol != "" && protocol != "https" && protocol != "http" {
		return false
	}

	note := strings.ToLower(strings.TrimSpace(item.FormatNote))
	if strings.Contains(note, "audio") && !strings.Contains(note, "video") {
		return false
	}
	if strings.Contains(note, "video only") {
		return false
	}

	return true
}

func isVideoOnlyMP4(item ytdlpFormat) bool {
	if strings.ToLower(item.Ext) != "mp4" {
		return false
	}
	if item.URL == "" || item.Height <= 0 {
		return false
	}
	// Has video but no audio
	vcodec := strings.ToLower(strings.TrimSpace(item.VideoCodec))
	acodec := strings.ToLower(strings.TrimSpace(item.AudioCodec))
	return vcodec != "" && vcodec != "none" && (acodec == "" || acodec == "none")
}

// isDashVideoMP4 matches Instagram DASH video-only MP4 segments.
// Instagram now primarily serves video via DASH with format IDs prefixed "dash-".
// These are valid downloadable video-only MP4 files served over HTTP(S).
func isDashVideoMP4(item ytdlpFormat) bool {
	formatID := strings.ToLower(strings.TrimSpace(item.FormatID))
	if !strings.HasPrefix(formatID, "dash-") {
		return false
	}
	if strings.ToLower(item.Ext) != "mp4" {
		return false
	}
	if item.URL == "" || item.Height <= 0 {
		return false
	}

	// Must use HTTP(S) protocol, not m3u8/HLS.
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	if protocol != "" && protocol != "https" && protocol != "http" {
		return false
	}

	// Reject audio-only DASH segments.
	vcodec := strings.ToLower(strings.TrimSpace(item.VideoCodec))
	acodec := strings.ToLower(strings.TrimSpace(item.AudioCodec))

	// If codec metadata is present, video codec must exist and audio must be absent.
	if vcodec != "" && vcodec == "none" {
		return false
	}
	if acodec != "" && acodec != "none" {
		return false
	}

	// Reject formats whose format_note indicates audio-only.
	note := strings.ToLower(strings.TrimSpace(item.FormatNote))
	if strings.Contains(note, "audio") && !strings.Contains(note, "video") {
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
