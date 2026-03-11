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
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/storage"
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

type Worker struct {
	cfg      config.Config
	logger   *log.Logger
	jobStore *jobs.Store
	r2       *storage.R2Client
}

func NewWorker(cfg config.Config, logger *log.Logger, jobStore *jobs.Store, r2 *storage.R2Client) *Worker {
	return &Worker{
		cfg:      cfg,
		logger:   logger,
		jobStore: jobStore,
		r2:       r2,
	}
}

func (w *Worker) Run(_ context.Context) error {
	server := asynq.NewServer(
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
	w.logger.Printf("convert mp3 failed job=%s err=%v", jobID, err)
	return err
}

func (w *Worker) convertMP3(ctx context.Context, payload ConvertMP3Payload) (string, func(), error) {
	if w.cfg.YTDLPBinary == "" {
		return "", nil, errors.New("yt-dlp binary is not configured")
	}

	tempDir, err := os.MkdirTemp("", "ytd-mp3-"+payload.JobID+"-")
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
