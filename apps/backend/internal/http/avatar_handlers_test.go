package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yt-downloader/backend/internal/auth"
	"yt-downloader/backend/internal/avatar"
)

type testAvatarObjectStore struct {
	uploadErr      error
	deleteErrByKey map[string]error

	uploads []struct {
		key string
	}
	deletes []string
}

func (s *testAvatarObjectStore) UploadObject(_ context.Context, key, _ string, _ []byte) error {
	if s.uploadErr != nil {
		return s.uploadErr
	}
	s.uploads = append(s.uploads, struct{ key string }{key: key})
	return nil
}

func (s *testAvatarObjectStore) DeleteObject(_ context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	if s.deleteErrByKey != nil {
		if err, ok := s.deleteErrByKey[key]; ok {
			return err
		}
	}
	return nil
}

type testAvatarProcessor struct {
	out []byte
	err error
}

func (p testAvatarProcessor) Normalize(_ context.Context, _ []byte) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	return append([]byte(nil), p.out...), nil
}

func installAvatarService(t *testing.T, server *Server, objectStore *testAvatarObjectStore, processor testAvatarProcessor) {
	t.Helper()
	svc, err := avatar.NewService(server.authStore, objectStore, processor, avatar.Options{
		PublicBaseURL:  "https://avatar.indobang.site",
		KeyPrefix:      "avatars",
		MaxUploadBytes: 2 * 1024 * 1024,
		DeleteAttempts: 1,
		DeleteBackoff:  1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new avatar service: %v", err)
	}
	server.avatarService = svc
}

func buildAvatarMultipartRequest(t *testing.T, targetPath string, payload []byte, includeField bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if includeField {
		part, err := writer.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("write form file payload: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, targetPath, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestProfileAvatarUploadAndDeleteFlow(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	objectStore := &testAvatarObjectStore{}
	installAvatarService(t, server, objectStore, testAvatarProcessor{out: []byte("webp-bytes")})

	uploadReq := buildAvatarMultipartRequest(t, "/v1/profile/avatar", []byte("raw-image"), true)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK {
		t.Fatalf("upload expected 200, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}
	uploadPayload := decodeJSONMap(t, uploadRec.Body.Bytes())
	profileObj, ok := uploadPayload["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile object, got %v", uploadPayload)
	}
	avatarURL, _ := profileObj["avatar_url"].(string)
	if avatarURL == "" || !strings.HasPrefix(avatarURL, "https://avatar.indobang.site/avatars/") {
		t.Fatalf("unexpected avatar url: %s", avatarURL)
	}
	if len(objectStore.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(objectStore.uploads))
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/profile/avatar", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	deletePayload := decodeJSONMap(t, deleteRec.Body.Bytes())
	profileObj, ok = deletePayload["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile object on delete, got %v", deletePayload)
	}
	if got := strings.TrimSpace(anyToString(profileObj["avatar_url"])); got != "" {
		t.Fatalf("expected empty avatar url after delete, got %q", got)
	}
	if len(objectStore.deletes) != 1 {
		t.Fatalf("expected one delete call, got %d", len(objectStore.deletes))
	}
}

func TestProfileAvatarUploadValidationErrors(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)

	objectStore := &testAvatarObjectStore{}
	installAvatarService(t, server, objectStore, testAvatarProcessor{out: []byte("webp-bytes")})

	t.Run("missing file field", func(t *testing.T) {
		req := buildAvatarMultipartRequest(t, "/v1/profile/avatar", nil, false)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 missing file, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "avatar_invalid_request" {
			t.Fatalf("expected avatar_invalid_request code, got %v", payload["code"])
		}
	})

	t.Run("payload too large", func(t *testing.T) {
		tooLarge := bytes.Repeat([]byte("a"), 2*1024*1024+64)
		req := buildAvatarMultipartRequest(t, "/v1/profile/avatar", tooLarge, true)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "avatar_payload_too_large" {
			t.Fatalf("expected avatar_payload_too_large code, got %v", payload["code"])
		}
	})

	t.Run("invalid image from processor", func(t *testing.T) {
		installAvatarService(t, server, objectStore, testAvatarProcessor{err: avatar.ErrInvalidImage})
		req := buildAvatarMultipartRequest(t, "/v1/profile/avatar", []byte("raw-image"), true)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid image, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "avatar_invalid_image" {
			t.Fatalf("expected avatar_invalid_image code, got %v", payload["code"])
		}
	})
}

func TestProfileAvatarDeleteRollbackOnDeleteFailure(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, userID := registerUserAndGetToken(t, server)

	oldURL := "https://avatar.indobang.site/avatars/" + userID + "/old.webp"
	if _, err := server.authService.UpdateAvatarURL(context.Background(), userID, oldURL); err != nil {
		t.Fatalf("seed avatar url: %v", err)
	}

	objectStore := &testAvatarObjectStore{
		deleteErrByKey: map[string]error{
			"avatars/" + userID + "/old.webp": errors.New("r2 delete failed"),
		},
	}
	installAvatarService(t, server, objectStore, testAvatarProcessor{out: []byte("webp-bytes")})

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/profile/avatar", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 delete conflict, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	payload := decodeJSONMap(t, deleteRec.Body.Bytes())
	if payload["code"] != "avatar_replace_conflict" {
		t.Fatalf("expected avatar_replace_conflict code, got %v", payload["code"])
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	profileReq.Header.Set("Authorization", "Bearer "+token)
	profileRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("expected profile 200, got %d body=%s", profileRec.Code, profileRec.Body.String())
	}
	decoded := struct {
		Profile struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"profile"`
	}{}
	if err := json.Unmarshal(profileRec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode profile payload: %v", err)
	}
	if decoded.Profile.AvatarURL != oldURL {
		t.Fatalf("expected avatar rollback to old URL, got %s", decoded.Profile.AvatarURL)
	}
}

func TestProfileAvatar_ServiceUnavailable(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	token, _ := registerUserAndGetToken(t, server)
	server.avatarService = nil

	uploadReq := buildAvatarMultipartRequest(t, "/v1/profile/avatar", []byte("raw"), true)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected upload 503, got %d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/profile/avatar", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected delete 503, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestProfileAvatarUpload_UnauthorizedAndMalformed(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())
	installAvatarService(t, server, &testAvatarObjectStore{}, testAvatarProcessor{out: []byte("webp-bytes")})

	t.Run("unauthorized", func(t *testing.T) {
		req := buildAvatarMultipartRequest(t, "/v1/profile/avatar", []byte("raw-image"), true)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 unauthorized, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/profile/avatar", strings.NewReader("not-multipart"))
		req.Header.Set("Content-Type", "application/json")
		token, _ := registerUserAndGetToken(t, server)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 malformed payload, got %d body=%s", rec.Code, rec.Body.String())
		}
		payload := decodeJSONMap(t, rec.Body.Bytes())
		if payload["code"] != "avatar_invalid_request" {
			t.Fatalf("expected avatar_invalid_request code, got %v", payload["code"])
		}
	})
}

func TestAvatarHandler_WriteAvatarPayloadErrorBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "payload too large", err: avatar.ErrPayloadTooLarge, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "avatar_payload_too_large"},
		{name: "payload empty", err: avatar.ErrPayloadEmpty, wantStatus: http.StatusBadRequest, wantCode: "avatar_invalid_request"},
		{name: "max bytes error", err: &http.MaxBytesError{Limit: 1}, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "avatar_payload_too_large"},
		{name: "multipart too large", err: multipart.ErrMessageTooLarge, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "avatar_payload_too_large"},
		{name: "missing file alias", err: errors.New("no such file or directory"), wantStatus: http.StatusBadRequest, wantCode: "avatar_invalid_request"},
		{name: "generic malformed", err: errors.New("bad multipart"), wantStatus: http.StatusBadRequest, wantCode: "avatar_invalid_request"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.writeAvatarPayloadError(rec, tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status mismatch: got=%d want=%d body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			payload := decodeJSONMap(t, rec.Body.Bytes())
			if payload["code"] != tc.wantCode {
				t.Fatalf("code mismatch: got=%v want=%s", payload["code"], tc.wantCode)
			}
		})
	}
}

func TestAvatarHandler_WriteAvatarMutationErrorBranches(t *testing.T) {
	cfg := baseTestConfig()
	server := newTestServer(t, cfg, &fakeResolver{}, &fakeQueue{}, newFakeJobStore())

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "payload too large", err: avatar.ErrPayloadTooLarge, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "avatar_payload_too_large"},
		{name: "payload empty", err: avatar.ErrPayloadEmpty, wantStatus: http.StatusBadRequest, wantCode: "avatar_invalid_request"},
		{name: "invalid image", err: avatar.ErrInvalidImage, wantStatus: http.StatusBadRequest, wantCode: "avatar_invalid_image"},
		{name: "delete failed", err: avatar.ErrDeleteFailed, wantStatus: http.StatusConflict, wantCode: "avatar_replace_conflict"},
		{name: "rollback failed", err: avatar.ErrRollbackFailed, wantStatus: http.StatusConflict, wantCode: "avatar_replace_conflict"},
		{name: "user not found", err: auth.ErrUserNotFound, wantStatus: http.StatusNotFound, wantCode: "profile_not_found"},
		{name: "deadline exceeded", err: context.DeadlineExceeded, wantStatus: http.StatusGatewayTimeout, wantCode: "avatar_timeout"},
		{name: "context canceled", err: context.Canceled, wantStatus: http.StatusRequestTimeout, wantCode: "avatar_canceled"},
		{name: "unknown error", err: errors.New("unknown"), wantStatus: http.StatusInternalServerError, wantCode: "avatar_unavailable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.writeAvatarMutationError(rec, "usr_1", "upload", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status mismatch: got=%d want=%d body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			payload := decodeJSONMap(t, rec.Body.Bytes())
			if payload["code"] != tc.wantCode {
				t.Fatalf("code mismatch: got=%v want=%s", payload["code"], tc.wantCode)
			}
		})
	}
}

func anyToString(v any) string {
	s, _ := v.(string)
	return s
}
