package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"yt-downloader/backend/internal/adminsettings"
)

func TestAdminSettingsPatchHelpers(t *testing.T) {
	t.Run("map request normalizes values", func(t *testing.T) {
		quality := "  4K  "
		processing := false

		patch, err := mapAdminSettingsPatchRequest(&adminSettingsPatchPayload{
			Preferences: &adminSettingsPatchPreferences{DefaultQuality: &quality},
			Notifications: &adminSettingsPatchNotifications{
				Email: &adminSettingsPatchEmailNotifications{Processing: &processing},
			},
		})
		if err != nil {
			t.Fatalf("expected valid admin settings patch mapping, got err=%v", err)
		}
		if patch.Preferences == nil || patch.Preferences.DefaultQuality == nil {
			t.Fatalf("expected preferences.default_quality to be present")
		}
		if *patch.Preferences.DefaultQuality != adminsettings.Quality4K {
			t.Fatalf("expected normalized quality=4k, got %q", *patch.Preferences.DefaultQuality)
		}
		if patch.Notifications == nil || patch.Notifications.Email == nil || patch.Notifications.Email.Processing == nil {
			t.Fatalf("expected notifications.email.processing to be present")
		}
		if *patch.Notifications.Email.Processing {
			t.Fatalf("expected processing=false in mapped patch")
		}
	})

	t.Run("empty patch rejected", func(t *testing.T) {
		_, err := mapAdminSettingsPatchRequest(&adminSettingsPatchPayload{})
		if err == nil {
			t.Fatalf("expected empty patch error")
		}
	})

	t.Run("adminSettingsPatchHasAnyField", func(t *testing.T) {
		if adminSettingsPatchHasAnyField(adminsettings.Patch{}) {
			t.Fatalf("expected no fields in empty patch")
		}

		autoTrim := true
		if !adminSettingsPatchHasAnyField(adminsettings.Patch{
			Preferences: &adminsettings.PreferencesPatch{AutoTrimSilence: &autoTrim},
		}) {
			t.Fatalf("expected patch with preferences field to be detected")
		}
	})
}

func TestAdminSettingsHandlers_ErrorScenarios(t *testing.T) {
	t.Run("admin settings service unavailable", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
		server.adminSettingsService = nil

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/settings", nil)
		req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when admin settings service missing, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "admin_settings_unavailable" {
			t.Fatalf("expected admin_settings_unavailable code, got %#v", payload["code"])
		}
	})

	t.Run("admin settings patch validation branches", func(t *testing.T) {
		cfg := baseTestConfig()
		server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

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
				req := httptest.NewRequest(http.MethodPatch, "/v1/admin/settings", bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
				req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
				rec := httptest.NewRecorder()
				server.Handler().ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("unexpected status for %s got=%d want=%d body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
				}
				payload := decodeJSONMap(t, rec.Body.Bytes())
				if payload["code"] != "admin_settings_invalid_request" {
					t.Fatalf("expected admin_settings_invalid_request code, got %#v", payload["code"])
				}
			})
		}
	})
}
