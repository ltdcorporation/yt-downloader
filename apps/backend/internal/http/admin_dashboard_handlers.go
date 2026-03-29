package http

import (
	"net/http"
	"strconv"
	"strings"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/jobs"
)

const (
	defaultAdminDashboardUsersLimit = 8
	defaultAdminDashboardJobsLimit  = 20
	maxAdminDashboardUsersLimit     = 100
	maxAdminDashboardJobsLimit      = 100
)

type adminDashboardUsersPage struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type adminDashboardUsersResponse struct {
	Items []auth.PublicUser       `json:"items"`
	Page  adminDashboardUsersPage `json:"page"`
}

type adminDashboardJobsResponse struct {
	Items []jobs.Record `json:"items"`
}

type adminDashboardSnapshotResponse struct {
	Stats       auth.UserStats              `json:"stats"`
	Users       adminDashboardUsersResponse `json:"users"`
	Jobs        adminDashboardJobsResponse  `json:"jobs"`
	Maintenance maintenanceSnapshotResponse `json:"maintenance"`
}

func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	if s.maintenanceService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	usersLimit := parsePositiveQueryLimit(r.URL.Query().Get("users_limit"), defaultAdminDashboardUsersLimit, maxAdminDashboardUsersLimit)
	jobsLimit := parsePositiveQueryLimit(r.URL.Query().Get("jobs_limit"), defaultAdminDashboardJobsLimit, maxAdminDashboardJobsLimit)

	stats, err := s.authService.GetUserStats(r.Context())
	if err != nil {
		s.logger.Printf("failed to get admin dashboard stats err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard")
		return
	}

	users, total, err := s.authService.ListUsers(r.Context(), usersLimit, 0)
	if err != nil {
		s.logger.Printf("failed to list admin dashboard users err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard")
		return
	}

	jobItems, err := s.jobStore.ListRecent(r.Context(), jobsLimit)
	if err != nil {
		s.logger.Printf("failed to list admin dashboard jobs err=%v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard")
		return
	}

	maintenanceSnapshot, err := s.maintenanceService.Get(r.Context())
	if err != nil {
		s.logger.Printf("failed to get admin dashboard maintenance err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, adminDashboardSnapshotResponse{
		Stats: stats,
		Users: adminDashboardUsersResponse{
			Items: users,
			Page: adminDashboardUsersPage{
				Total:  total,
				Limit:  usersLimit,
				Offset: 0,
			},
		},
		Jobs: adminDashboardJobsResponse{
			Items: jobItems,
		},
		Maintenance: toMaintenanceSnapshotResponse(maintenanceSnapshot),
	})
}

func parsePositiveQueryLimit(raw string, fallback, max int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	if parsed < 1 {
		return fallback
	}
	if parsed > max {
		return max
	}

	return parsed
}
