package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/redis/go-redis/v9"
)

// Options berisi jadwal cron untuk setiap job (format 5 kolom standar).
type Options struct {
	TokenCleanupCron  string
	OutboxCleanupCron string
}

// New membuat scheduler dan mendaftarkan job pembersihan.
//
// locker opsional (nil = tanpa lock). Bila worker berjalan di beberapa
// replica, berikan locker bersama (misalnya Redis) supaya setiap jadwal
// hanya dijalankan oleh satu replica.
func New(opts Options, locker gocron.Locker, log *slog.Logger, jobs Jobs) (gocron.Scheduler, error) {
	schedulerOpts := []gocron.SchedulerOption{gocron.WithLocation(time.UTC)}
	if locker != nil {
		schedulerOpts = append(schedulerOpts, gocron.WithDistributedLocker(locker))
	}
	s, err := gocron.NewScheduler(schedulerOpts...)
	if err != nil {
		return nil, err
	}

	register := func(name, cron string, run func(context.Context) error) error {
		_, err := s.NewJob(gocron.CronJob(cron, false), gocron.NewTask(func(ctx context.Context) {
			start := time.Now()
			if err := run(ctx); err != nil {
				log.ErrorContext(ctx, "scheduled job failed", "job", name, "error", err)
				return
			}
			log.InfoContext(ctx, "scheduled job finished", "job", name, "duration_ms", time.Since(start).Milliseconds())
		}), gocron.WithName(name))
		if err != nil {
			return fmt.Errorf("register job %s: %w", name, err)
		}
		return nil
	}
	if err := errors.Join(
		register("cleanup-refresh-tokens", opts.TokenCleanupCron, jobs.CleanupRefreshTokens),
		register("cleanup-outbox", opts.OutboxCleanupCron, jobs.CleanupOutbox),
	); err != nil {
		_ = s.Shutdown()
		return nil, err
	}
	return s, nil
}

// RedisLocker adalah gocron.Locker berbasis Redis (SET NX dengan masa
// berlaku). Replica yang berhasil membuat key lebih dulu menjalankan job;
// yang lain melewatkan jadwal tersebut.
type RedisLocker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisLocker membuat RedisLocker. ttl harus lebih lama dari durasi job,
// dan menjadi pengaman bila replica pemegang lock mati sebelum melepasnya.
func NewRedisLocker(client *redis.Client, ttl time.Duration) *RedisLocker {
	return &RedisLocker{client: client, ttl: ttl}
}

// Lock mencoba mengambil lock untuk key. Mengembalikan error bila lock
// sedang dipegang replica lain; gocron lalu melewatkan jadwal ini.
func (l *RedisLocker) Lock(ctx context.Context, key string) (gocron.Lock, error) {
	redisKey := "scheduler:lock:" + key
	ok, err := l.client.SetNX(ctx, redisKey, "locked", l.ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("lock held by another worker")
	}
	return &redisLock{client: l.client, key: redisKey}, nil
}

// redisLock adalah lock yang sedang dipegang.
type redisLock struct {
	client *redis.Client
	key    string
}

// Unlock melepas lock.
func (l *redisLock) Unlock(ctx context.Context) error {
	return l.client.Del(ctx, l.key).Err()
}
