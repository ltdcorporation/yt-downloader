package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/history"
)

const historyWriteTimeout = 3 * time.Second

type historyAttemptCreateParams struct {
	UserID       string
	Platform     string
	SourceURL    string
	Title        string
	ThumbnailURL string
	RequestKind  history.RequestKind
	Status       history.AttemptStatus
	FormatID     string
	QualityLabel string
	JobID        string
	OutputKey    string
	DownloadURL  string
	ExpiresAt    *time.Time
}

func (s *Server) optionalSessionIdentity(r *http.Request) *auth.SessionIdentity {
	if s == nil || s.authService == nil {
		return nil
	}

	token := s.readSessionToken(r)
	if strings.TrimSpace(token) == "" {
		return nil
	}

	identity, err := s.authService.AuthenticateToken(r.Context(), token)
	if err == nil {
		return &identity
	}

	switch {
	case errors.Is(err, auth.ErrInvalidSessionToken),
		errors.Is(err, auth.ErrSessionRevoked),
		errors.Is(err, auth.ErrSessionExpired):
		s.logger.Printf("history auth skipped due to invalid session err=%v", err)
		return nil
	default:
		s.logger.Printf("history auth skipped due to auth service error err=%v", err)
		return nil
	}
}

func (s *Server) createHistoryAttempt(ctx context.Context, params historyAttemptCreateParams) (*history.Attempt, bool) {
	if s == nil || s.historyStore == nil {
		return nil, false
	}
	if strings.TrimSpace(params.UserID) == "" {
		return nil, false
	}

	if err := s.historyStore.EnsureReady(ctx); err != nil {
		s.logger.Printf("history ensure ready failed err=%v", err)
		return nil, false
	}

	historyPlatform, ok := toHistoryPlatform(params.Platform)
	if !ok {
		s.logger.Printf("history create skipped unsupported platform=%q", params.Platform)
		return nil, false
	}

	now := time.Now().UTC()
	item, err := s.historyStore.UpsertItem(ctx, history.Item{
		ID:            "his_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:        strings.TrimSpace(params.UserID),
		Platform:      historyPlatform,
		SourceURL:     params.SourceURL,
		Title:         params.Title,
		ThumbnailURL:  params.ThumbnailURL,
		LastAttemptAt: ptrTime(now),
		AttemptCount:  1,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		s.logger.Printf("history item upsert failed user_id=%s err=%v", strings.TrimSpace(params.UserID), err)
		return nil, false
	}

	attempt, err := s.historyStore.CreateAttempt(ctx, history.Attempt{
		ID:            "hat_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		HistoryItemID: item.ID,
		UserID:        item.UserID,
		RequestKind:   params.RequestKind,
		Status:        params.Status,
		FormatID:      params.FormatID,
		QualityLabel:  params.QualityLabel,
		JobID:         params.JobID,
		OutputKey:     params.OutputKey,
		DownloadURL:   params.DownloadURL,
		ExpiresAt:     params.ExpiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		s.logger.Printf("history attempt create failed user_id=%s item_id=%s err=%v", item.UserID, item.ID, err)
		return nil, false
	}

	return &attempt, true
}

func (s *Server) markHistoryAttemptFailed(attempt *history.Attempt, errorCode string, err error) {
	if s == nil || s.historyStore == nil || attempt == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), historyWriteTimeout)
	defer cancel()

	_, updateErr := s.historyStore.UpdateAttempt(ctx, attempt.UserID, attempt.ID, func(a *history.Attempt) {
		a.Status = history.StatusFailed
		a.ErrorCode = strings.TrimSpace(errorCode)
		a.ErrorText = clipHistoryError(err)
		now := time.Now().UTC()
		a.CompletedAt = &now
	})
	if updateErr != nil {
		s.logger.Printf("history attempt mark failed failed attempt_id=%s user_id=%s err=%v", attempt.ID, attempt.UserID, updateErr)
	}
}

func (s *Server) markHistoryAttemptDone(attempt *history.Attempt, sizeBytes *int64) {
	if s == nil || s.historyStore == nil || attempt == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), historyWriteTimeout)
	defer cancel()

	now := time.Now().UTC()
	updated, err := s.historyStore.UpdateAttempt(ctx, attempt.UserID, attempt.ID, func(a *history.Attempt) {
		a.Status = history.StatusDone
		a.ErrorCode = ""
		a.ErrorText = ""
		a.SizeBytes = sizeBytes
		a.CompletedAt = &now
	})
	if err != nil {
		s.logger.Printf("history attempt mark done failed attempt_id=%s user_id=%s err=%v", attempt.ID, attempt.UserID, err)
		return
	}

	if err := s.historyStore.MarkItemSuccess(ctx, updated.UserID, updated.HistoryItemID, now); err != nil {
		s.logger.Printf("history item mark success failed item_id=%s user_id=%s err=%v", updated.HistoryItemID, updated.UserID, err)
	}
}

func toHistoryPlatform(platform string) (history.Platform, bool) {
	switch strings.TrimSpace(strings.ToLower(platform)) {
	case "youtube":
		return history.PlatformYouTube, true
	case "tiktok":
		return history.PlatformTikTok, true
	case "instagram":
		return history.PlatformInstagram, true
	case "x":
		return history.PlatformX, true
	default:
		return "", false
	}
}

func clipHistoryError(err error) string {
	if err == nil {
		return ""
	}
	const max = 400
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= max {
		return msg
	}
	return msg[:max]
}

func ptrTime(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
