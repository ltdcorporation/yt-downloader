package http

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/avatar"
)

const avatarFormFieldName = "avatar"

func (s *Server) handleProfileAvatarUpload(w http.ResponseWriter, r *http.Request) {
	if s.avatarService == nil {
		writeAvatarError(w, http.StatusServiceUnavailable, "avatar service unavailable", "avatar_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	payload, err := s.readAvatarPayload(w, r)
	if err != nil {
		s.writeAvatarPayloadError(w, err)
		return
	}

	updatedProfile, err := s.avatarService.ReplaceAvatar(r.Context(), identity.User.ID, payload)
	if err != nil {
		s.writeAvatarMutationError(w, identity.User.ID, "upload", err)
		return
	}

	writeJSON(w, http.StatusOK, profileResponse{Profile: updatedProfile})
}

func (s *Server) handleProfileAvatarDelete(w http.ResponseWriter, r *http.Request) {
	if s.avatarService == nil {
		writeAvatarError(w, http.StatusServiceUnavailable, "avatar service unavailable", "avatar_unavailable")
		return
	}

	identity, ok := s.requireSessionIdentity(w, r)
	if !ok {
		return
	}

	updatedProfile, err := s.avatarService.RemoveAvatar(r.Context(), identity.User.ID)
	if err != nil {
		s.writeAvatarMutationError(w, identity.User.ID, "delete", err)
		return
	}

	writeJSON(w, http.StatusOK, profileResponse{Profile: updatedProfile})
}

func (s *Server) readAvatarPayload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	maxBytes := int64(avatar.DefaultMaxUploadBytes)
	if s != nil && s.cfg.AvatarUploadMaxBytes > 0 {
		maxBytes = s.cfg.AvatarUploadMaxBytes
	}

	requestLimit := maxBytes + 256*1024
	r.Body = http.MaxBytesReader(w, r.Body, requestLimit)
	if err := r.ParseMultipartForm(requestLimit); err != nil {
		return nil, err
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile(avatarFormFieldName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if header != nil && header.Size > 0 && header.Size > maxBytes {
		return nil, avatar.ErrPayloadTooLarge
	}

	payload, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxBytes {
		return nil, avatar.ErrPayloadTooLarge
	}
	if len(payload) == 0 {
		return nil, avatar.ErrPayloadEmpty
	}

	return payload, nil
}

func (s *Server) writeAvatarPayloadError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	if errors.Is(err, avatar.ErrPayloadTooLarge) {
		writeAvatarError(w, http.StatusRequestEntityTooLarge, "avatar payload exceeds max size", "avatar_payload_too_large")
		return
	}
	if errors.Is(err, avatar.ErrPayloadEmpty) {
		writeAvatarError(w, http.StatusBadRequest, "avatar file is required", "avatar_invalid_request")
		return
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeAvatarError(w, http.StatusRequestEntityTooLarge, "avatar payload exceeds max size", "avatar_payload_too_large")
		return
	}

	if errors.Is(err, multipart.ErrMessageTooLarge) {
		writeAvatarError(w, http.StatusRequestEntityTooLarge, "avatar payload exceeds max size", "avatar_payload_too_large")
		return
	}

	if errors.Is(err, http.ErrMissingFile) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
		writeAvatarError(w, http.StatusBadRequest, "avatar file is required", "avatar_invalid_request")
		return
	}

	writeAvatarError(w, http.StatusBadRequest, "invalid avatar upload request", "avatar_invalid_request")
}

func (s *Server) writeAvatarMutationError(w http.ResponseWriter, userID, operation string, err error) {
	if err == nil {
		return
	}

	s.logger.Printf("profile avatar %s failed user_id=%s err=%v", operation, userID, err)

	switch {
	case errors.Is(err, avatar.ErrPayloadTooLarge):
		writeAvatarError(w, http.StatusRequestEntityTooLarge, "avatar payload exceeds max size", "avatar_payload_too_large")
		return
	case errors.Is(err, avatar.ErrPayloadEmpty):
		writeAvatarError(w, http.StatusBadRequest, "avatar file is required", "avatar_invalid_request")
		return
	case errors.Is(err, avatar.ErrInvalidImage):
		writeAvatarError(w, http.StatusBadRequest, "invalid avatar image", "avatar_invalid_image")
		return
	case errors.Is(err, avatar.ErrDeleteFailed), errors.Is(err, avatar.ErrRollbackFailed):
		writeAvatarError(w, http.StatusConflict, "failed to delete previous avatar, please retry", "avatar_replace_conflict")
		return
	case errors.Is(err, auth.ErrUserNotFound):
		writeAvatarError(w, http.StatusNotFound, "profile not found", "profile_not_found")
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeAvatarError(w, http.StatusGatewayTimeout, "avatar processing timed out", "avatar_timeout")
		return
	case errors.Is(err, context.Canceled):
		writeAvatarError(w, http.StatusRequestTimeout, "avatar request canceled", "avatar_canceled")
		return
	default:
		writeAvatarError(w, http.StatusInternalServerError, "failed to update avatar", "avatar_unavailable")
		return
	}
}

func writeAvatarError(w http.ResponseWriter, status int, message string, code string) {
	payload := map[string]any{
		"error": strings.TrimSpace(message),
	}
	if trimmedCode := strings.TrimSpace(code); trimmedCode != "" {
		payload["code"] = trimmedCode
	}
	writeJSON(w, status, payload)
}
