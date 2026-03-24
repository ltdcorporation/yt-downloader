package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"yt-downloader/backend/internal/config"
)

type minioObjectClient interface {
	FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
}

var newMinioClient = func(endpoint string, opts *minio.Options) (minioObjectClient, error) {
	return minio.New(endpoint, opts)
}

type R2Client struct {
	bucket string
	client minioObjectClient
}

func NewR2Client(_ context.Context, cfg config.Config) (*R2Client, error) {
	if cfg.R2Endpoint == "" || cfg.R2Bucket == "" || cfg.R2AccessKeyID == "" || cfg.R2SecretAccessKey == "" {
		return nil, errors.New("r2 configuration is incomplete")
	}

	parsed, err := url.Parse(strings.TrimSpace(cfg.R2Endpoint))
	if err != nil {
		return nil, fmt.Errorf("invalid r2 endpoint: %w", err)
	}
	if parsed.Host == "" {
		return nil, errors.New("invalid r2 endpoint host")
	}

	region := cfg.R2Region
	if region == "" {
		region = "auto"
	}

	client, err := newMinioClient(parsed.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, ""),
		Secure:       parsed.Scheme == "https",
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("create r2 client: %w", err)
	}

	return &R2Client{
		bucket: cfg.R2Bucket,
		client: client,
	}, nil
}

func (c *R2Client) UploadFile(ctx context.Context, key string, localPath string) error {
	_, err := c.client.FPutObject(
		ctx,
		c.bucket,
		key,
		localPath,
		minio.PutObjectOptions{
			ContentType: "audio/mpeg",
		},
	)
	if err != nil {
		return fmt.Errorf("upload object to r2: %w", err)
	}
	return nil
}

func (c *R2Client) UploadObject(ctx context.Context, key, contentType string, body []byte) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("r2 object key is required")
	}
	if len(body) == 0 {
		return errors.New("r2 object payload is empty")
	}

	cleanContentType := strings.TrimSpace(contentType)
	if cleanContentType == "" {
		cleanContentType = "application/octet-stream"
	}

	reader := bytes.NewReader(body)
	_, err := c.client.PutObject(
		ctx,
		c.bucket,
		key,
		reader,
		int64(len(body)),
		minio.PutObjectOptions{ContentType: cleanContentType},
	)
	if err != nil {
		return fmt.Errorf("upload object bytes to r2: %w", err)
	}
	return nil
}

func (c *R2Client) DeleteObject(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("r2 object key is required")
	}

	if err := c.client.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object from r2: %w", err)
	}
	return nil
}

func (c *R2Client) PresignDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, time.Time, error) {
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}
	urlValue, err := c.client.PresignedGetObject(ctx, c.bucket, key, expiresIn, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create signed download url: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	return urlValue.String(), expiresAt, nil
}
