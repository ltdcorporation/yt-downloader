package http

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/queue"
)

type enqueueMP3Params struct {
	SourceURL  string
	Headers    map[string]string
	UserAgent  string
	Platform   string
	Title      string
	Thumbnail  string
	UserID     string
	RequestCtx context.Context
}

func (s *Server) enqueueMP3Job(ctx context.Context, params enqueueMP3Params) (string, error) {
	if s == nil {
		return "", fmt.Errorf("server is not initialized")
	}

	sourceURL := strings.TrimSpace(params.SourceURL)
	if sourceURL == "" {
		return "", fmt.Errorf("source url is required")
	}

	bitrateKbps := s.cfg.MP3Bitrate
	if bitrateKbps <= 0 {
		bitrateKbps = 128
	}

	jobID := "job_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	outputKey := buildMP3OutputKey(s.cfg.R2KeyPrefix, jobID)
	now := time.Now().UTC()

	record := jobs.Record{
		ID:         jobID,
		Status:     jobs.StatusQueued,
		InputURL:   sourceURL,
		OutputKind: "mp3",
		OutputKey:  outputKey,
		Title:      strings.TrimSpace(params.Title),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.jobStore.Put(ctx, record); err != nil {
		return "", fmt.Errorf("persist queued job: %w", err)
	}

	var historyAttempt *history.Attempt
	if strings.TrimSpace(params.UserID) != "" {
		createdAttempt, ok := s.createHistoryAttempt(ctx, historyAttemptCreateParams{
			UserID:       params.UserID,
			Platform:     params.Platform,
			SourceURL:    sourceURL,
			Title:        params.Title,
			ThumbnailURL: params.Thumbnail,
			RequestKind:  history.RequestKindMP3,
			Status:       history.StatusQueued,
			QualityLabel: fmt.Sprintf("%dkbps", bitrateKbps),
			JobID:        jobID,
			OutputKey:    outputKey,
		})
		if ok {
			historyAttempt = createdAttempt
		}
	}

	payload := queue.ConvertMP3Payload{
		JobID:       jobID,
		SourceURL:   sourceURL,
		Headers:     params.Headers,
		UserAgent:   strings.TrimSpace(params.UserAgent),
		OutputKey:   outputKey,
		BitrateKbps: bitrateKbps,
	}
	taskBytes, err := json.Marshal(payload)
	if err != nil {
		_, _ = s.jobStore.Update(ctx, jobID, func(item *jobs.Record) {
			item.Status = jobs.StatusFailed
			item.Error = "failed to encode job payload"
		})
		if historyAttempt != nil {
			s.markHistoryAttemptFailed(historyAttempt, "queue_payload_encode_failed", err)
		}
		return "", fmt.Errorf("encode queue payload: %w", err)
	}

	task := asynq.NewTask(queue.TaskConvertMP3, taskBytes)
	_, err = s.queue.Enqueue(
		task,
		asynq.TaskID(jobID),
		asynq.Queue("mp3"),
		asynq.Timeout(20*time.Minute),
		asynq.MaxRetry(2),
	)
	if err != nil {
		_, _ = s.jobStore.Update(ctx, jobID, func(item *jobs.Record) {
			item.Status = jobs.StatusFailed
			item.Error = "failed to enqueue job"
		})
		if historyAttempt != nil {
			s.markHistoryAttemptFailed(historyAttempt, "queue_enqueue_failed", err)
		}
		return "", fmt.Errorf("enqueue mp3 job: %w", err)
	}

	return jobID, nil
}
