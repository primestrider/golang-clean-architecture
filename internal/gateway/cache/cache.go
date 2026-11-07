// Package cache berisi implementasi port usecase.Cache: Redis untuk
// production, dan NoopCache saat Redis dimatikan.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache menyimpan nilai sebagai JSON di Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache membuat RedisCache di atas client yang sudah terhubung.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Get membaca key dan men-decode JSON-nya ke dst. Key yang tidak ada
// menghasilkan (false, nil), bukan error.
func (c *RedisCache) Get(ctx context.Context, key string, dst any) (bool, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, err
	}
	return true, nil
}

// Set menyimpan value (di-encode JSON) dengan masa berlaku ttl.
func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, raw, ttl).Err()
}

// Delete menghapus satu atau lebih key.
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// NoopCache adalah cache yang tidak menyimpan apa pun. Dipakai saat Redis
// dimatikan, sehingga use case tidak perlu memeriksa ada tidaknya cache.
type NoopCache struct{}

// Get selalu melaporkan cache miss.
func (NoopCache) Get(context.Context, string, any) (bool, error) { return false, nil }

// Set tidak melakukan apa-apa.
func (NoopCache) Set(context.Context, string, any, time.Duration) error { return nil }

// Delete tidak melakukan apa-apa.
func (NoopCache) Delete(context.Context, ...string) error { return nil }
