package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"yt-downloader/backend/internal/config"
)

type fakeMinioClient struct {
	fputErr    error
	putErr     error
	removeErr  error
	presignErr error

	fputCalls       int
	fputBucket      string
	fputKey         string
	fputPath        string
	fputContentType string

	putCalls       int
	putBucket      string
	putKey         string
	putSize        int64
	putContentType string
	putPayload     []byte

	removeCalls  int
	removeBucket string
	removeKey    string

	presignCalls   int
	presignBucket  string
	presignKey     string
	presignExpires time.Duration
	presignParams  url.Values
	presignURL     *url.URL
}

func (f *fakeMinioClient) FPutObject(_ context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	f.fputCalls++
	f.fputBucket = bucketName
	f.fputKey = objectName
	f.fputPath = filePath
	f.fputContentType = opts.ContentType

	if f.fputErr != nil {
		return minio.UploadInfo{}, f.fputErr
	}

	return minio.UploadInfo{Bucket: bucketName, Key: objectName}, nil
}

func (f *fakeMinioClient) PutObject(_ context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	f.putCalls++
	f.putBucket = bucketName
	f.putKey = objectName
	f.putSize = objectSize
	f.putContentType = opts.ContentType

	if reader != nil {
		payload, _ := io.ReadAll(reader)
		f.putPayload = payload
	}

	if f.putErr != nil {
		return minio.UploadInfo{}, f.putErr
	}

	return minio.UploadInfo{Bucket: bucketName, Key: objectName}, nil
}

func (f *fakeMinioClient) RemoveObject(_ context.Context, bucketName, objectName string, _ minio.RemoveObjectOptions) error {
	f.removeCalls++
	f.removeBucket = bucketName
	f.removeKey = objectName
	if f.removeErr != nil {
		return f.removeErr
	}
	return nil
}

func (f *fakeMinioClient) PresignedGetObject(_ context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	f.presignCalls++
	f.presignBucket = bucketName
	f.presignKey = objectName
	f.presignExpires = expires
	f.presignParams = reqParams

	if f.presignErr != nil {
		return nil, f.presignErr
	}
	if f.presignURL != nil {
		return f.presignURL, nil
	}

	defaultURL, _ := url.Parse("https://example.com/default-signed-url")
	return defaultURL, nil
}

func overrideMinioFactory(t *testing.T, factory func(endpoint string, opts *minio.Options) (minioObjectClient, error)) {
	t.Helper()
	prev := newMinioClient
	newMinioClient = factory
	t.Cleanup(func() {
		newMinioClient = prev
	})
}

func validR2Config() config.Config {
	return config.Config{
		R2Endpoint:        "https://example.cloudflare.r2.cloudflarestorage.com",
		R2Region:          "",
		R2Bucket:          "bucket-a",
		R2AccessKeyID:     "key",
		R2SecretAccessKey: "secret",
	}
}

func TestNewR2Client_RequiresCompleteConfig(t *testing.T) {
	_, err := NewR2Client(context.Background(), config.Config{})
	if err == nil {
		t.Fatal("expected error for incomplete R2 configuration")
	}
	if !strings.Contains(err.Error(), "r2 configuration is incomplete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewR2Client_InvalidEndpoint(t *testing.T) {
	t.Run("parse error", func(t *testing.T) {
		cfg := validR2Config()
		cfg.R2Endpoint = "://bad-url"

		_, err := NewR2Client(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected invalid endpoint error")
		}
		if !strings.Contains(err.Error(), "invalid r2 endpoint") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing host", func(t *testing.T) {
		cfg := validR2Config()
		cfg.R2Endpoint = "https:///missing-host"

		_, err := NewR2Client(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected invalid endpoint host error")
		}
		if !strings.Contains(err.Error(), "invalid r2 endpoint host") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestNewR2Client_CreateClientError(t *testing.T) {
	overrideMinioFactory(t, func(_ string, _ *minio.Options) (minioObjectClient, error) {
		return nil, errors.New("dial failed")
	})

	_, err := NewR2Client(context.Background(), validR2Config())
	if err == nil {
		t.Fatal("expected create client error")
	}
	if !strings.Contains(err.Error(), "create r2 client: dial failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewR2Client_UsesFactoryOptions(t *testing.T) {
	t.Run("https endpoint uses secure + auto region fallback", func(t *testing.T) {
		fake := &fakeMinioClient{}
		capturedEndpoint := ""
		capturedOpts := &minio.Options{}

		overrideMinioFactory(t, func(endpoint string, opts *minio.Options) (minioObjectClient, error) {
			capturedEndpoint = endpoint
			capturedOpts = opts
			return fake, nil
		})

		cfg := validR2Config()
		cfg.R2Endpoint = "https://r2.example.com:9443"
		cfg.R2Region = ""

		client, err := NewR2Client(context.Background(), cfg)
		if err != nil {
			t.Fatalf("expected success, got err: %v", err)
		}
		if client == nil || client.client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.bucket != "bucket-a" {
			t.Fatalf("unexpected bucket: %s", client.bucket)
		}
		if capturedEndpoint != "r2.example.com:9443" {
			t.Fatalf("unexpected endpoint passed to minio factory: %s", capturedEndpoint)
		}
		if !capturedOpts.Secure {
			t.Fatalf("expected secure=true for https endpoint")
		}
		if capturedOpts.Region != "auto" {
			t.Fatalf("expected default region auto, got %s", capturedOpts.Region)
		}
		if capturedOpts.BucketLookup != minio.BucketLookupPath {
			t.Fatalf("expected bucket lookup path, got %v", capturedOpts.BucketLookup)
		}
	})

	t.Run("http endpoint uses insecure + explicit region", func(t *testing.T) {
		fake := &fakeMinioClient{}
		capturedOpts := &minio.Options{}

		overrideMinioFactory(t, func(_ string, opts *minio.Options) (minioObjectClient, error) {
			capturedOpts = opts
			return fake, nil
		})

		cfg := validR2Config()
		cfg.R2Endpoint = "http://r2.example.com"
		cfg.R2Region = "apac"

		_, err := NewR2Client(context.Background(), cfg)
		if err != nil {
			t.Fatalf("expected success, got err: %v", err)
		}
		if capturedOpts.Secure {
			t.Fatalf("expected secure=false for http endpoint")
		}
		if capturedOpts.Region != "apac" {
			t.Fatalf("expected explicit region apac, got %s", capturedOpts.Region)
		}
	})
}

func TestNewR2Client_SuccessAndPresignWithRealMinioClient(t *testing.T) {
	cfg := validR2Config()
	client, err := NewR2Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected client creation success, got err: %v", err)
	}
	if client == nil || client.client == nil {
		t.Fatal("expected non-nil R2 client")
	}
	if client.bucket != "bucket-a" {
		t.Fatalf("unexpected bucket: %s", client.bucket)
	}

	urlValue, expiresAt, err := client.PresignDownloadURL(context.Background(), "mp3/job_test.mp3", 0)
	if err != nil {
		t.Fatalf("expected presign success, got err: %v", err)
	}
	if !strings.Contains(urlValue, "X-Amz-Signature=") {
		t.Fatalf("expected signed URL, got: %s", urlValue)
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatalf("expected future expiration time, got: %v", expiresAt)
	}
}

func TestUploadFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeMinioClient{}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.UploadFile(context.Background(), "mp3/job_ok.mp3", "/tmp/job_ok.mp3")
		if err != nil {
			t.Fatalf("expected upload success, got err: %v", err)
		}
		if fake.fputCalls != 1 {
			t.Fatalf("expected one fput call, got %d", fake.fputCalls)
		}
		if fake.fputBucket != "bucket-a" || fake.fputKey != "mp3/job_ok.mp3" || fake.fputPath != "/tmp/job_ok.mp3" {
			t.Fatalf("unexpected fput call args: bucket=%s key=%s path=%s", fake.fputBucket, fake.fputKey, fake.fputPath)
		}
		if fake.fputContentType != "audio/mpeg" {
			t.Fatalf("expected audio/mpeg content type, got %s", fake.fputContentType)
		}
	})

	t.Run("error wrapped", func(t *testing.T) {
		fake := &fakeMinioClient{fputErr: errors.New("upload failed")}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.UploadFile(context.Background(), "mp3/job_fail.mp3", "/tmp/job_fail.mp3")
		if err == nil {
			t.Fatal("expected upload error")
		}
		if !strings.Contains(err.Error(), "upload object to r2: upload failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUploadObject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeMinioClient{}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.UploadObject(context.Background(), "avatars/usr_1/avatar.webp", "image/webp", []byte("payload"))
		if err != nil {
			t.Fatalf("expected upload success, got err: %v", err)
		}
		if fake.putCalls != 1 {
			t.Fatalf("expected one put call, got %d", fake.putCalls)
		}
		if fake.putBucket != "bucket-a" || fake.putKey != "avatars/usr_1/avatar.webp" {
			t.Fatalf("unexpected put args: bucket=%s key=%s", fake.putBucket, fake.putKey)
		}
		if fake.putSize != int64(len("payload")) {
			t.Fatalf("unexpected put size: %d", fake.putSize)
		}
		if fake.putContentType != "image/webp" {
			t.Fatalf("unexpected content type: %s", fake.putContentType)
		}
		if string(fake.putPayload) != "payload" {
			t.Fatalf("unexpected payload: %q", string(fake.putPayload))
		}
	})

	t.Run("default content type", func(t *testing.T) {
		fake := &fakeMinioClient{}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.UploadObject(context.Background(), "avatars/usr_1/avatar.webp", "", []byte("payload"))
		if err != nil {
			t.Fatalf("expected upload success, got err: %v", err)
		}
		if fake.putContentType != "application/octet-stream" {
			t.Fatalf("expected fallback content type, got %s", fake.putContentType)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		fake := &fakeMinioClient{}
		client := &R2Client{bucket: "bucket-a", client: fake}

		if err := client.UploadObject(context.Background(), "", "image/webp", []byte("payload")); err == nil {
			t.Fatalf("expected key validation error")
		}
		if err := client.UploadObject(context.Background(), "avatars/usr_1/avatar.webp", "image/webp", nil); err == nil {
			t.Fatalf("expected payload validation error")
		}
	})

	t.Run("error wrapped", func(t *testing.T) {
		fake := &fakeMinioClient{putErr: errors.New("upload bytes failed")}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.UploadObject(context.Background(), "avatars/usr_1/avatar.webp", "image/webp", []byte("payload"))
		if err == nil {
			t.Fatal("expected upload error")
		}
		if !strings.Contains(err.Error(), "upload object bytes to r2: upload bytes failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteObject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeMinioClient{}
		client := &R2Client{bucket: "bucket-a", client: fake}

		err := client.DeleteObject(context.Background(), "avatars/usr_1/avatar.webp")
		if err != nil {
			t.Fatalf("expected delete success, got err: %v", err)
		}
		if fake.removeCalls != 1 {
			t.Fatalf("expected one remove call, got %d", fake.removeCalls)
		}
		if fake.removeBucket != "bucket-a" || fake.removeKey != "avatars/usr_1/avatar.webp" {
			t.Fatalf("unexpected remove args: bucket=%s key=%s", fake.removeBucket, fake.removeKey)
		}
	})

	t.Run("validation and wrapped errors", func(t *testing.T) {
		fake := &fakeMinioClient{removeErr: errors.New("remove failed")}
		client := &R2Client{bucket: "bucket-a", client: fake}

		if err := client.DeleteObject(context.Background(), ""); err == nil {
			t.Fatalf("expected key validation error")
		}

		err := client.DeleteObject(context.Background(), "avatars/usr_1/avatar.webp")
		if err == nil {
			t.Fatal("expected delete error")
		}
		if !strings.Contains(err.Error(), "delete object from r2: remove failed") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPresignDownloadURL(t *testing.T) {
	t.Run("success with default expiry", func(t *testing.T) {
		signedURL, _ := url.Parse("https://example.com/signed?token=abc")
		fake := &fakeMinioClient{presignURL: signedURL}
		client := &R2Client{bucket: "bucket-a", client: fake}

		start := time.Now().UTC()
		urlValue, expiresAt, err := client.PresignDownloadURL(context.Background(), "mp3/job_ok.mp3", 0)
		if err != nil {
			t.Fatalf("expected presign success, got err: %v", err)
		}
		if urlValue != signedURL.String() {
			t.Fatalf("unexpected signed URL, got %s want %s", urlValue, signedURL.String())
		}
		if fake.presignCalls != 1 {
			t.Fatalf("expected one presign call, got %d", fake.presignCalls)
		}
		if fake.presignExpires != time.Hour {
			t.Fatalf("expected default expiry 1h, got %v", fake.presignExpires)
		}
		if fake.presignBucket != "bucket-a" || fake.presignKey != "mp3/job_ok.mp3" {
			t.Fatalf("unexpected presign args: bucket=%s key=%s", fake.presignBucket, fake.presignKey)
		}
		if fake.presignParams != nil {
			t.Fatalf("expected nil req params, got %#v", fake.presignParams)
		}
		if expiresAt.Before(start.Add(59*time.Minute)) || expiresAt.After(start.Add(61*time.Minute)) {
			t.Fatalf("expected expiresAt around +1h, got %v (start=%v)", expiresAt, start)
		}
	})

	t.Run("error wrapped with custom expiry", func(t *testing.T) {
		fake := &fakeMinioClient{presignErr: errors.New("presign failed")}
		client := &R2Client{bucket: "bucket-a", client: fake}

		_, _, err := client.PresignDownloadURL(context.Background(), "mp3/job_fail.mp3", 5*time.Minute)
		if err == nil {
			t.Fatal("expected presign error")
		}
		if !strings.Contains(err.Error(), "create signed download url: presign failed") {
			t.Fatalf("unexpected error: %v", err)
		}
		if fake.presignExpires != 5*time.Minute {
			t.Fatalf("expected custom expiry to pass through, got %v", fake.presignExpires)
		}
	})
}
