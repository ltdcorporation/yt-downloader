package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/config"
	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
)

const TaskConvertMP3 = "mp3:convert"

type ConvertMP3Payload struct {
	JobID       string            `json:"job_id"`
	SourceURL   string            `json:"source_url"`
	Headers     map[string]string `json:"headers,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	OutputKey   string            `json:"output_key"`
	BitrateKbps int               `json:"bitrate_kbps"`
}

type jobStoreUpdater interface {
	Update(ctx context.Context, jobID string, mutate func(*jobs.Record)) (jobs.Record, error)
}

type r2Storage interface {
	UploadFile(ctx context.Context, key string, localPath string) error
	PresignDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, time.Time, error)
}

type historyStoreUpdater interface {
	GetAttemptByJobID(ctx context.Context, jobID string) (history.Attempt, error)
	UpdateAttempt(ctx context.Context, userID, attemptID string, mutate func(*history.Attempt)) (history.Attempt, error)
	MarkItemSuccess(ctx context.Context, userID, itemID string, succeededAt time.Time) error
}

type asynqServerRunner interface {
	Run(handler asynq.Handler) error
}

type Worker struct {
	cfg           config.Config
	logger        *log.Logger
	jobStore      jobStoreUpdater
	historyStore  historyStoreUpdater
	r2            r2Storage
	serverFactory func(redisOpt asynq.RedisClientOpt, cfg asynq.Config) asynqServerRunner
	mkTempDir     func(dir, pattern string) (string, error)
}

func NewWorker(cfg config.Config, logger *log.Logger, jobStore jobStoreUpdater, r2 r2Storage, historyStore historyStoreUpdater) *Worker {
	if logger == nil {
		logger = log.Default()
	}

	return &Worker{
		cfg:          cfg,
		logger:       logger,
		jobStore:     jobStore,
		historyStore: historyStore,
		r2:           r2,
		serverFactory: func(redisOpt asynq.RedisClientOpt, cfg asynq.Config) asynqServerRunner {
			return asynq.NewServer(redisOpt, cfg)
		},
		mkTempDir: os.MkdirTemp,
	}
}

func (w *Worker) Run(_ context.Context) error {
	factory := w.serverFactory
	if factory == nil {
		factory = func(redisOpt asynq.RedisClientOpt, cfg asynq.Config) asynqServerRunner {
			return asynq.NewServer(redisOpt, cfg)
		}
	}

	server := factory(
		asynq.RedisClientOpt{
			Addr:     w.cfg.RedisAddr,
			Password: w.cfg.RedisPassword,
		},
		asynq.Config{
			Concurrency: 8,
			Queues: map[string]int{
				"mp3": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskConvertMP3, w.handleConvertMP3)

	return server.Run(mux)
}

func (w *Worker) handleConvertMP3(_ context.Context, task *asynq.Task) error {
	var payload ConvertMP3Payload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	if payload.JobID == "" || payload.SourceURL == "" || payload.OutputKey == "" {
		return errors.New("invalid payload")
	}
	if payload.BitrateKbps <= 0 {
		payload.BitrateKbps = 128
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if w.jobStore != nil {
		if _, err := w.jobStore.Update(ctx, payload.JobID, func(record *jobs.Record) {
			record.Status = jobs.StatusProcessing
			record.Error = ""
		}); err != nil {
			w.logger.Printf("failed to mark job processing id=%s err=%v", payload.JobID, err)
		}
	}
	w.markHistoryAttemptProcessing(payload.JobID)

	localFile, cleanup, err := w.convertMP3(ctx, payload)
	if err != nil {
		return w.failJob(ctx, payload.JobID, err)
	}
	defer cleanup()

	if w.r2 == nil {
		return w.failJob(ctx, payload.JobID, errors.New("r2 client is not configured"))
	}
	if err := w.r2.UploadFile(ctx, payload.OutputKey, localFile); err != nil {
		return w.failJob(ctx, payload.JobID, err)
	}

	downloadURL, expiresAt, err := w.r2.PresignDownloadURL(ctx, payload.OutputKey, time.Duration(w.cfg.MP3OutputTTLMinutes)*time.Minute)
	if err != nil {
		return w.failJob(ctx, payload.JobID, err)
	}

	if w.jobStore != nil {
		if _, err := w.jobStore.Update(ctx, payload.JobID, func(record *jobs.Record) {
			record.Status = jobs.StatusDone
			record.Error = ""
			record.DownloadURL = downloadURL
			record.ExpiresAt = &expiresAt
		}); err != nil {
			w.logger.Printf("failed to mark job done id=%s err=%v", payload.JobID, err)
			return err
		}
	}
	w.markHistoryAttemptDone(payload.JobID, payload.OutputKey, downloadURL, expiresAt)

	w.logger.Printf("convert mp3 done job=%s output_key=%s", payload.JobID, payload.OutputKey)
	return nil
}

func (w *Worker) failJob(ctx context.Context, jobID string, err error) error {
	if w.jobStore != nil && jobID != "" {
		_, updateErr := w.jobStore.Update(ctx, jobID, func(record *jobs.Record) {
			record.Status = jobs.StatusFailed
			record.Error = clipError(err)
		})
		if updateErr != nil {
			w.logger.Printf("failed to mark job failed id=%s err=%v", jobID, updateErr)
		}
	}
	w.markHistoryAttemptFailed(jobID, err)
	w.logger.Printf("convert mp3 failed job=%s err=%v", jobID, err)
	return err
}

func (w *Worker) convertMP3(ctx context.Context, payload ConvertMP3Payload) (string, func(), error) {
	if w.cfg.YTDLPBinary == "" {
		return "", nil, errors.New("yt-dlp binary is not configured")
	}

	mkTempDir := w.mkTempDir
	if mkTempDir == nil {
		mkTempDir = os.MkdirTemp
	}

	tempDir, err := mkTempDir("", "ytd-mp3-"+payload.JobID+"-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	outputTemplate := filepath.Join(tempDir, payload.JobID+".%(ext)s")
	audioQuality := fmt.Sprintf("%dk", payload.BitrateKbps)

	args := []string{
		"--ignore-config",
		"--no-playlist",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", audioQuality,
		"--output", outputTemplate,
	}
	if w.cfg.YTDLPJSRuntimes != "" {
		args = append(args, "--js-runtimes", w.cfg.YTDLPJSRuntimes)
	}
	for key, value := range payload.Headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		args = append(args, "--add-header", fmt.Sprintf("%s: %s", key, value))
	}
	if payload.UserAgent != "" {
		args = append(args, "--user-agent", payload.UserAgent)
	}
	args = append(args, payload.SourceURL)

	cmd := exec.CommandContext(ctx, w.cfg.YTDLPBinary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		cleanup()
		return "", nil, fmt.Errorf("yt-dlp convert failed: %s", errText)
	}

	expected := filepath.Join(tempDir, payload.JobID+".mp3")
	if _, err := os.Stat(expected); err == nil {
		return expected, cleanup, nil
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "*.mp3"))
	if err != nil || len(matches) == 0 {
		cleanup()
		return "", nil, errors.New("mp3 output not found after conversion")
	}
	return matches[0], cleanup, nil
}

func (w *Worker) markHistoryAttemptProcessing(jobID string) {
	if w == nil || w.historyStore == nil || strings.TrimSpace(jobID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempt, err := w.historyStore.GetAttemptByJobID(ctx, jobID)
	if err != nil {
		if errors.Is(err, history.ErrAttemptNotFound) {
			return
		}
		w.logger.Printf("history update skipped job=%s stage=processing err=%v", jobID, err)
		return
	}

	_, err = w.historyStore.UpdateAttempt(ctx, attempt.UserID, attempt.ID, func(a *history.Attempt) {
		a.Status = history.StatusProcessing
		a.ErrorCode = ""
		a.ErrorText = ""
		a.CompletedAt = nil
	})
	if err != nil {
		w.logger.Printf("history update failed job=%s stage=processing attempt_id=%s err=%v", jobID, attempt.ID, err)
	}
}

func (w *Worker) markHistoryAttemptDone(jobID, outputKey, downloadURL string, expiresAt time.Time) {
	if w == nil || w.historyStore == nil || strings.TrimSpace(jobID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempt, err := w.historyStore.GetAttemptByJobID(ctx, jobID)
	if err != nil {
		if errors.Is(err, history.ErrAttemptNotFound) {
			return
		}
		w.logger.Printf("history update skipped job=%s stage=done err=%v", jobID, err)
		return
	}

	now := time.Now().UTC()
	updated, err := w.historyStore.UpdateAttempt(ctx, attempt.UserID, attempt.ID, func(a *history.Attempt) {
		a.Status = history.StatusDone
		a.OutputKey = strings.TrimSpace(outputKey)
		a.DownloadURL = strings.TrimSpace(downloadURL)
		if expiresAt.IsZero() {
			a.ExpiresAt = nil
		} else {
			expires := expiresAt.UTC()
			a.ExpiresAt = &expires
		}
		a.ErrorCode = ""
		a.ErrorText = ""
		a.CompletedAt = &now
	})
	if err != nil {
		w.logger.Printf("history update failed job=%s stage=done attempt_id=%s err=%v", jobID, attempt.ID, err)
		return
	}

	if err := w.historyStore.MarkItemSuccess(ctx, updated.UserID, updated.HistoryItemID, now); err != nil {
		w.logger.Printf("history item success mark failed job=%s item_id=%s err=%v", jobID, updated.HistoryItemID, err)
	}
}

func (w *Worker) markHistoryAttemptFailed(jobID string, rootErr error) {
	if w == nil || w.historyStore == nil || strings.TrimSpace(jobID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempt, err := w.historyStore.GetAttemptByJobID(ctx, jobID)
	if err != nil {
		if errors.Is(err, history.ErrAttemptNotFound) {
			return
		}
		w.logger.Printf("history update skipped job=%s stage=failed err=%v", jobID, err)
		return
	}

	now := time.Now().UTC()
	_, err = w.historyStore.UpdateAttempt(ctx, attempt.UserID, attempt.ID, func(a *history.Attempt) {
		a.Status = history.StatusFailed
		a.ErrorCode = "mp3_conversion_failed"
		a.ErrorText = clipError(rootErr)
		a.CompletedAt = &now
	})
	if err != nil {
		w.logger.Printf("history update failed job=%s stage=failed attempt_id=%s err=%v", jobID, attempt.ID, err)
	}
}

func clipError(err error) string {
	if err == nil {
		return ""
	}
	const max = 400
	msg := err.Error()
	if len(msg) <= max {
		return msg
	}
	return msg[:max]
}
