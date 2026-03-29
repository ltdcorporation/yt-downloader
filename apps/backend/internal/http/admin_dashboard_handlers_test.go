package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yt-downloader/backend/internal/jobs"
)

func TestParsePositiveQueryLimit(t *testing.T) {
	const (
		fallback = 8
		max      = 100
	)

	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "empty uses fallback", raw: "", want: fallback},
		{name: "spaces uses fallback", raw: "   ", want: fallback},
		{name: "invalid uses fallback", raw: "abc", want: fallback},
		{name: "zero uses fallback", raw: "0", want: fallback},
		{name: "negative uses fallback", raw: "-12", want: fallback},
		{name: "valid", raw: "25", want: 25},
		{name: "clamped to max", raw: "250", want: max},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePositiveQueryLimit(tt.raw, fallback, max)
			if got != tt.want {
				t.Fatalf("unexpected parsed limit raw=%q got=%d want=%d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestHandleAdminDashboard(t *testing.T) {
	cfg := baseTestConfig()
	store := newFakeJobStore()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, store)

	for i := 0; i < 2; i++ {
		_, _ = registerUserAndGetToken(t, server)
	}

	now := time.Now().UTC()
	jobID := fmt.Sprintf("job_dash_%d", now.UnixNano())
	if err := store.Put(context.Background(), jobs.Record{
		ID:         jobID,
		Status:     jobs.StatusDone,
		InputURL:   "https://youtube.com/watch?v=dummy",
		OutputKind: "mp3",
		Title:      "Dashboard Job",
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatalf("failed to seed job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard?users_limit=5&jobs_limit=7", nil)
	req.SetBasicAuth(cfg.AdminBasicAuthUser, cfg.AdminBasicAuthPass)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin dashboard, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec.Body.Bytes())

	stats, ok := payload["stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected stats object, got %#v", payload["stats"])
	}
	if _, ok := stats["total_users"].(float64); !ok {
		t.Fatalf("expected stats.total_users number, got %#v", stats["total_users"])
	}

	users, ok := payload["users"].(map[string]any)
	if !ok {
		t.Fatalf("expected users object, got %#v", payload["users"])
	}
	page, ok := users["page"].(map[string]any)
	if !ok {
		t.Fatalf("expected users.page object, got %#v", users["page"])
	}
	if gotLimit, ok := page["limit"].(float64); !ok || int(gotLimit) != 5 {
		t.Fatalf("expected users.page.limit=5, got %#v", page["limit"])
	}

	jobsPayload, ok := payload["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("expected jobs object, got %#v", payload["jobs"])
	}
	jobItems, ok := jobsPayload["items"].([]any)
	if !ok || len(jobItems) == 0 {
		t.Fatalf("expected non-empty jobs.items, got %#v", jobsPayload["items"])
	}

	maintenancePayload, ok := payload["maintenance"].(map[string]any)
	if !ok {
		t.Fatalf("expected maintenance object, got %#v", payload["maintenance"])
	}
	if _, ok := maintenancePayload["maintenance"].(map[string]any); !ok {
		t.Fatalf("expected maintenance.maintenance object, got %#v", maintenancePayload["maintenance"])
	}
}

func TestHandleAdminDashboard_RequiresAdminAuth(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing admin auth, got %d body=%s", rec.Code, rec.Body.String())
	}
}
