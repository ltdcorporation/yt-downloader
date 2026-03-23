package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
)

type historyCursorPayload struct {
	SortAt string `json:"sort_at"`
	ItemID string `json:"item_id"`
}

type historyRedownloadRequest struct {
	RequestKind string `json:"request_kind"`
	FormatID    string `json:"format_id"`
}

func (s *Server) requireSessionIdentity(w http.ResponseWriter, r *http.Request) (*auth.SessionIdentity, bool) {
	if s == nil || s.authService == nil {
		writeError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return nil, false
	}

	identity, err := s.authService.AuthenticateToken(r.Context(), s.readSessionToken(r))
	if err != nil {
		s.writeAuthSessionError(w, err)
		return nil, false
	}

	return &identity, true
}

func (s *Server) handleHistoryList(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeHistoryError(w, http.StatusServiceUnavailable, "history service unavailable", "history_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	filter := history.ListFilter{Limit: history.DefaultListLimit}
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 {
			writeHistoryError(w, http.StatusBadRequest, "limit must be a positive integer", "history_invalid_request")
			return
		}
		filter.Limit = limit
	}
	if filter.Limit > history.MaxListLimit {
		filter.Limit = history.MaxListLimit
	}

	if rawPlatform := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("platform"))); rawPlatform != "" && rawPlatform != "all" {
		platform, supported := toHistoryPlatform(rawPlatform)
		if !supported {
			writeHistoryError(w, http.StatusBadRequest, "invalid platform filter", "history_invalid_request")
			return
		}
		filter.Platform = platform
	}

	if rawStatus := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("status"))); rawStatus != "" && rawStatus != "all" {
		status := history.AttemptStatus(rawStatus)
		if !isHistoryStatusFilterSupported(status) {
			writeHistoryError(w, http.StatusBadRequest, "invalid status filter", "history_invalid_request")
			return
		}
		filter.Status = status
	}

	filter.Query = strings.TrimSpace(r.URL.Query().Get("q"))

	if rawCursor := strings.TrimSpace(r.URL.Query().Get("cursor")); rawCursor != "" {
		cursor, err := decodeHistoryCursor(rawCursor)
		if err != nil {
			writeHistoryError(w, http.StatusBadRequest, "invalid cursor", "history_invalid_cursor")
			return
		}
		filter.Cursor = cursor
	}

	page, err := s.historyStore.ListItems(r.Context(), identity.User.ID, filter)
	if err != nil {
		if errors.Is(err, history.ErrInvalidInput) {
			writeHistoryError(w, http.StatusBadRequest, err.Error(), "history_invalid_request")
			return
		}
		s.logger.Printf("history list failed user_id=%s err=%v", identity.User.ID, err)
		writeHistoryError(w, http.StatusInternalServerError, "failed to fetch history", "history_unavailable")
		return
	}

	items := make([]map[string]any, 0, len(page.Entries))
	for _, entry := range page.Entries {
		lastAttemptAt := entry.Item.CreatedAt.UTC()
		if entry.Item.LastAttemptAt != nil {
			lastAttemptAt = entry.Item.LastAttemptAt.UTC()
		}

		itemPayload := map[string]any{
			"id":              entry.Item.ID,
			"title":           entry.Item.Title,
			"thumbnail_url":   entry.Item.ThumbnailURL,
			"platform":        entry.Item.Platform,
			"source_url":      entry.Item.SourceURL,
			"last_attempt_at": lastAttemptAt,
		}
		if entry.LatestAttempt != nil {
			attempt := entry.LatestAttempt
			attemptPayload := map[string]any{
				"id":            attempt.ID,
				"request_kind":  attempt.RequestKind,
				"status":        attempt.Status,
				"format_id":     attempt.FormatID,
				"quality_label": attempt.QualityLabel,
				"size_bytes":    attempt.SizeBytes,
				"download_url":  attempt.DownloadURL,
				"expires_at":    attempt.ExpiresAt,
				"created_at":    attempt.CreatedAt.UTC(),
			}
			itemPayload["latest_attempt"] = attemptPayload
		}
		items = append(items, itemPayload)
	}

	var nextCursor any
	if page.NextCursor != nil {
		nextCursor = encodeHistoryCursor(page.NextCursor)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page": map[string]any{
			"next_cursor": nextCursor,
			"has_more":    page.HasMore,
			"limit":       filter.Limit,
		},
	})
}

func (s *Server) handleHistoryStats(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeHistoryError(w, http.StatusServiceUnavailable, "history service unavailable", "history_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	stats, err := s.historyStore.GetStats(r.Context(), identity.User.ID)
	if err != nil {
		if errors.Is(err, history.ErrInvalidInput) {
			writeHistoryError(w, http.StatusBadRequest, err.Error(), "history_invalid_request")
			return
		}
		s.logger.Printf("history stats failed user_id=%s err=%v", identity.User.ID, err)
		writeHistoryError(w, http.StatusInternalServerError, "failed to compute history stats", "history_unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_items":            stats.TotalItems,
		"total_attempts":         stats.TotalAttempts,
		"success_count":          stats.SuccessCount,
		"failed_count":           stats.FailedCount,
		"total_bytes_downloaded": stats.TotalBytesDownloaded,
		"this_month_attempts":    stats.ThisMonthAttempts,
	})
}

type historyCreateRequest struct {
	URL          string `json:"url"`
	Platform     string `json:"platform"`
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func (s *Server) handleHistoryCreate(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeHistoryError(w, http.StatusServiceUnavailable, "history service unavailable", "history_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	var req historyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHistoryError(w, http.StatusBadRequest, "invalid JSON body", "history_invalid_request")
		return
	}

	if strings.TrimSpace(req.URL) == "" {
		writeHistoryError(w, http.StatusBadRequest, "url is required", "history_invalid_request")
		return
	}

	_, ok = s.createHistoryAttempt(r.Context(), historyAttemptCreateParams{
		UserID:       identity.User.ID,
		Platform:     req.Platform,
		SourceURL:    req.URL,
		Title:        req.Title,
		ThumbnailURL: req.ThumbnailURL,
		RequestKind:  history.RequestKindMP4, // Default to MP4
		Status:       history.StatusResolved,
	})
	if !ok {
		writeHistoryError(w, http.StatusInternalServerError, "failed to create history entry", "history_unavailable")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) handleHistoryDelete(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeHistoryError(w, http.StatusServiceUnavailable, "history service unavailable", "history_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	if itemID == "" {
		writeHistoryError(w, http.StatusBadRequest, "history id is required", "history_invalid_request")
		return
	}

	err := s.historyStore.SoftDeleteItem(r.Context(), identity.User.ID, itemID, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, history.ErrItemNotFound):
			writeHistoryError(w, http.StatusNotFound, "history item not found", "history_not_found")
		case errors.Is(err, history.ErrInvalidInput):
			writeHistoryError(w, http.StatusBadRequest, err.Error(), "history_invalid_request")
		default:
			s.logger.Printf("history delete failed user_id=%s item_id=%s err=%v", identity.User.ID, itemID, err)
			writeHistoryError(w, http.StatusInternalServerError, "failed to delete history item", "history_unavailable")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleHistoryRedownload(w http.ResponseWriter, r *http.Request) {
	if s.historyStore == nil {
		writeHistoryError(w, http.StatusServiceUnavailable, "history service unavailable", "history_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	itemID := strings.TrimSpace(chi.URLParam(r, "id"))
	if itemID == "" {
		writeHistoryError(w, http.StatusBadRequest, "history id is required", "history_invalid_request")
		return
	}

	item, err := s.historyStore.GetItemByID(r.Context(), identity.User.ID, itemID)
	if err != nil {
		switch {
		case errors.Is(err, history.ErrItemNotFound):
			writeHistoryError(w, http.StatusNotFound, "history item not found", "history_not_found")
		case errors.Is(err, history.ErrInvalidInput):
			writeHistoryError(w, http.StatusBadRequest, err.Error(), "history_invalid_request")
		default:
			s.logger.Printf("history redownload get item failed user_id=%s item_id=%s err=%v", identity.User.ID, itemID, err)
			writeHistoryError(w, http.StatusInternalServerError, "failed to load history item", "history_unavailable")
		}
		return
	}

	latestAttempt, err := s.historyStore.GetLatestAttemptByItem(r.Context(), identity.User.ID, item.ID)
	if err != nil {
		switch {
		case errors.Is(err, history.ErrAttemptNotFound):
			writeHistoryError(w, http.StatusConflict, "history item has no downloadable attempts", "history_conflict")
		case errors.Is(err, history.ErrItemNotFound):
			writeHistoryError(w, http.StatusNotFound, "history item not found", "history_not_found")
		default:
			s.logger.Printf("history redownload latest attempt failed user_id=%s item_id=%s err=%v", identity.User.ID, item.ID, err)
			writeHistoryError(w, http.StatusInternalServerError, "failed to resolve latest history attempt", "history_unavailable")
		}
		return
	}

	request, err := decodeHistoryRedownloadRequest(r)
	if err != nil {
		writeHistoryError(w, http.StatusBadRequest, "invalid JSON body", "history_invalid_request")
		return
	}

	requestKind, err := resolveRedownloadKind(request.RequestKind, latestAttempt.RequestKind)
	if err != nil {
		writeHistoryError(w, http.StatusBadRequest, err.Error(), "history_invalid_request")
		return
	}

	now := time.Now().UTC()
	switch requestKind {
	case history.RequestKindMP3:
		if strings.TrimSpace(latestAttempt.DownloadURL) != "" && (latestAttempt.ExpiresAt == nil || latestAttempt.ExpiresAt.After(now)) {
			writeJSON(w, http.StatusOK, map[string]any{
				"mode":         "direct",
				"download_url": latestAttempt.DownloadURL,
			})
			return
		}

		jobID, queueErr := s.enqueueMP3Job(r.Context(), enqueueMP3Params{
			SourceURL: item.SourceURL,
			Platform:  string(item.Platform),
			Title:     item.Title,
			Thumbnail: item.ThumbnailURL,
			UserID:    identity.User.ID,
		})
		if queueErr != nil {
			s.logger.Printf("history redownload enqueue mp3 failed user_id=%s item_id=%s err=%v", identity.User.ID, item.ID, queueErr)
			writeHistoryError(w, http.StatusInternalServerError, "failed to queue redownload", "history_unavailable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"mode":   "queued",
			"job_id": jobID,
			"status": jobs.StatusQueued,
		})
		return

	case history.RequestKindMP4, history.RequestKindImage:
		formatID := strings.TrimSpace(request.FormatID)
		if formatID == "" {
			formatID = strings.TrimSpace(latestAttempt.FormatID)
		}
		if formatID == "" {
			writeHistoryError(w, http.StatusBadRequest, "format_id is required for this history item", "history_invalid_request")
			return
		}

		downloadURL := "/api/v1/download/mp4?url=" + url.QueryEscape(item.SourceURL) + "&format_id=" + url.QueryEscape(formatID)
		writeJSON(w, http.StatusOK, map[string]any{
			"mode":         "direct",
			"download_url": downloadURL,
		})
		return

	default:
		writeHistoryError(w, http.StatusBadRequest, "unsupported request_kind", "history_invalid_request")
	}
}

func decodeHistoryCursor(raw string) (*history.ListCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}

	var payload historyCursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}

	sortAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.SortAt))
	if err != nil {
		return nil, err
	}
	itemID := strings.TrimSpace(payload.ItemID)
	if itemID == "" {
		return nil, errors.New("cursor item_id is required")
	}

	return &history.ListCursor{SortAt: sortAt.UTC(), ItemID: itemID}, nil
}

func encodeHistoryCursor(cursor *history.ListCursor) string {
	if cursor == nil {
		return ""
	}
	payload := historyCursorPayload{SortAt: cursor.SortAt.UTC().Format(time.RFC3339Nano), ItemID: cursor.ItemID}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeHistoryRedownloadRequest(r *http.Request) (historyRedownloadRequest, error) {
	var req historyRedownloadRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxAuthBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return historyRedownloadRequest{}, nil
		}
		return historyRedownloadRequest{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return historyRedownloadRequest{}, errors.New("multiple JSON objects are not allowed")
		}
		return historyRedownloadRequest{}, err
	}

	return req, nil
}

func resolveRedownloadKind(raw string, fallback history.RequestKind) (history.RequestKind, error) {
	candidate := strings.TrimSpace(strings.ToLower(raw))
	if candidate == "" {
		if fallback == "" {
			return "", errors.New("request_kind is required")
		}
		return fallback, nil
	}

	kind := history.RequestKind(candidate)
	switch kind {
	case history.RequestKindMP3, history.RequestKindMP4, history.RequestKindImage:
		return kind, nil
	default:
		return "", errors.New("request_kind must be one of: mp3, mp4, image")
	}
}

func isHistoryStatusFilterSupported(status history.AttemptStatus) bool {
	switch status {
	case history.StatusResolved, history.StatusQueued, history.StatusProcessing, history.StatusDone, history.StatusFailed, history.StatusExpired:
		return true
	default:
		return false
	}
}

func writeHistoryError(w http.ResponseWriter, status int, message, code string) {
	payload := map[string]any{"error": message}
	if strings.TrimSpace(code) != "" {
		payload["code"] = strings.TrimSpace(code)
	}
	writeJSON(w, status, payload)
}
