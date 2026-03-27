package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/maintenance"
)

type maintenanceSnapshotResponse struct {
	Maintenance maintenanceResponseData `json:"maintenance"`
	Meta        maintenanceResponseMeta `json:"meta"`
}

type maintenanceResponseData struct {
	Enabled           bool                             `json:"enabled"`
	EstimatedDowntime string                           `json:"estimated_downtime"`
	PublicMessage     string                           `json:"public_message"`
	Services          []maintenanceResponseServiceItem `json:"services"`
}

type maintenanceResponseServiceItem struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

type maintenanceResponseMeta struct {
	Version         int64  `json:"version"`
	UpdatedAt       string `json:"updated_at"`
	UpdatedByUserID string `json:"updated_by_user_id,omitempty"`
}

type maintenancePatchRequest struct {
	Maintenance *maintenancePatchPayload `json:"maintenance"`
	Meta        *maintenancePatchMeta    `json:"meta"`
}

type maintenancePatchMeta struct {
	Version int64 `json:"version"`
}

type maintenancePatchPayload struct {
	Enabled           *bool                         `json:"enabled"`
	EstimatedDowntime *string                       `json:"estimated_downtime"`
	PublicMessage     *string                       `json:"public_message"`
	Services          []maintenanceServicePatchItem `json:"services"`
}

type maintenanceServicePatchItem struct {
	Key     string  `json:"key"`
	Status  *string `json:"status"`
	Enabled *bool   `json:"enabled"`
}

func (s *Server) handleMaintenancePublicGet(w http.ResponseWriter, r *http.Request) {
	if s.maintenanceService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	snapshot, err := s.maintenanceService.Get(r.Context())
	if err != nil {
		s.logger.Printf("maintenance get public failed err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toMaintenanceSnapshotResponse(snapshot))
}

func (s *Server) handleAdminMaintenanceGet(w http.ResponseWriter, r *http.Request) {
	if s.maintenanceService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	snapshot, err := s.maintenanceService.Get(r.Context())
	if err != nil {
		s.logger.Printf("maintenance admin get failed err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toMaintenanceSnapshotResponse(snapshot))
}

func (s *Server) handleAdminMaintenancePatch(w http.ResponseWriter, r *http.Request) {
	if s.maintenanceService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	var req maintenancePatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
			"code":  "maintenance_invalid_request",
		})
		return
	}

	if req.Meta == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "meta is required",
			"code":  "maintenance_invalid_request",
		})
		return
	}
	if req.Maintenance == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "maintenance is required",
			"code":  "maintenance_invalid_request",
		})
		return
	}

	patch, err := mapMaintenancePatch(req.Maintenance)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
			"code":  "maintenance_invalid_request",
		})
		return
	}

	actorUserID := ""
	if identity, ok := r.Context().Value("identity").(auth.SessionIdentity); ok {
		actorUserID = identity.User.ID
	}

	snapshot, err := s.maintenanceService.Patch(r.Context(), maintenance.PatchInput{
		ExpectedVersion: req.Meta.Version,
		Patch:           patch,
		ActorUserID:     actorUserID,
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Source:          "admin_web",
	})
	if err != nil {
		var validationErr *maintenance.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": validationErr.Error(),
				"code":  "maintenance_invalid_request",
			})
			return
		}

		var conflictErr *maintenance.VersionConflictError
		if errors.As(err, &conflictErr) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "maintenance version conflict",
				"code":  "maintenance_version_conflict",
				"meta": map[string]any{
					"current_version": conflictErr.CurrentVersion,
				},
			})
			return
		}

		s.logger.Printf("maintenance admin patch failed err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance service unavailable",
			"code":  "maintenance_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toMaintenanceSnapshotResponse(snapshot))
}

func (s *Server) maintenanceGuardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.maintenanceService == nil {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		if maintenancePathBypass(path) {
			next.ServeHTTP(w, r)
			return
		}

		snapshot, err := s.maintenanceService.Get(r.Context())
		if err != nil {
			// fail-open to avoid global outage when maintenance service is unhealthy
			s.logger.Printf("maintenance middleware fail-open err=%v", err)
			next.ServeHTTP(w, r)
			return
		}
		if !snapshot.Data.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "service unavailable: maintenance mode",
			"code":  "maintenance_mode",
			"maintenance": map[string]any{
				"enabled":            true,
				"estimated_downtime": snapshot.Data.EstimatedDowntime,
				"public_message":     snapshot.Data.PublicMessage,
			},
		})
	})
}

func maintenancePathBypass(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if path == "/healthz" || path == "/v1/maintenance" || path == "/v1/auth/me" {
		return true
	}
	if strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/v1/admin") {
		return true
	}
	return false
}

func mapMaintenancePatch(payload *maintenancePatchPayload) (maintenance.Patch, error) {
	patch := maintenance.Patch{}

	if payload.Enabled != nil {
		value := *payload.Enabled
		patch.Enabled = &value
	}
	if payload.EstimatedDowntime != nil {
		value := strings.TrimSpace(*payload.EstimatedDowntime)
		patch.EstimatedDowntime = &value
	}
	if payload.PublicMessage != nil {
		value := strings.TrimSpace(*payload.PublicMessage)
		patch.PublicMessage = &value
	}

	if len(payload.Services) > 0 {
		patch.Services = make([]maintenance.ServicePatch, 0, len(payload.Services))
		for _, item := range payload.Services {
			key := maintenance.ServiceKey(strings.TrimSpace(strings.ToLower(item.Key)))
			if !maintenance.IsValidServiceKey(key) {
				return maintenance.Patch{}, fmt.Errorf("unsupported service key: %s", key)
			}

			servicePatch := maintenance.ServicePatch{Key: key}
			if item.Status != nil {
				status := maintenance.ServiceStatus(strings.TrimSpace(strings.ToLower(*item.Status)))
				if !maintenance.IsValidServiceStatus(status) {
					return maintenance.Patch{}, fmt.Errorf("unsupported service status: %s", status)
				}
				servicePatch.Status = &status
			}
			if item.Enabled != nil {
				value := *item.Enabled
				servicePatch.Enabled = &value
			}
			if servicePatch.Status == nil && servicePatch.Enabled == nil {
				return maintenance.Patch{}, fmt.Errorf("service patch for %s must include status or enabled", key)
			}

			patch.Services = append(patch.Services, servicePatch)
		}
	}

	if patch.Enabled == nil && patch.EstimatedDowntime == nil && patch.PublicMessage == nil && len(patch.Services) == 0 {
		return maintenance.Patch{}, errors.New("maintenance patch must include at least one field")
	}

	return patch, nil
}

func toMaintenanceSnapshotResponse(snapshot maintenance.Snapshot) maintenanceSnapshotResponse {
	services := make([]maintenanceResponseServiceItem, 0, len(snapshot.Data.Services))
	for _, service := range snapshot.Data.Services {
		services = append(services, maintenanceResponseServiceItem{
			Key:     string(service.Key),
			Name:    service.Name,
			Status:  string(service.Status),
			Enabled: service.Enabled,
		})
	}

	return maintenanceSnapshotResponse{
		Maintenance: maintenanceResponseData{
			Enabled:           snapshot.Data.Enabled,
			EstimatedDowntime: snapshot.Data.EstimatedDowntime,
			PublicMessage:     snapshot.Data.PublicMessage,
			Services:          services,
		},
		Meta: maintenanceResponseMeta{
			Version:         snapshot.Version,
			UpdatedAt:       snapshot.UpdatedAt.UTC().Format(timeRFC3339Nano),
			UpdatedByUserID: snapshot.UpdatedByUserID,
		},
	}
}

const timeRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
