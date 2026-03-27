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
	"github.com/hibiken/asynq"
	"golang.org/x/time/rate"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/avatar"
	"yt-downloader/backend/internal/billing"
	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/igresolver"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/maintenance"
	"yt-downloader/backend/internal/settings"
	"yt-downloader/backend/internal/storage"
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
	cfg                config.Config
	logger             *log.Logger
	resolver           youtubeResolver
	xResolver          xMediaResolver
	igResolver         igMediaResolver
	ttResolver         ttMediaResolver
	queue              taskQueue
	jobStore           jobStore
	authStore          *auth.Store
	historyStore       *history.Store
	settingsStore      *settings.Store
	maintenanceStore   *maintenance.Store
	billingStore       *billing.Store
	authService        *auth.Service
	settingsService    *settings.Service
	maintenanceService *maintenance.Service
	billingService     *billing.Service
	avatarService      *avatar.Service
	origins            map[string]struct{}
	limiter            *ipRateLimiter
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
	settingsStore := settings.NewStore(cfg, logger)
	maintenanceStore := maintenance.NewStore(cfg, logger)
	billingStore := billing.NewStore(cfg, logger)
	googleVerifier := auth.NewGoogleTokenVerifier(auth.GoogleTokenVerifierOptions{
		ClientIDs: splitCommaSeparated(cfg.GoogleClientIDs),
	})
	authService := auth.NewService(authStore, auth.Options{
		SessionTTL:          time.Duration(cfg.AuthSessionTTLHours) * time.Hour,
		RememberSessionTTL:  time.Duration(cfg.AuthRememberSessionTTLHours) * time.Hour,
		BcryptCost:          cfg.AuthBcryptCost,
		GoogleTokenVerifier: googleVerifier,
	})
	settingsService := settings.NewService(settingsStore)
	maintenanceService := maintenance.NewService(maintenanceStore)
	billingService := billing.NewService(billingStore)

	var avatarService *avatar.Service
	r2Client, r2Err := storage.NewR2Client(context.Background(), cfg)
	if r2Err != nil {
		logger.Printf("warning: avatar service disabled, r2 unavailable: %v", r2Err)
	} else {
		avatarProcessor := avatar.NewFFmpegWebPProcessor(cfg.AvatarFFmpegBinary, avatar.DefaultTargetSize)
		avatarService, r2Err = avatar.NewService(authStore, r2Client, avatarProcessor, avatar.Options{
			PublicBaseURL:  cfg.AvatarPublicBaseURL,
			KeyPrefix:      cfg.AvatarR2KeyPrefix,
			MaxUploadBytes: cfg.AvatarUploadMaxBytes,
		})
		if r2Err != nil {
			logger.Printf("warning: avatar service disabled, init failed: %v", r2Err)
		}
	}

	server := &Server{
		cfg:                cfg,
		logger:             logger,
		resolver:           resolver,
		xResolver:          xResolver,
		igResolver:         igResolver,
		ttResolver:         ttResolver,
		queue:              queue,
		jobStore:           store,
		authStore:          authStore,
		historyStore:       historyStore,
		settingsStore:      settingsStore,
		maintenanceStore:   maintenanceStore,
		billingStore:       billingStore,
		authService:        authService,
		settingsService:    settingsService,
		maintenanceService: maintenanceService,
		billingService:     billingService,
		avatarService:      avatarService,
		origins:            parseAllowedOrigins(cfg.CORSAllowedOrigins),
		limiter:            newIPRateLimiter(rate.Limit(cfg.RateLimitRPS), burst),
	}

	server.seedDummyUsers()

	return server
}

func (s *Server) seedDummyUsers() {
	if s.authStore == nil {
		return
	}

	ctx := context.Background()
	now := time.Now().UTC()

	// 4 Free Users
	freeUsers := []struct {
		ID    string
		Name  string
		Email string
	}{
		{"usr_dummy_free1", "Free User 1", "free1@example.com"},
		{"usr_dummy_free2", "Free User 2", "free2@example.com"},
		{"usr_dummy_free3", "Free User 3", "free3@example.com"},
		{"usr_dummy_free4", "Free User 4", "free4@example.com"},
	}

	// 6 Subscribed Users
	subscribedUsers := []struct {
		ID    string
		Name  string
		Email string
		Plan  auth.Plan
	}{
		{"usr_dummy_daily1", "Daily Subscriber 1", "daily1@example.com", auth.PlanDaily},
		{"usr_dummy_daily2", "Daily Subscriber 2", "daily2@example.com", auth.PlanDaily},
		{"usr_dummy_weekly1", "Weekly Subscriber 1", "weekly1@example.com", auth.PlanWeekly},
		{"usr_dummy_weekly2", "Weekly Subscriber 2", "weekly2@example.com", auth.PlanWeekly},
		{"usr_dummy_monthly1", "Monthly Subscriber 1", "monthly1@example.com", auth.PlanMonthly},
		{"usr_dummy_monthly2", "Monthly Subscriber 2", "monthly2@example.com", auth.PlanMonthly},
	}

	dummyHash := "$2a$10$7EqJtq98hPqEX7fNZaFWoOe2fN5z9Yx8RJWb1x1o1t4bm/FvGV8e6" // change-me

	for _, u := range freeUsers {
		_ = s.authStore.CreateUser(ctx, auth.User{
			ID:           u.ID,
			FullName:     u.Name,
			Email:        u.Email,
			PasswordHash: dummyHash,
			Role:         auth.RoleUser,
			Plan:         auth.PlanFree,
			CreatedAt:    now.Add(-24 * time.Hour),
			UpdatedAt:    now.Add(-24 * time.Hour),
		})
	}

	for i, u := range subscribedUsers {
		expiresAt := now.Add(30 * 24 * time.Hour)
		_ = s.authStore.CreateUser(ctx, auth.User{
			ID:            u.ID,
			FullName:      u.Name,
			Email:         u.Email,
			PasswordHash:  dummyHash,
			Role:          auth.RoleUser,
			Plan:          u.Plan,
			PlanExpiresAt: &expiresAt,
			CreatedAt:     now.Add(time.Duration(-i) * time.Hour),
			UpdatedAt:     now.Add(time.Duration(-i) * time.Hour),
		})
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
	if s.settingsStore != nil {
		if err := s.settingsStore.Close(); err != nil {
			s.logger.Printf("warning: close settings store: %v", err)
		}
	}
	if s.maintenanceStore != nil {
		if err := s.maintenanceStore.Close(); err != nil {
			s.logger.Printf("warning: close maintenance store: %v", err)
		}
	}
	if s.billingStore != nil {
		if err := s.billingStore.Close(); err != nil {
			s.logger.Printf("warning: close billing store: %v", err)
		}
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(s.corsMiddleware)
	if s.limiter != nil {
		r.Use(s.rateLimitMiddleware)
	}
	r.Use(s.maintenanceGuardMiddleware)

	r.Get("/healthz", s.handleHealthz)
	r.Get("/v1/maintenance", s.handleMaintenancePublicGet)

	r.Post("/v1/auth/register", s.handleAuthRegister)
	r.Post("/v1/auth/login", s.handleAuthLogin)
	r.Post("/v1/auth/google", s.handleAuthGoogleLogin)
	r.Get("/v1/auth/me", s.handleAuthMe)
	r.Post("/v1/auth/logout", s.handleAuthLogout)
	r.Get("/v1/profile", s.handleProfileGet)
	r.Patch("/v1/profile", s.handleProfilePatch)
	r.Post("/v1/profile/avatar", s.handleProfileAvatarUpload)
	r.Delete("/v1/profile/avatar", s.handleProfileAvatarDelete)
	r.Get("/v1/settings", s.handleSettingsGet)
	r.Patch("/v1/settings", s.handleSettingsPatch)
	// Subscription & billing
	r.Get("/v1/subscription", s.handleSubscriptionGet)
	r.Patch("/v1/subscription", s.handleSubscriptionPatch)
	r.Post("/v1/subscription/cancel", s.handleSubscriptionCancel)
	r.Post("/v1/subscription/reactivate", s.handleSubscriptionReactivate)
	r.Get("/v1/billing/history", s.handleBillingHistoryList)
	r.Get("/v1/billing/invoices/{id}", s.handleBillingInvoiceGet)
	r.Get("/v1/billing/invoices/{id}/receipt", s.handleBillingInvoiceReceipt)
	r.Get("/v1/history", s.handleHistoryList)
	r.Post("/v1/history", s.handleHistoryCreate)
	r.Get("/v1/history/stats", s.handleHistoryStats)
	r.Post("/v1/history/{id}/redownload", s.handleHistoryRedownload)
	r.Delete("/v1/history/{id}", s.handleHistoryDelete)
	r.Post("/v1/youtube/resolve", s.handleResolveYouTube)
	r.Post("/v1/x/resolve", s.handleResolveX)
	r.Post("/v1/instagram/resolve", s.handleResolveInstagram)
	r.Post("/v1/ig/resolve", s.handleResolveInstagram)
	r.Post("/v1/tiktok/resolve", s.handleResolveTikTok)
	r.Post("/v1/tt/resolve", s.handleResolveTikTok)
	r.Post("/v1/jobs/mp3", s.handleCreateMP3Job)
	r.Post("/v1/jobs/video-cut", s.handleCreateVideoCutJob)
	r.Get("/v1/jobs/{id}", s.handleGetJob)
	r.Get("/v1/download/mp4", s.handleRedirectMP4)

	// Admin routes
	r.Route("/admin", func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Get("/jobs", s.handleAdminJobs)
		r.Get("/users", s.handleAdminUsersList)
		r.Get("/users/{id}", s.handleAdminUserGet)
		r.Patch("/users/{id}", s.handleAdminUserPatch)
		r.Get("/maintenance", s.handleAdminMaintenanceGet)
		r.Patch("/maintenance", s.handleAdminMaintenancePatch)
	})

	r.Route("/v1/admin", func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Get("/users", s.handleAdminUsersList)
		r.Get("/users/{id}", s.handleAdminUserGet)
		r.Patch("/users/{id}", s.handleAdminUserPatch)
		r.Get("/maintenance", s.handleAdminMaintenanceGet)
		r.Patch("/maintenance", s.handleAdminMaintenancePatch)
	})

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
	var thumbnail string
	switch platform {
	case "youtube":
		res, err := s.resolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
	case "tiktok":
		res, err := s.ttResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
	case "x":
		res, err := s.xResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
	case "instagram":
		res, err := s.igResolver.Resolve(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
	default:
		writeError(w, http.StatusBadRequest, "unsupported platform")
		return
	}

	sourceURL, headers, userAgent, err := youtube.ParseInput(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	identity := s.optionalSessionIdentity(r)
	userID := ""
	if identity != nil {
		userID = identity.User.ID
	}

	jobID, err := s.enqueueMP3Job(r.Context(), enqueueMP3Params{
		SourceURL: sourceURL,
		Headers:   headers,
		UserAgent: userAgent,
		Platform:  platform,
		Title:     title,
		Thumbnail: thumbnail,
		UserID:    userID,
	})
	if err != nil {
		s.logger.Printf("failed to create mp3 job source=%s err=%v", sourceURL, err)
		writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id": jobID,
		"status": jobs.StatusQueued,
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "id"))
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	if !s.authorizeJobRead(w, r, jobID) {
		return
	}

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

func (s *Server) authorizeJobRead(w http.ResponseWriter, r *http.Request, jobID string) bool {
	if s == nil || s.historyStore == nil {
		return true
	}

	attempt, err := s.historyStore.GetAttemptByJobID(r.Context(), jobID)
	if err != nil {
		switch {
		case errors.Is(err, history.ErrAttemptNotFound):
			// Job without owned history attempt (anonymous/legacy) stays readable.
			return true
		case errors.Is(err, history.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "job id is required")
			return false
		default:
			s.logger.Printf("failed to authorize job read job_id=%s err=%v", jobID, err)
			writeError(w, http.StatusInternalServerError, "failed to authorize job access")
			return false
		}
	}

	ownerUserID := strings.TrimSpace(attempt.UserID)
	if ownerUserID == "" {
		return true
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return false
	}
	if strings.TrimSpace(identity.User.ID) != ownerUserID {
		writeError(w, http.StatusNotFound, "job not found")
		return false
	}

	return true
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

	type downloadableFormat struct {
		ID        string
		URL       string
		Type      string
		Quality   string
		Filesize  int64
		Thumbnail string
	}

	platform := s.detectPlatform(sourceURL)
	var title string
	var thumbnail string
	formats := make([]downloadableFormat, 0, 16)

	switch platform {
	case "youtube":
		res, err := s.resolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
		for _, f := range res.Formats {
			formats = append(formats, downloadableFormat{ID: f.ID, URL: f.URL, Type: f.Type, Quality: f.Quality, Filesize: f.Filesize})
		}
	case "tiktok":
		res, err := s.ttResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
		for _, f := range res.Formats {
			formats = append(formats, downloadableFormat{ID: f.ID, URL: f.URL, Type: f.Type, Quality: f.Quality, Filesize: f.Filesize})
		}
	case "x":
		res, err := s.xResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
		for _, f := range res.Formats {
			formats = append(formats, downloadableFormat{ID: f.ID, URL: f.URL, Type: f.Type, Quality: f.Quality, Filesize: f.Filesize})
		}
		for _, m := range res.Medias {
			formats = append(formats, downloadableFormat{ID: m.ID, URL: m.URL, Type: m.Type, Quality: m.Quality, Thumbnail: m.Thumbnail})
		}
	case "instagram":
		res, err := s.igResolver.Resolve(r.Context(), sourceURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title = res.Title
		thumbnail = res.Thumbnail
		for _, f := range res.Formats {
			formats = append(formats, downloadableFormat{ID: f.ID, URL: f.URL, Type: f.Type, Quality: f.Quality, Filesize: f.Filesize})
		}
		for _, m := range res.Medias {
			formats = append(formats, downloadableFormat{ID: m.ID, URL: m.URL, Type: m.Type, Quality: m.Quality, Thumbnail: m.Thumbnail})
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported platform")
		return
	}

	identity := s.optionalSessionIdentity(r)
	for _, format := range formats {
		if format.ID != formatID {
			continue
		}

		if format.URL == "" {
			writeError(w, http.StatusBadRequest, "selected format is unavailable")
			return
		}

		requestKind := history.RequestKindMP4
		ext := "mp4"
		contentType := "video/mp4"
		if strings.EqualFold(format.Type, "image") {
			requestKind = history.RequestKindImage
			ext = "jpg"
			contentType = "image/jpeg"
		}

		var historyAttempt *history.Attempt
		if identity != nil {
			createdAttempt, ok := s.createHistoryAttempt(r.Context(), historyAttemptCreateParams{
				UserID:       identity.User.ID,
				Platform:     platform,
				SourceURL:    sourceURL,
				Title:        title,
				ThumbnailURL: firstNonEmpty(format.Thumbnail, thumbnail),
				RequestKind:  requestKind,
				Status:       history.StatusProcessing,
				FormatID:     format.ID,
				QualityLabel: format.Quality,
			})
			if ok {
				historyAttempt = createdAttempt
			}
		}

		filename := "file." + ext
		if strings.TrimSpace(title) != "" {
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

		upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, format.URL, nil)
		if err != nil {
			if historyAttempt != nil {
				s.markHistoryAttemptFailed(historyAttempt, "upstream_request_create_failed", err)
			}
			writeError(w, http.StatusInternalServerError, "failed to create upstream request")
			return
		}
		upstreamReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := http.DefaultClient.Do(upstreamReq)
		if err != nil {
			if historyAttempt != nil {
				s.markHistoryAttemptFailed(historyAttempt, "upstream_fetch_failed", err)
			}
			writeError(w, http.StatusBadGateway, "failed to fetch content from source")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if historyAttempt != nil {
				s.markHistoryAttemptFailed(historyAttempt, "upstream_bad_status", fmt.Errorf("source returned status %d", resp.StatusCode))
			}
			writeError(w, http.StatusBadGateway, fmt.Sprintf("source returned status %d", resp.StatusCode))
			return
		}

		if resp.ContentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		}

		bytesWritten, copyErr := io.Copy(w, resp.Body)
		if copyErr != nil {
			if historyAttempt != nil {
				s.markHistoryAttemptFailed(historyAttempt, "upstream_copy_failed", copyErr)
			}
			return
		}

		if historyAttempt != nil {
			var sizeBytes *int64
			if resp.ContentLength > 0 {
				value := resp.ContentLength
				sizeBytes = &value
			} else if bytesWritten > 0 {
				value := bytesWritten
				sizeBytes = &value
			} else if format.Filesize > 0 {
				value := format.Filesize
				sizeBytes = &value
			}
			s.markHistoryAttemptDone(historyAttempt, sizeBytes)
		}
		return
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

func (s *Server) handleAdminUsersList(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, total, err := s.authService.ListUsers(r.Context(), limit, offset)
	if err != nil {
		s.logger.Printf("failed to list admin users err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"page": map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
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

func (s *Server) requireSessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := s.requireSessionIdentity(w, r)
		if !ok {
			return
		}
		ctx := context.WithValue(r.Context(), "identity", *identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try Basic Auth first (common for CLI/scripts)
		user, pass, ok := r.BasicAuth()
		if ok && user == s.cfg.AdminBasicAuthUser && pass == s.cfg.AdminBasicAuthPass {
			next.ServeHTTP(w, r)
			return
		}

		// Try Session Auth (Bearer token or Cookie)
		token := s.readSessionToken(r)
		if token != "" {
			if s.authService == nil {
				writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
				return
			}
			identity, err := s.authService.AuthenticateToken(r.Context(), token)
			if err == nil {
				if identity.User.Role == auth.RoleAdmin {
					ctx := context.WithValue(r.Context(), "identity", identity)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				writeError(w, http.StatusForbidden, "forbidden: admin role required")
				return
			}
			s.writeAuthSessionError(w, err)
			return
		}

		// If no valid auth provided, prompt for Basic Auth
		w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := r.Context().Value("identity").(auth.SessionIdentity)
		if !ok || identity.User.Role != auth.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden: admin role required")
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

func writeErrorWithCode(w http.ResponseWriter, statusCode int, errorCode string, message string) {
	payload := map[string]any{
		"error": message,
	}
	if strings.TrimSpace(errorCode) != "" {
		payload["code"] = strings.TrimSpace(errorCode)
	}
	writeJSON(w, statusCode, payload)
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
