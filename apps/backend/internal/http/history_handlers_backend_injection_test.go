package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"yt-downloader/backend/internal/history"
)

type historyBackendStub struct {
	listItemsFn              func(ctx context.Context, userID string, filter history.ListFilter) (history.ListPage, error)
	getStatsFn               func(ctx context.Context, userID string) (history.Stats, error)
	softDeleteItemFn         func(ctx context.Context, userID, itemID string, deletedAt time.Time) error
	getItemByIDFn            func(ctx context.Context, userID, itemID string) (history.Item, error)
	getLatestAttemptByItemFn func(ctx context.Context, userID, itemID string) (history.Attempt, error)
}

func (s *historyBackendStub) Close() error                      { return nil }
func (s *historyBackendStub) EnsureReady(context.Context) error { return nil }
func (s *historyBackendStub) UpsertItem(context.Context, history.Item) (history.Item, error) {
	return history.Item{}, nil
}
func (s *historyBackendStub) GetItemByID(ctx context.Context, userID, itemID string) (history.Item, error) {
	if s.getItemByIDFn != nil {
		return s.getItemByIDFn(ctx, userID, itemID)
	}
	return history.Item{}, history.ErrItemNotFound
}
func (s *historyBackendStub) SoftDeleteItem(ctx context.Context, userID, itemID string, deletedAt time.Time) error {
	if s.softDeleteItemFn != nil {
		return s.softDeleteItemFn(ctx, userID, itemID, deletedAt)
	}
	return nil
}
func (s *historyBackendStub) MarkItemSuccess(context.Context, string, string, time.Time) error {
	return nil
}
func (s *historyBackendStub) CreateAttempt(context.Context, history.Attempt) error { return nil }
func (s *historyBackendStub) UpdateAttempt(context.Context, history.Attempt) error { return nil }
func (s *historyBackendStub) GetAttemptByID(context.Context, string, string) (history.Attempt, error) {
	return history.Attempt{}, history.ErrAttemptNotFound
}
func (s *historyBackendStub) GetAttemptByJobID(context.Context, string) (history.Attempt, error) {
	return history.Attempt{}, history.ErrAttemptNotFound
}
func (s *historyBackendStub) GetLatestAttemptByItem(ctx context.Context, userID, itemID string) (history.Attempt, error) {
	if s.getLatestAttemptByItemFn != nil {
		return s.getLatestAttemptByItemFn(ctx, userID, itemID)
	}
	return history.Attempt{}, history.ErrAttemptNotFound
}
func (s *historyBackendStub) ListItems(ctx context.Context, userID string, filter history.ListFilter) (history.ListPage, error) {
	if s.listItemsFn != nil {
		return s.listItemsFn(ctx, userID, filter)
	}
	return history.ListPage{}, nil
}
func (s *historyBackendStub) GetStats(ctx context.Context, userID string) (history.Stats, error) {
	if s.getStatsFn != nil {
		return s.getStatsFn(ctx, userID)
	}
	return history.Stats{}, nil
}

func setHistoryStoreBackendForTest(t *testing.T, store *history.Store, backend any) {
	t.Helper()
	if store == nil {
		t.Fatalf("store is nil")
	}
	if backend == nil {
		t.Fatalf("backend is nil")
	}

	field := reflect.ValueOf(store).Elem().FieldByName("backend")
	if !field.IsValid() {
		t.Fatalf("history.Store.backend field not found")
	}

	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(backend))
}

func TestHistoryHandlers_InvalidInputAndFallbackBranchesViaInjectedBackend(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	t.Run("list invalid input branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			listItemsFn: func(context.Context, string, history.ListFilter) (history.ListPage, error) {
				return history.ListPage{}, history.ErrInvalidInput
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodGet, "/v1/history", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_invalid_request" {
			t.Fatalf("unexpected code for list invalid input branch: %#v", payload["code"])
		}
	})

	t.Run("stats invalid input branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			getStatsFn: func(context.Context, string) (history.Stats, error) {
				return history.Stats{}, history.ErrInvalidInput
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodGet, "/v1/history/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_invalid_request" {
			t.Fatalf("unexpected code for stats invalid input branch: %#v", payload["code"])
		}
	})

	t.Run("delete invalid input branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			softDeleteItemFn: func(context.Context, string, string, time.Time) error {
				return history.ErrInvalidInput
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodDelete, "/v1/history/his_invalid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_invalid_request" {
			t.Fatalf("unexpected code for delete invalid input branch: %#v", payload["code"])
		}
	})

	t.Run("redownload get item invalid input branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			getItemByIDFn: func(context.Context, string, string) (history.Item, error) {
				return history.Item{}, history.ErrInvalidInput
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodPost, "/v1/history/his_invalid/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "history_invalid_request" {
			t.Fatalf("unexpected code for redownload get item invalid input branch: %#v", payload["code"])
		}
	})

	t.Run("redownload latest attempt not found branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			getItemByIDFn: func(context.Context, string, string) (history.Item, error) {
				return history.Item{ID: "his_1", UserID: userID, Platform: history.PlatformYouTube, SourceURL: "https://youtube.com/watch?v=1"}, nil
			},
			getLatestAttemptByItemFn: func(context.Context, string, string) (history.Attempt, error) {
				return history.Attempt{}, history.ErrItemNotFound
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodPost, "/v1/history/his_1/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload latest attempt internal error branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			getItemByIDFn: func(context.Context, string, string) (history.Item, error) {
				return history.Item{ID: "his_1", UserID: userID, Platform: history.PlatformYouTube, SourceURL: "https://youtube.com/watch?v=1"}, nil
			},
			getLatestAttemptByItemFn: func(context.Context, string, string) (history.Attempt, error) {
				return history.Attempt{}, errors.New("backend exploded")
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodPost, "/v1/history/his_1/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("redownload unsupported request kind default switch branch", func(t *testing.T) {
		store := &history.Store{}
		setHistoryStoreBackendForTest(t, store, &historyBackendStub{
			getItemByIDFn: func(context.Context, string, string) (history.Item, error) {
				return history.Item{ID: "his_1", UserID: userID, Platform: history.PlatformYouTube, SourceURL: "https://youtube.com/watch?v=1"}, nil
			},
			getLatestAttemptByItemFn: func(context.Context, string, string) (history.Attempt, error) {
				return history.Attempt{ID: "hat_1", UserID: userID, HistoryItemID: "his_1", RequestKind: history.RequestKind("unsupported")}, nil
			},
		})
		server.historyStore = store

		req := httptest.NewRequest(http.MethodPost, "/v1/history/his_1/redownload", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}
