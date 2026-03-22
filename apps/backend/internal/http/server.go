package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"golang.org/x/time/rate"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/igresolver"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/ttresolver"
	"yt-downloader/backend/internal/xresolver"
	"yt-downloader/backend/internal/youtube"
)

type youtubeResolver interface {
	Resolve(ctx context.Context, rawURL string) (youtube.ResolveResult, error)
}

type xMediaResolver interface {
	Resolve(ctx context.Context, rawURL string) (xresolver.ResolveResult, error)
}

type igMediaResolver interface {
	Resolve(ctx context.Context, rawURL string) (igresolver.ResolveResult, error)
}

type ttMediaResolver interface {
	Resolve(ctx context.Context, rawURL string) (ttresolver.ResolveResult, error)
}

type taskQueue interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
	Close() error
}

type jobStore interface {
	Close() error
	Put(ctx context.Context, record jobs.Record) error
	Get(ctx context.Context, jobID string) (jobs.Record, error)
	Update(ctx context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error)
	ListRecent(ctx context.Context, limit int) ([]jobs.Record, error)
}

type Server struct {
	cfg          config.Config
	logger       *log.Logger
	resolver     youtubeResolver
	xResolver    xMediaResolver
	igResolver   igMediaResolver
	ttResolver   ttMediaResolver
	queue        taskQueue
	jobStore     jobStore
	authStore    *auth.Store
	historyStore *history.Store
	authService  *auth.Service
	origins      map[string]struct{}
	limiter      *ipRateLimiter
}

func NewServer(cfg config.Config, logger *log.Logger, resolver youtubeResolver) *Server {
	xResolver := xresolver.NewResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.XMaxQuality,
		cfg.MaxFileSizeBytes,
		cfg.XCookiesDir,
		cfg.XCookiesFiles,
		cfg.XResolveTryWithoutCookies,
	)
	igResolver := igresolver.NewResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.IGMaxQuality,
		cfg.MaxFileSizeBytes,
		cfg.IGCookiesDir,
		cfg.IGCookiesFiles,
		cfg.IGResolveTryWithoutCookies,
	)
	ttResolver := ttresolver.NewResolver(
		cfg.YTDLPBinary,
		cfg.YTDLPJSRuntimes,
		cfg.TTMaxQuality,
		cfg.MaxFileSizeBytes,
		cfg.TTCookiesDir,
		cfg.TTCookiesFiles,
		cfg.TTResolveTryWithoutCookies,
	)

	return newServerWithDeps(
		cfg,
		logger,
		resolver,
		xResolver,
		igResolver,
		ttResolver,
		asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword}),
		jobs.NewStore(cfg, logger),
	)
}

func newServerWithDeps(cfg config.Config, logger *log.Logger, resolver youtubeResolver, xResolver xMediaResolver, igResolver igMediaResolver, ttResolver ttMediaResolver, queue taskQueue, store jobStore) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if resolver == nil {
		panic("resolver is required")
	}
	if xResolver == nil {
		panic("x resolver is required")
	}
	if igResolver == nil {
		panic("instagram resolver is required")
	}
	if ttResolver == nil {
		panic("tiktok resolver is required")
	}
	if queue == nil {
		panic("queue is required")
	}
	if store == nil {
		panic("job store is required")
	}

	burst := int(math.Ceil(cfg.RateLimitRPS))
	if burst < 1 {
		burst = 1
	}

	authStore := auth.NewStore(cfg, logger)
	historyStore := history.NewStore(cfg, logger)
	googleVerifier := auth.NewGoogleTokenVerifier(auth.GoogleTokenVerifierOptions{
		ClientIDs: splitCommaSeparated(cfg.GoogleClientIDs),
	})
	authService := auth.NewService(authStore, auth.Options{
		SessionTTL:          time.Duration(cfg.AuthSessionTTLHours) * time.Hour,
		RememberSessionTTL:  time.Duration(cfg.AuthRememberSessionTTLHours) * time.Hour,
		BcryptCost:          cfg.AuthBcryptCost,
		GoogleTokenVerifier: googleVerifier,
	})

	return &Server{
		cfg:          cfg,
		logger:       logger,
		resolver:     resolver,
		xResolver:    xResolver,
		igResolver:   igResolver,
		ttResolver:   ttResolver,
		queue:        queue,
		jobStore:     store,
		authStore:    authStore,
		historyStore: historyStore,
		authService:  authService,
		origins:      parseAllowedOrigins(cfg.CORSAllowedOrigins),
		limiter:      newIPRateLimiter(rate.Limit(cfg.RateLimitRPS), burst),
	}
}

func (s *Server) Close() {
	if s.queue != nil {
		if err := s.queue.Close(); err != nil {
			s.logger.Printf("warning: close queue client: %v", err)
		}
	}
	if s.jobStore != nil {
		if err := s.jobStore.Close(); err != nil {
			s.logger.Printf("warning: close job store: %v", err)
		}
	}
	if s.authStore != nil {
		if err := s.authStore.Close(); err != nil {
			s.logger.Printf("warning: close auth store: %v", err)
		}
	}
	if s.historyStore != nil {
		if err := s.historyStore.Close(); err != nil {
			s.logger.Printf("warning: close history store: %v", err)
		}
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(s.corsMiddleware)
	if s.limiter != nil {
		r.Use(s.rateLimitMiddleware)
	}

	r.Get("/healthz", s.handleHealthz)
	r.Post("/v1/auth/register", s.handleAuthRegister)
	r.Post("/v1/auth/login", s.handleAuthLogin)
	r.Post("/v1/auth/google", s.handleAuthGoogleLogin)
	r.Get("/v1/auth/me", s.handleAuthMe)
	r.Post("/v1/auth/logout", s.handleAuthLogout)
	r.Post("/v1/youtube/resolve", s.handleResolveYouTube)
	r.Post("/v1/x/resolve", s.handleResolveX)
	r.Post("/v1/instagram/resolve", s.handleResolveInstagram)
	r.Post("/v1/ig/resolve", s.handleResolveInstagram)
	r.Post("/v1/tiktok/resolve", s.handleResolveTikTok)
	r.Post("/v1/tt/resolve", s.handleResolveTikTok)
	r.Post("/v1/jobs/mp3", s.handleCreateMP3Job)
	r.Get("/v1/jobs/{id}", s.handleGetJob)
	r.Get("/v1/download/mp4", s.handleRedirectMP4)
	r.With(s.basicAuth).Get("/admin/jobs", s.handleAdminJobs)

	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "api",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

type resolveYouTubeRequest struct {
	URL string `json:"url"`
}

func (s *Server) handleResolveYouTube(w http.ResponseWriter, r *http.Request) {
	var req resolveYouTubeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	result, err := s.resolver.Resolve(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResolveX(w http.ResponseWriter, r *http.Request) {
	var req resolveYouTubeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	result, err := s.xResolver.Resolve(r.Context(), req.URL)
	if err != nil {
		var resolveErr *xresolver.ResolveError
		if errors.As(err, &resolveErr) && strings.TrimSpace(resolveErr.Code) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": resolveErr.Error(),
				"code":  resolveErr.Code,
			})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResolveInstagram(w http.ResponseWriter, r *http.Request) {
	var req resolveYouTubeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	result, err := s.igResolver.Resolve(r.Context(), req.URL)
	if err != nil {
		var resolveErr *igresolver.ResolveError
		if errors.As(err, &resolveErr) && strings.TrimSpace(resolveErr.Code) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": resolveErr.Error(),
				"code":  resolveErr.Code,
			})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResolveTikTok(w http.ResponseWriter, r *http.Request) {
	var req resolveYouTubeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	result, err := s.ttResolver.Resolve(r.Context(), req.URL)
	if err != nil {
		var resolveErr *ttresolver.ResolveError
		if errors.As(err, &resolveErr) && strings.TrimSpace(resolveErr.Code) != "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": resolveErr.Error(),
				"code":  resolveErr.Code,
			})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type createMP3JobRequest struct {
	URL string `json:"url"`
}

func (s *Server) handleCreateMP3Job(w http.ResponseWriter, r *http.Request) {
	var req createMP3JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	platform := s.detectPlatform(req.URL)
	var title string
	switch platform {
	case "youtube":
		res, err := s.resolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
	case "tiktok":
		res, err := s.ttResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
	case "x":
		res, err := s.xResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
	case "instagram":
		res, err := s.igResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
	default:
		writeError(w, http.StatusBadRequest, "unsupported platform")
		return
	}

	sourceURL, headers, userAgent, err := youtube.ParseInput(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	jobID := "job_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	outputKey := buildMP3OutputKey(s.cfg.R2KeyPrefix, jobID)
	now := time.Now().UTC()

	record := jobs.Record{
		ID:         jobID,
		Status:     jobs.StatusQueued,
		InputURL:   sourceURL,
		OutputKind: "mp3",
		OutputKey:  outputKey,
		Title:      title,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.jobStore.Put(r.Context(), record); err != nil {
		s.logger.Printf("failed to persist queued job id=%s err=%v", jobID, err)
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	payload := queue.ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   sourceURL,
		Headers:     headers,
		UserAgent:   userAgent,
		OutputKey:   outputKey,
		BitrateKbps: s.cfg.MP3Bitrate,
	}
	taskBytes, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	task := asynq.NewTask(queue.TaskConvertMP3, taskBytes)
	_, err = s.queue.Enqueue(
		task,
		asynq.TaskID(jobID),
		asynq.Queue("mp3"),
		asynq.Timeout(20*time.Minute),
		asynq.MaxRetry(2),
	)
	if err != nil {
		_, _ = s.jobStore.Update(r.Context(), jobID, func(item *jobs.Record) {
			item.Status = jobs.StatusFailed
			item.Error = "failed to enqueue job"
		})
		s.logger.Printf("failed to enqueue mp3 job id=%s err=%v", jobID, err)
		writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id": jobID,
		"status": jobs.StatusQueued,
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	record, err := s.jobStore.Get(r.Context(), jobID)
	if errors.Is(err, jobs.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.logger.Printf("failed to read job id=%s err=%v", jobID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleRedirectMP4(w http.ResponseWriter, r *http.Request) {
	sourceURL := strings.TrimSpace(r.URL.Query().Get("url"))
	formatID := strings.TrimSpace(r.URL.Query().Get("format_id"))
	if sourceURL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if formatID == "" {
		writeError(w, http.StatusBadRequest, "format_id is required")
		return
	}

	platform := s.detectPlatform(sourceURL)
	var title string
	var formats []struct {
		ID   string
		URL  string
		Type string
	}

	switch platform {
	case "youtube":
		res, err := s.resolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		for _, f := range res.Formats {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: f.ID, URL: f.URL, Type: f.Type})
		}
	case "tiktok":
		res, err := s.ttResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		for _, f := range res.Formats {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: f.ID, URL: f.URL, Type: f.Type})
		}
	case "x":
		res, err := s.xResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		for _, f := range res.Formats {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: f.ID, URL: f.URL, Type: f.Type})
		}
		for _, m := range res.Medias {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: m.ID, URL: m.URL, Type: m.Type})
		}
	case "instagram":
		res, err := s.igResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		for _, f := range res.Formats {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: f.ID, URL: f.URL, Type: f.Type})
		}
		// Also look into Medias (for carousel or single images)
		for _, m := range res.Medias {
			formats = append(formats, struct {
				ID   string
				URL  string
				Type string
			}{ID: m.ID, URL: m.URL, Type: m.Type})
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported platform")
		return
	}

	for _, format := range formats {
		if format.ID == formatID {
			if format.URL == "" {
				writeError(w, http.StatusBadRequest, "selected format is unavailable")
				return
			}

			// Force download by setting Content-Disposition
			ext := "mp4"
			contentType := "video/mp4"
			if format.Type == "image" {
				ext = "jpg"
				contentType = "image/jpeg"
			}

			filename := "file." + ext
			if strings.TrimSpace(title) != "" {
				// Simple cleanup of title for filename
				cleanTitle := strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
						return r
					}
					return '_'
				}, title)
				if cleanTitle != "" {
					filename = cleanTitle + "." + ext
				}
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			w.Header().Set("Content-Type", contentType)

			// Proxy the stream
			req, err := http.NewRequestWithContext(r.Context(), "GET", format.URL, nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create upstream request")
				return
			}

			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to fetch content from source")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				writeError(w, http.StatusBadGateway, fmt.Sprintf("source returned status %d", resp.StatusCode))
				return
			}

			if resp.ContentLength > 0 {
				w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
			}

			_, _ = io.Copy(w, resp.Body)
			return
		}
	}

	writeError(w, http.StatusBadRequest, "selected format is not available")
}

func (s *Server) detectPlatform(rawURL string) string {
	targetURL, _, _, err := youtube.ParseInput(rawURL)
	if err != nil {
		return "unknown"
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "unknown"
	}

	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		return "youtube"
	}
	if strings.Contains(host, "tiktok.com") {
		return "tiktok"
	}
	if strings.Contains(host, "instagram.com") {
		return "instagram"
	}
	if strings.Contains(host, "twitter.com") || strings.Contains(host, "x.com") {
		return "x"
	}

	return "unknown"
}

func (s *Server) handleAdminJobs(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	items, err := s.jobStore.ListRecent(r.Context(), limit)
	if err != nil {
		s.logger.Printf("failed to list admin jobs err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch admin jobs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != s.cfg.AdminBasicAuthUser || pass != s.cfg.AdminBasicAuthPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if s.originAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		if !s.limiter.Allow(clientIP) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if _, ok := s.origins["*"]; ok {
		return true
	}
	_, ok := s.origins[origin]
	return ok
}

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(limit rate.Limit, burst int) *ipRateLimiter {
	if limit <= 0 {
		return nil
	}
	if burst < 1 {
		burst = 1
	}

	limiter := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		burst:    burst,
	}
	go limiter.cleanupEvery(2 * time.Minute)
	return limiter
}

func (l *ipRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.visitors[ip]
	if !exists {
		entry = &visitor{
			limiter:  rate.NewLimiter(l.limit, l.burst),
			lastSeen: time.Now().UTC(),
		}
		l.visitors[ip] = entry
	}

	entry.lastSeen = time.Now().UTC()
	return entry.limiter.Allow()
}

func (l *ipRateLimiter) cleanupEvery(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().UTC().Add(-5 * time.Minute)
		l.mu.Lock()
		for ip, entry := range l.visitors {
			if entry.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func buildMP3OutputKey(prefix, jobID string) string {
	cleanJobID := strings.TrimSpace(jobID)
	if cleanJobID == "" {
		cleanJobID = "unknown"
	}

	segments := make([]string, 0, 3)
	if trimmedPrefix := strings.Trim(prefix, " /"); trimmedPrefix != "" {
		segments = append(segments, trimmedPrefix)
	}
	segments = append(segments, "mp3", cleanJobID+".mp3")

	return strings.Join(segments, "/")
}

func splitCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, value := range splitCommaSeparated(raw) {
		origins[value] = struct{}{}
	}
	if len(origins) == 0 {
		origins["http://127.0.0.1:3000"] = struct{}{}
		origins["http://localhost:3000"] = struct{}{}
	}
	return origins
}

func getClientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]any{
		"error": message,
	})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
