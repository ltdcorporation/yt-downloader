package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"yt-downloader/backend/internal/history"
	"yt-downloader/backend/internal/jobs"
	"yt-downloader/backend/internal/queue"
	"yt-downloader/backend/internal/youtube"
)

const (
	videoCutModeManual  = "manual"
	videoCutModeHeatmap = "heatmap"

	videoCutDefaultWindowSec = 35
	videoCutMinWindowSec     = 3
	videoCutDefaultMaxSec    = 180
)

type createVideoCutJobRequest struct {
	URL      string                  `json:"url"`
	FormatID string                  `json:"format_id"`
	CutMode  string                  `json:"cut_mode"`
	Manual   *videoCutManualRequest  `json:"manual,omitempty"`
	Heatmap  *videoCutHeatmapRequest `json:"heatmap,omitempty"`
}

type videoCutManualRequest struct {
	StartSec int `json:"start_sec"`
	EndSec   int `json:"end_sec"`
}

type videoCutHeatmapRequest struct {
	TargetSec int `json:"target_sec,omitempty"`
	WindowSec int `json:"window_sec,omitempty"`
}

type enqueueVideoCutParams struct {
	SourceURL        string
	Headers          map[string]string
	UserAgent        string
	Platform         string
	Title            string
	Thumbnail        string
	UserID           string
	FormatID         string
	QualityLabel     string
	CutMode          string
	ManualStartSec   int
	ManualEndSec     int
	HeatmapTargetSec int
	HeatmapWindowSec int
}

type computedVideoCutPlan struct {
	Mode            string
	ManualStartSec  int
	ManualEndSec    int
	HeatmapTarget   int
	HeatmapWindow   int
	EffectiveStart  int
	EffectiveEnd    int
	EffectiveWindow int
}

func (s *Server) handleCreateVideoCutJob(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HeatmapTrimEnabled {
		writeErrorWithCode(w, http.StatusServiceUnavailable, "video_cut_disabled", "video cut feature is disabled")
		return
	}

	var req createVideoCutJobRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	req.FormatID = strings.TrimSpace(req.FormatID)
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if req.FormatID == "" {
		writeError(w, http.StatusBadRequest, "format_id is required")
		return
	}
	if s.detectPlatform(req.URL) != "youtube" {
		writeErrorWithCode(w, http.StatusBadRequest, "video_cut_platform_not_supported", "video cut currently supports YouTube only")
		return
	}

	resolved, err := s.resolver.Resolve(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	selectedFormat, ok := findYouTubeMP4Format(resolved.Formats, req.FormatID)
	if !ok {
		writeErrorWithCode(w, http.StatusBadRequest, "format_not_available", "selected format is not available")
		return
	}

	plan, err := s.computeVideoCutPlan(req, resolved)
	if err != nil {
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_trim_range", err.Error())
		return
	}

	sourceURL, headers, userAgent, err := youtube.ParseInput(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	identity := s.optionalSessionIdentity(r)
	userID := ""
	if identity != nil {
		userID = identity.User.ID
	}

	jobID, err := s.enqueueVideoCutJob(r.Context(), enqueueVideoCutParams{
		SourceURL:        sourceURL,
		Headers:          headers,
		UserAgent:        userAgent,
		Platform:         "youtube",
		Title:            resolved.Title,
		Thumbnail:        resolved.Thumbnail,
		UserID:           userID,
		FormatID:         selectedFormat.ID,
		QualityLabel:     selectedFormat.Quality,
		CutMode:          plan.Mode,
		ManualStartSec:   plan.ManualStartSec,
		ManualEndSec:     plan.ManualEndSec,
		HeatmapTargetSec: plan.HeatmapTarget,
		HeatmapWindowSec: plan.HeatmapWindow,
	})
	if err != nil {
		s.logger.Printf("failed to create video-cut job source=%s err=%v", sourceURL, err)
		writeError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id": jobID,
		"status": jobs.StatusQueued,
	})
}

func (s *Server) enqueueVideoCutJob(ctx context.Context, params enqueueVideoCutParams) (string, error) {
	if s == nil {
		return "", fmt.Errorf("server is not initialized")
	}

	sourceURL := strings.TrimSpace(params.SourceURL)
	if sourceURL == "" {
		return "", fmt.Errorf("source url is required")
	}
	formatID := strings.TrimSpace(params.FormatID)
	if formatID == "" {
		return "", fmt.Errorf("format_id is required")
	}
	if strings.TrimSpace(params.CutMode) == "" {
		return "", fmt.Errorf("cut_mode is required")
	}

	jobID := "job_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	outputKey := buildVideoCutOutputKey(s.cfg.R2KeyPrefix, jobID)
	now := time.Now().UTC()

	record := jobs.Record{
		ID:         jobID,
		Status:     jobs.StatusQueued,
		InputURL:   sourceURL,
		OutputKind: "video_cut",
		OutputKey:  outputKey,
		Title:      strings.TrimSpace(params.Title),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.jobStore.Put(ctx, record); err != nil {
		return "", fmt.Errorf("persist queued video-cut job: %w", err)
	}

	var historyAttempt *history.Attempt
	if strings.TrimSpace(params.UserID) != "" {
		qualityLabel := strings.TrimSpace(params.QualityLabel)
		if qualityLabel == "" {
			qualityLabel = "mp4"
		}
		qualityLabel = fmt.Sprintf("%s • %s", qualityLabel, strings.TrimSpace(strings.ToLower(params.CutMode)))
		createdAttempt, ok := s.createHistoryAttempt(ctx, historyAttemptCreateParams{
			UserID:       params.UserID,
			Platform:     params.Platform,
			SourceURL:    sourceURL,
			Title:        params.Title,
			ThumbnailURL: params.Thumbnail,
			RequestKind:  history.RequestKindMP4,
			Status:       history.StatusQueued,
			FormatID:     formatID,
			QualityLabel: qualityLabel,
			JobID:        jobID,
			OutputKey:    outputKey,
		})
		if ok {
			historyAttempt = createdAttempt
		}
	}

	payload := queue.VideoCutPayload{
		JobID:            jobID,
		SourceURL:        sourceURL,
		Headers:          params.Headers,
		UserAgent:        strings.TrimSpace(params.UserAgent),
		FormatID:         formatID,
		OutputKey:        outputKey,
		CutMode:          strings.TrimSpace(strings.ToLower(params.CutMode)),
		ManualStartSec:   params.ManualStartSec,
		ManualEndSec:     params.ManualEndSec,
		HeatmapTargetSec: params.HeatmapTargetSec,
		HeatmapWindowSec: params.HeatmapWindowSec,
	}
	taskBytes, err := json.Marshal(payload)
	if err != nil {
		_, _ = s.jobStore.Update(ctx, jobID, func(item *jobs.Record) {
			item.Status = jobs.StatusFailed
			item.Error = "failed to encode video-cut payload"
		})
		if historyAttempt != nil {
			s.markHistoryAttemptFailed(historyAttempt, "video_cut_payload_encode_failed", err)
		}
		return "", fmt.Errorf("encode video-cut payload: %w", err)
	}

	task := asynq.NewTask(queue.TaskVideoCut, taskBytes)
	_, err = s.queue.Enqueue(
		task,
		asynq.TaskID(jobID),
		asynq.Queue("video"),
		asynq.Timeout(40*time.Minute),
		asynq.MaxRetry(2),
	)
	if err != nil {
		_, _ = s.jobStore.Update(ctx, jobID, func(item *jobs.Record) {
			item.Status = jobs.StatusFailed
			item.Error = "failed to enqueue video-cut job"
		})
		if historyAttempt != nil {
			s.markHistoryAttemptFailed(historyAttempt, "video_cut_queue_enqueue_failed", err)
		}
		return "", fmt.Errorf("enqueue video-cut job: %w", err)
	}

	return jobID, nil
}

func (s *Server) computeVideoCutPlan(req createVideoCutJobRequest, resolved youtube.ResolveResult) (computedVideoCutPlan, error) {
	duration := resolved.DurationSeconds
	if duration <= 0 {
		return computedVideoCutPlan{}, fmt.Errorf("video duration is unavailable")
	}

	maxCutSec := s.cfg.VideoCutMaxDurationSec
	if maxCutSec <= 0 {
		maxCutSec = videoCutDefaultMaxSec
	}

	mode := strings.TrimSpace(strings.ToLower(req.CutMode))
	if mode == "" {
		mode = videoCutModeManual
	}

	switch mode {
	case videoCutModeManual:
		if req.Manual == nil {
			return computedVideoCutPlan{}, fmt.Errorf("manual trim payload is required")
		}
		startSec := req.Manual.StartSec
		endSec := req.Manual.EndSec
		if startSec < 0 {
			return computedVideoCutPlan{}, fmt.Errorf("start_sec must be >= 0")
		}
		if endSec <= startSec {
			return computedVideoCutPlan{}, fmt.Errorf("end_sec must be greater than start_sec")
		}
		if endSec > duration {
			return computedVideoCutPlan{}, fmt.Errorf("end_sec exceeds video duration")
		}
		if endSec-startSec > maxCutSec {
			return computedVideoCutPlan{}, fmt.Errorf("trim range exceeds maximum allowed duration (%d sec)", maxCutSec)
		}
		return computedVideoCutPlan{
			Mode:           mode,
			ManualStartSec: startSec,
			ManualEndSec:   endSec,
			EffectiveStart: startSec,
			EffectiveEnd:   endSec,
		}, nil
	case videoCutModeHeatmap:
		if !resolved.HeatmapMeta.Available || len(resolved.KeyMoments) == 0 {
			return computedVideoCutPlan{}, fmt.Errorf("heatmap moments are unavailable")
		}

		targetSec := 0
		windowSec := videoCutDefaultWindowSec
		if req.Heatmap != nil {
			targetSec = req.Heatmap.TargetSec
			if req.Heatmap.WindowSec > 0 {
				windowSec = req.Heatmap.WindowSec
			}
		}
		if targetSec <= 0 {
			targetSec = resolved.KeyMoments[0]
		}
		if targetSec < 0 || targetSec > duration {
			return computedVideoCutPlan{}, fmt.Errorf("heatmap target is out of range")
		}
		if windowSec < videoCutMinWindowSec {
			return computedVideoCutPlan{}, fmt.Errorf("heatmap window must be >= %d sec", videoCutMinWindowSec)
		}
		if windowSec > maxCutSec {
			return computedVideoCutPlan{}, fmt.Errorf("heatmap window exceeds maximum allowed duration (%d sec)", maxCutSec)
		}
		if windowSec > duration {
			windowSec = duration
		}

		half := windowSec / 2
		startSec := targetSec - half
		if startSec < 0 {
			startSec = 0
		}
		endSec := startSec + windowSec
		if endSec > duration {
			endSec = duration
			startSec = endSec - windowSec
			if startSec < 0 {
				startSec = 0
			}
		}
		if endSec <= startSec {
			return computedVideoCutPlan{}, fmt.Errorf("computed heatmap range is invalid")
		}

		return computedVideoCutPlan{
			Mode:            mode,
			ManualStartSec:  startSec,
			ManualEndSec:    endSec,
			HeatmapTarget:   targetSec,
			HeatmapWindow:   windowSec,
			EffectiveStart:  startSec,
			EffectiveEnd:    endSec,
			EffectiveWindow: endSec - startSec,
		}, nil
	default:
		return computedVideoCutPlan{}, fmt.Errorf("cut_mode is invalid")
	}
}

func findYouTubeMP4Format(formats []youtube.Format, formatID string) (youtube.Format, bool) {
	id := strings.TrimSpace(formatID)
	if id == "" {
		return youtube.Format{}, false
	}
	for _, format := range formats {
		if format.ID != id {
			continue
		}
		if strings.TrimSpace(strings.ToLower(format.Type)) != "mp4" {
			continue
		}
		return format, true
	}
	return youtube.Format{}, false
}

func buildVideoCutOutputKey(prefix, jobID string) string {
	cleanJobID := strings.TrimSpace(jobID)
	if cleanJobID == "" {
		cleanJobID = "unknown"
	}

	segments := make([]string, 0, 3)
	if trimmedPrefix := strings.Trim(prefix, " /"); trimmedPrefix != "" {
		segments = append(segments, trimmedPrefix)
	}
	segments = append(segments, "video-cut", cleanJobID+".mp4")
	return strings.Join(segments, "/")
}
