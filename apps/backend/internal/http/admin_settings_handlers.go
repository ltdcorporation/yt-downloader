package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"yt-downloader/backend/internal/adminsettings"
	"yt-downloader/backend/internal/auth"
)

type adminSettingsResponse struct {
	Settings adminSettingsResponseSettings `json:"settings"`
	Meta     adminSettingsResponseMeta     `json:"meta"`
}

type adminSettingsResponseSettings struct {
	Preferences   adminSettingsResponsePreferences   `json:"preferences"`
	Notifications adminSettingsResponseNotifications `json:"notifications"`
}

type adminSettingsResponsePreferences struct {
	DefaultQuality      adminsettings.Quality `json:"default_quality"`
	AutoTrimSilence     bool                  `json:"auto_trim_silence"`
	ThumbnailGeneration bool                  `json:"thumbnail_generation"`
}

type adminSettingsResponseNotifications struct {
	Email adminSettingsResponseEmailNotifications `json:"email"`
}

type adminSettingsResponseEmailNotifications struct {
	Processing bool `json:"processing"`
	Storage    bool `json:"storage"`
	Summary    bool `json:"summary"`
}

type adminSettingsResponseMeta struct {
	Version         int64     `json:"version"`
	UpdatedAt       time.Time `json:"updated_at"`
	UpdatedByUserID string    `json:"updated_by_user_id,omitempty"`
}

type adminSettingsPatchRequest struct {
	Settings *adminSettingsPatchPayload `json:"settings"`
	Meta     *adminSettingsPatchMeta    `json:"meta"`
}

type adminSettingsPatchMeta struct {
	Version int64 `json:"version"`
}

type adminSettingsPatchPayload struct {
	Preferences   *adminSettingsPatchPreferences   `json:"preferences"`
	Notifications *adminSettingsPatchNotifications `json:"notifications"`
}

type adminSettingsPatchPreferences struct {
	DefaultQuality      *string `json:"default_quality"`
	AutoTrimSilence     *bool   `json:"auto_trim_silence"`
	ThumbnailGeneration *bool   `json:"thumbnail_generation"`
}

type adminSettingsPatchNotifications struct {
	Email *adminSettingsPatchEmailNotifications `json:"email"`
}

type adminSettingsPatchEmailNotifications struct {
	Processing *bool `json:"processing"`
	Storage    *bool `json:"storage"`
	Summary    *bool `json:"summary"`
}

func (s *Server) handleAdminSettingsGet(w http.ResponseWriter, r *http.Request) {
	if s.adminSettingsService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin settings service unavailable",
			"code":  "admin_settings_unavailable",
		})
		return
	}

	snapshot, err := s.adminSettingsService.Get(r.Context())
	if err != nil {
		s.logger.Printf("admin settings get failed err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin settings service unavailable",
			"code":  "admin_settings_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toAdminSettingsResponse(snapshot))
}

func (s *Server) handleAdminSettingsPatch(w http.ResponseWriter, r *http.Request) {
	if s.adminSettingsService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin settings service unavailable",
			"code":  "admin_settings_unavailable",
		})
		return
	}

	var req adminSettingsPatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
			"code":  "admin_settings_invalid_request",
		})
		return
	}

	if req.Settings == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "settings is required",
			"code":  "admin_settings_invalid_request",
		})
		return
	}
	if req.Meta == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "meta is required",
			"code":  "admin_settings_invalid_request",
		})
		return
	}

	patch, err := mapAdminSettingsPatchRequest(req.Settings)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
			"code":  "admin_settings_invalid_request",
		})
		return
	}

	actorUserID := ""
	if identity, ok := r.Context().Value("identity").(auth.SessionIdentity); ok {
		actorUserID = identity.User.ID
	}

	snapshot, err := s.adminSettingsService.Patch(r.Context(), adminsettings.PatchInput{
		ExpectedVersion: req.Meta.Version,
		Patch:           patch,
		ActorUserID:     actorUserID,
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Source:          "admin_web",
	})
	if err != nil {
		var validationErr *adminsettings.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": validationErr.Error(),
				"code":  "admin_settings_invalid_request",
			})
			return
		}

		var versionConflictErr *adminsettings.VersionConflictError
		if errors.As(err, &versionConflictErr) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "admin settings version conflict",
				"code":  "admin_settings_version_conflict",
				"meta": map[string]any{
					"current_version": versionConflictErr.CurrentVersion,
				},
			})
			return
		}

		s.logger.Printf("admin settings patch failed err=%v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin settings service unavailable",
			"code":  "admin_settings_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toAdminSettingsResponse(snapshot))
}

func mapAdminSettingsPatchRequest(req *adminSettingsPatchPayload) (adminsettings.Patch, error) {
	patch := adminsettings.Patch{}

	if req.Preferences != nil {
		prefPatch := &adminsettings.PreferencesPatch{}
		if req.Preferences.DefaultQuality != nil {
			value := adminsettings.Quality(strings.TrimSpace(strings.ToLower(*req.Preferences.DefaultQuality)))
			prefPatch.DefaultQuality = &value
		}
		if req.Preferences.AutoTrimSilence != nil {
			value := *req.Preferences.AutoTrimSilence
			prefPatch.AutoTrimSilence = &value
		}
		if req.Preferences.ThumbnailGeneration != nil {
			value := *req.Preferences.ThumbnailGeneration
			prefPatch.ThumbnailGeneration = &value
		}
		patch.Preferences = prefPatch
	}

	if req.Notifications != nil {
		notifPatch := &adminsettings.NotificationsPatch{}
		if req.Notifications.Email != nil {
			emailPatch := &adminsettings.EmailNotificationsPatch{}
			if req.Notifications.Email.Processing != nil {
				value := *req.Notifications.Email.Processing
				emailPatch.Processing = &value
			}
			if req.Notifications.Email.Storage != nil {
				value := *req.Notifications.Email.Storage
				emailPatch.Storage = &value
			}
			if req.Notifications.Email.Summary != nil {
				value := *req.Notifications.Email.Summary
				emailPatch.Summary = &value
			}
			notifPatch.Email = emailPatch
		}
		patch.Notifications = notifPatch
	}

	if !adminSettingsPatchHasAnyField(patch) {
		return adminsettings.Patch{}, errors.New("settings patch must include at least one field")
	}

	return patch, nil
}

func adminSettingsPatchHasAnyField(patch adminsettings.Patch) bool {
	if patch.Preferences != nil {
		if patch.Preferences.DefaultQuality != nil ||
			patch.Preferences.AutoTrimSilence != nil ||
			patch.Preferences.ThumbnailGeneration != nil {
			return true
		}
	}

	if patch.Notifications != nil && patch.Notifications.Email != nil {
		emailPatch := patch.Notifications.Email
		if emailPatch.Processing != nil || emailPatch.Storage != nil || emailPatch.Summary != nil {
			return true
		}
	}

	return false
}

func toAdminSettingsResponse(snapshot adminsettings.Snapshot) adminSettingsResponse {
	return adminSettingsResponse{
		Settings: adminSettingsResponseSettings{
			Preferences: adminSettingsResponsePreferences{
				DefaultQuality:      snapshot.Data.Preferences.DefaultQuality,
				AutoTrimSilence:     snapshot.Data.Preferences.AutoTrimSilence,
				ThumbnailGeneration: snapshot.Data.Preferences.ThumbnailGeneration,
			},
			Notifications: adminSettingsResponseNotifications{
				Email: adminSettingsResponseEmailNotifications{
					Processing: snapshot.Data.Notifications.Email.Processing,
					Storage:    snapshot.Data.Notifications.Email.Storage,
					Summary:    snapshot.Data.Notifications.Email.Summary,
				},
			},
		},
		Meta: adminSettingsResponseMeta{
			Version:         snapshot.Version,
			UpdatedAt:       snapshot.UpdatedAt.UTC(),
			UpdatedByUserID: snapshot.UpdatedByUserID,
		},
	}
}
