package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yt-downloader/backend/internal/settings"
)

func TestSettingsPatchHelpers(t *testing.T) {
	t.Run("map request normalizes values", func(t *testing.T) {
		quality := "  720P  "
		autoTrim := true
		summary := true

		patch, err := mapSettingsPatchRequest(&settingsPatchPayload{
			Preferences: &settingsPatchPreferences{
				DefaultQuality:  &quality,
				AutoTrimSilence: &autoTrim,
			},
			Notifications: &settingsPatchNotifications{
				Email: &settingsPatchEmailNotifications{Summary: &summary},
			},
		})
		if err != nil {
			t.Fatalf("expected valid settings patch mapping, got err=%v", err)
		}
		if patch.Preferences == nil || patch.Preferences.DefaultQuality == nil {
			t.Fatalf("expected preferences.default_quality to be present")
		}
		if *patch.Preferences.DefaultQuality != settings.Quality720p {
			t.Fatalf("expected normalized quality=720p, got %q", *patch.Preferences.DefaultQuality)
		}
		if patch.Notifications == nil || patch.Notifications.Email == nil || patch.Notifications.Email.Summary == nil || !*patch.Notifications.Email.Summary {
			t.Fatalf("expected notifications.email.summary=true, got %#v", patch.Notifications)
		}
	})

	t.Run("empty patch rejected", func(t *testing.T) {
		_, err := mapSettingsPatchRequest(&settingsPatchPayload{})
		if err == nil {
			t.Fatalf("expected empty patch error")
		}
	})

	t.Run("settingsPatchHasAnyField", func(t *testing.T) {
		if settingsPatchHasAnyField(settings.Patch{}) {
			t.Fatalf("expected no fields in empty patch")
		}

		autoTrim := true
		if !settingsPatchHasAnyField(settings.Patch{
			Preferences: &settings.PreferencesPatch{AutoTrimSilence: &autoTrim},
		}) {
			t.Fatalf("expected patch with preferences field to be detected")
		}

		emailSummary := false
		if !settingsPatchHasAnyField(settings.Patch{
			Notifications: &settings.NotificationsPatch{
				Email: &settings.EmailNotificationsPatch{Summary: &emailSummary},
			},
		}) {
			t.Fatalf("expected patch with notification field to be detected")
		}
	})
}

func TestSettingsHandlers_ErrorScenarios(t *testing.T) {
	t.Run("settings service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.settingsService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/settings", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when settings service missing, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "settings_unavailable" {
			t.Fatalf("expected settings_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("settings patch validation branches", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		cases := []struct {
			name string
			body string
			want int
		}{
			{name: "invalid json", body: `{`, want: http.StatusBadRequest},
			{name: "missing settings", body: `{"meta":{"version":1}}`, want: http.StatusBadRequest},
			{name: "missing meta", body: `{"settings":{"preferences":{"default_quality":"720p"}}}`, want: http.StatusBadRequest},
			{name: "empty patch", body: `{"settings":{},"meta":{"version":1}}`, want: http.StatusBadRequest},
			{name: "invalid quality", body: `{"settings":{"preferences":{"default_quality":"8k"}},"meta":{"version":1}}`, want: http.StatusBadRequest},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPatch, "/v1/settings", bytes.NewBufferString(tc.body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("unexpected status for %s got=%d want=%d body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
				}
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != "settings_invalid_request" {
					t.Fatalf("expected settings_invalid_request code, got %#v", payload["code"])
				}
			})
		}
	})
}

func TestProfileHandlers_ErrorScenarios(t *testing.T) {
	t.Run("auth service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.authService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when auth service missing, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("profile patch invalid payload branches", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		token, _ := registerUserAndGetToken(t, server)

		invalidJSONReq := httptest.NewRequest(http.MethodPatch, "/v1/profile", bytes.NewBufferString(`{`))
		invalidJSONReq.Header.Set("Authorization", "Bearer "+token)
		invalidJSONReq.Header.Set("Content-Type", "application/json")
		invalidJSONRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(invalidJSONRec, invalidJSONReq)
		if invalidJSONRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid profile patch JSON, got %d body=%s", invalidJSONRec.Code, invalidJSONRec.Body.String())
		}
		invalidPayload := decodeJSONMap(t, invalidJSONRec.Body.Bytes())
		if invalidPayload["code"] != "profile_invalid_request" {
			t.Fatalf("expected profile_invalid_request code, got %#v", invalidPayload["code"])
		}

		missingProfileBody, _ := json.Marshal(map[string]any{"meta": map[string]any{"x": 1}})
		missingProfileReq := httptest.NewRequest(http.MethodPatch, "/v1/profile", bytes.NewReader(missingProfileBody))
		missingProfileReq.Header.Set("Authorization", "Bearer "+token)
		missingProfileReq.Header.Set("Content-Type", "application/json")
		missingProfileRec := httptest.NewRecorder()
		server.Handler().ServeHTTP(missingProfileRec, missingProfileReq)
		if missingProfileRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when profile payload missing, got %d body=%s", missingProfileRec.Code, missingProfileRec.Body.String())
		}
		missingPayload := decodeJSONMap(t, missingProfileRec.Body.Bytes())
		if missingPayload["code"] != "profile_invalid_request" {
			t.Fatalf("expected profile_invalid_request code for missing profile payload, got %#v", missingPayload["code"])
		}
	})
}
