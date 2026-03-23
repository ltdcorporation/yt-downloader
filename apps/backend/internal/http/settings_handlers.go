package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/settings"
)

type settingsResponse struct {
	Settings settingsResponseSettings `json:"settings"`
	Meta     settingsResponseMeta     `json:"meta"`
}

type settingsResponseSettings struct {
	Preferences   settingsResponsePreferences   `json:"preferences"`
	Notifications settingsResponseNotifications `json:"notifications"`
}

type settingsResponsePreferences struct {
	DefaultQuality      settings.Quality `json:"default_quality"`
	AutoTrimSilence     bool             `json:"auto_trim_silence"`
	ThumbnailGeneration bool             `json:"thumbnail_generation"`
}

type settingsResponseNotifications struct {
	Email settingsResponseEmailNotifications `json:"email"`
}

type settingsResponseEmailNotifications struct {
	Processing bool `json:"processing"`
	Storage    bool `json:"storage"`
	Summary    bool `json:"summary"`
}

type settingsResponseMeta struct {
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

type settingsPatchRequest struct {
	Settings *settingsPatchPayload `json:"settings"`
	Meta     *settingsPatchMeta    `json:"meta"`
}

type settingsPatchMeta struct {
	Version int64 `json:"version"`
}

type settingsPatchPayload struct {
	Preferences   *settingsPatchPreferences   `json:"preferences"`
	Notifications *settingsPatchNotifications `json:"notifications"`
}

type settingsPatchPreferences struct {
	DefaultQuality      *string `json:"default_quality"`
	AutoTrimSilence     *bool   `json:"auto_trim_silence"`
	ThumbnailGeneration *bool   `json:"thumbnail_generation"`
}

type settingsPatchNotifications struct {
	Email *settingsPatchEmailNotifications `json:"email"`
}

type settingsPatchEmailNotifications struct {
	Processing *bool `json:"processing"`
	Storage    *bool `json:"storage"`
	Summary    *bool `json:"summary"`
}

type profileResponse struct {
	Profile auth.PublicUser `json:"profile"`
}

type profilePatchRequest struct {
	Profile *profilePatchPayload `json:"profile"`
}

type profilePatchPayload struct {
	FullName string `json:"full_name"`
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	if s.settingsService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "settings service unavailable",
			"code":  "settings_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	snapshot, err := s.settingsService.Get(r.Context(), identity.User.ID)
	if err != nil {
		s.logger.Printf("settings get failed user_id=%s err=%v", identity.User.ID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "settings service unavailable",
			"code":  "settings_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toSettingsResponse(snapshot))
}

func (s *Server) handleSettingsPatch(w http.ResponseWriter, r *http.Request) {
	if s.settingsService == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "settings service unavailable",
			"code":  "settings_unavailable",
		})
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	var req settingsPatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
			"code":  "settings_invalid_request",
		})
		return
	}

	if req.Settings == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "settings is required",
			"code":  "settings_invalid_request",
		})
		return
	}
	if req.Meta == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "meta is required",
			"code":  "settings_invalid_request",
		})
		return
	}

	patch, err := mapSettingsPatchRequest(req.Settings)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
			"code":  "settings_invalid_request",
		})
		return
	}

	snapshot, err := s.settingsService.Patch(r.Context(), settings.PatchInput{
		UserID:          identity.User.ID,
		ExpectedVersion: req.Meta.Version,
		Patch:           patch,
		ActorUserID:     identity.User.ID,
		RequestID:       strings.TrimSpace(r.Header.Get("X-Request-ID")),
		Source:          "web",
	})
	if err != nil {
		var validationErr *settings.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": validationErr.Error(),
				"code":  "settings_invalid_request",
			})
			return
		}

		var versionConflictErr *settings.VersionConflictError
		if errors.As(err, &versionConflictErr) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "settings version conflict",
				"code":  "settings_version_conflict",
				"meta": map[string]any{
					"current_version": versionConflictErr.CurrentVersion,
				},
			})
			return
		}

		s.logger.Printf("settings patch failed user_id=%s err=%v", identity.User.ID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "settings service unavailable",
			"code":  "settings_unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, toSettingsResponse(snapshot))
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, profileResponse{Profile: identity.User})
}

func (s *Server) handleProfilePatch(w http.ResponseWriter, r *http.Request) {
	if s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	var req profilePatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
			"code":  "profile_invalid_request",
		})
		return
	}
	if req.Profile == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "profile is required",
			"code":  "profile_invalid_request",
		})
		return
	}

	updatedProfile, err := s.authService.UpdateProfile(r.Context(), identity.User.ID, auth.UpdateProfileInput{
		FullName: req.Profile.FullName,
	})
	if err != nil {
		var validationErr *auth.ValidationError
		if errors.As(err, &validationErr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": validationErr.Error(),
				"code":  "profile_invalid_request",
			})
			return
		}
		s.logger.Printf("profile patch failed user_id=%s err=%v", identity.User.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, profileResponse{Profile: updatedProfile})
}

func mapSettingsPatchRequest(req *settingsPatchPayload) (settings.Patch, error) {
	patch := settings.Patch{}

	if req.Preferences != nil {
		prefPatch := &settings.PreferencesPatch{}
		if req.Preferences.DefaultQuality != nil {
			value := settings.Quality(strings.TrimSpace(strings.ToLower(*req.Preferences.DefaultQuality)))
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
		notifPatch := &settings.NotificationsPatch{}
		if req.Notifications.Email != nil {
			emailPatch := &settings.EmailNotificationsPatch{}
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

	if !settingsPatchHasAnyField(patch) {
		return settings.Patch{}, errors.New("settings patch must include at least one field")
	}

	return patch, nil
}

func settingsPatchHasAnyField(patch settings.Patch) bool {
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

func toSettingsResponse(snapshot settings.Snapshot) settingsResponse {
	return settingsResponse{
		Settings: settingsResponseSettings{
			Preferences: settingsResponsePreferences{
				DefaultQuality:      snapshot.Data.Preferences.DefaultQuality,
				AutoTrimSilence:     snapshot.Data.Preferences.AutoTrimSilence,
				ThumbnailGeneration: snapshot.Data.Preferences.ThumbnailGeneration,
			},
			Notifications: settingsResponseNotifications{
				Email: settingsResponseEmailNotifications{
					Processing: snapshot.Data.Notifications.Email.Processing,
					Storage:    snapshot.Data.Notifications.Email.Storage,
					Summary:    snapshot.Data.Notifications.Email.Summary,
				},
			},
		},
		Meta: settingsResponseMeta{
			Version:   snapshot.Version,
			UpdatedAt: snapshot.UpdatedAt.UTC(),
		},
	}
}
