package http

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/youtube"
)

type Server struct {
	cfg      config.Config
	logger   *log.Logger
	resolver *youtube.Resolver
}

func NewServer(cfg config.Config, logger *log.Logger, resolver *youtube.Resolver) *Server {
	return &Server{
		cfg:      cfg,
		logger:   logger,
		resolver: resolver,
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", s.handleHealthz)
	r.Post("/v1/youtube/resolve", s.handleResolveYouTube)
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
	if req.URL == "" {
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

func (s *Server) handleCreateMP3Job(w http.ResponseWriter, _ *http.Request) {
	// Placeholder: this endpoint will enqueue Asynq task and return job_id.
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id": "job_stub",
		"status": "queued",
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         jobID,
		"status":     "processing",
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRedirectMP4(w http.ResponseWriter, r *http.Request) {
	// Placeholder: use query params + resolved format map to redirect final URL.
	target := r.URL.Query().Get("target")
	if target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) handleAdminJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": []map[string]any{
			{
				"id":         "job_01HYE6B5RWM",
				"status":     "done",
				"output":     "mp3",
				"created_at": "2026-03-10T11:20:00Z",
			},
			{
				"id":         "job_01HYE6B7EHM",
				"status":     "failed",
				"output":     "mp3",
				"created_at": "2026-03-10T11:24:00Z",
			},
		},
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
