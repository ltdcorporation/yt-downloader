package queue

import (
	"context"
	"encoding/json"
	"log"

	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/config"
)

const TaskConvertMP3 = "mp3:convert"

type ConvertMP3Payload struct {
	JobID       string `json:"job_id"`
	SourceURL   string `json:"source_url"`
	OutputKey   string `json:"output_key"`
	BitrateKbps int    `json:"bitrate_kbps"`
}

type Worker struct {
	cfg    config.Config
	logger *log.Logger
}

func NewWorker(cfg config.Config, logger *log.Logger) *Worker {
	return &Worker{
		cfg:    cfg,
		logger: logger,
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

	// Placeholder: integrate yt-dlp + ffmpeg + upload to R2.
	w.logger.Printf("convert mp3 job=%s src=%s bitrate=%dkbps", payload.JobID, payload.SourceURL, payload.BitrateKbps)
	return nil
}
