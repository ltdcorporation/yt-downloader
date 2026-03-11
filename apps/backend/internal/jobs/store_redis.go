package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"yt-downloader/backend/internal/config"
)

type redisBackend struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func newRedisBackend(cfg config.Config) *redisBackend {
	retentionDays := cfg.JobRetentionDays
	if retentionDays <= 0 {
		retentionDays = 14
	}

	return &redisBackend{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
		}),
		keyPrefix: "ytd",
		ttl:       time.Duration(retentionDays) * 24 * time.Hour,
	}
}

func (r *redisBackend) Close() error {
	return r.client.Close()
}

func (r *redisBackend) Put(ctx context.Context, record Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal job record: %w", err)
	}

	cutoff := time.Now().Add(-r.ttl).Unix()
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, r.jobKey(record.ID), encoded, r.ttl)
	pipe.ZAdd(ctx, r.jobsIndexKey(), redis.Z{
		Score:  float64(record.CreatedAt.Unix()),
		Member: record.ID,
	})
	pipe.ZRemRangeByScore(ctx, r.jobsIndexKey(), "-inf", strconv.FormatInt(cutoff, 10))
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("persist job record: %w", err)
	}
	return nil
}

func (r *redisBackend) Get(ctx context.Context, jobID string) (Record, error) {
	val, err := r.client.Get(ctx, r.jobKey(jobID)).Result()
	if errors.Is(err, redis.Nil) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read job record: %w", err)
	}

	var record Record
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return Record{}, fmt.Errorf("decode job record: %w", err)
	}
	return record, nil
}

func (r *redisBackend) ListRecent(ctx context.Context, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 20
	}

	ids, err := r.client.ZRevRange(ctx, r.jobsIndexKey(), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("read jobs index: %w", err)
	}

	items := make([]Record, 0, len(ids))
	for _, id := range ids {
		record, err := r.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		items = append(items, record)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	return items, nil
}

func (r *redisBackend) jobsIndexKey() string {
	return r.keyPrefix + ":jobs:index"
}

func (r *redisBackend) jobKey(jobID string) string {
	return r.keyPrefix + ":job:" + jobID
}
