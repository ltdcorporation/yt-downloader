package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
)

type adminUserPatchRequest struct {
	FullName      *string `json:"full_name"`
	Role          *string `json:"role"`
	Plan          *string `json:"plan"`
	PlanExpiresAt *string `json:"plan_expires_at"`
}

func (s *Server) handleAdminUserGet(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	user, err := s.authService.GetUser(r.Context(), userID)
	if err != nil {
		var validationErr *auth.ValidationError
		switch {
		case errors.As(err, &validationErr):
			writeError(w, http.StatusBadRequest, validationErr.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user not found")
		default:
			s.logger.Printf("failed to get admin user detail user_id=%s err=%v", userID, err)
			writeError(w, http.StatusInternalServerError, "failed to fetch user")
		}
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleAdminUserPatch(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	userID := strings.TrimSpace(chi.URLParam(r, "id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	var req adminUserPatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	input, err := parseAdminUserPatchRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	actorUserID := ""
	if identity, ok := r.Context().Value("identity").(auth.SessionIdentity); ok {
		actorUserID = identity.User.ID
	}

	updatedUser, err := s.authService.UpdateUserByAdmin(r.Context(), actorUserID, userID, input)
	if err != nil {
		var validationErr *auth.ValidationError
		switch {
		case errors.As(err, &validationErr):
			writeError(w, http.StatusBadRequest, validationErr.Error())
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user not found")
		default:
			s.logger.Printf("failed to patch admin user user_id=%s err=%v", userID, err)
			writeError(w, http.StatusInternalServerError, "failed to update user")
		}
		return
	}

	writeJSON(w, http.StatusOK, updatedUser)
}

func parseAdminUserPatchRequest(req adminUserPatchRequest) (auth.AdminUpdateUserInput, error) {
	input := auth.AdminUpdateUserInput{}

	if req.FullName != nil {
		value := strings.TrimSpace(*req.FullName)
		input.FullName = &value
	}

	if req.Role != nil {
		switch strings.ToLower(strings.TrimSpace(*req.Role)) {
		case string(auth.RoleAdmin):
			role := auth.RoleAdmin
			input.Role = &role
		case string(auth.RoleUser):
			role := auth.RoleUser
			input.Role = &role
		default:
			return auth.AdminUpdateUserInput{}, fmt.Errorf("role must be one of: %s, %s", auth.RoleAdmin, auth.RoleUser)
		}
	}

	if req.Plan != nil {
		switch strings.ToLower(strings.TrimSpace(*req.Plan)) {
		case string(auth.PlanFree):
			plan := auth.PlanFree
			input.Plan = &plan
		case string(auth.PlanDaily):
			plan := auth.PlanDaily
			input.Plan = &plan
		case string(auth.PlanWeekly):
			plan := auth.PlanWeekly
			input.Plan = &plan
		case string(auth.PlanMonthly):
			plan := auth.PlanMonthly
			input.Plan = &plan
		default:
			return auth.AdminUpdateUserInput{}, fmt.Errorf("plan must be one of: %s, %s, %s, %s", auth.PlanFree, auth.PlanDaily, auth.PlanWeekly, auth.PlanMonthly)
		}
	}

	if req.PlanExpiresAt != nil {
		input.PlanExpiresAtSet = true
		value := strings.TrimSpace(*req.PlanExpiresAt)
		if value == "" {
			input.PlanExpiresAt = nil
		} else {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return auth.AdminUpdateUserInput{}, errors.New("plan_expires_at must be RFC3339 timestamp")
			}
			parsedUTC := parsed.UTC()
			input.PlanExpiresAt = &parsedUTC
		}
	}

	if input.FullName == nil && input.Role == nil && input.Plan == nil && !input.PlanExpiresAtSet {
		return auth.AdminUpdateUserInput{}, errors.New("at least one field must be provided")
	}

	return input, nil
}
