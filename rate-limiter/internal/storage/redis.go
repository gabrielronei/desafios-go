package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage is the Redis-backed Storage implementation.
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage creates a connected RedisStorage.
func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

// Increment uses a fixed-window strategy: the key is bucketed by the current
// Unix second divided by the window length, ensuring the counter resets at
// clean window boundaries. The Lua script guarantees atomicity.
var incrScript = redis.NewScript(`
local key    = KEYS[1]
local window = tonumber(ARGV[1])
local now    = tonumber(ARGV[2])
local bucket = key .. ":" .. math.floor(now / window)

local count = redis.call("INCR", bucket)
if count == 1 then
    redis.call("EXPIRE", bucket, window + 1)
end
return count
`)

func (r *RedisStorage) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	windowSecs := int64(window.Seconds())
	if windowSecs < 1 {
		windowSecs = 1
	}

	count, err := incrScript.Run(ctx, r.client,
		[]string{key},
		windowSecs,
		time.Now().Unix(),
	).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis increment script: %w", err)
	}

	return count, nil
}

func (r *RedisStorage) IsBlocked(ctx context.Context, key string) (bool, error) {
	blockKey := "blocked:" + key
	n, err := r.client.Exists(ctx, blockKey).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return n > 0, nil
}

func (r *RedisStorage) Block(ctx context.Context, key string, duration time.Duration) error {
	blockKey := "blocked:" + key
	if err := r.client.Set(ctx, blockKey, 1, duration).Err(); err != nil {
		return fmt.Errorf("redis set block: %w", err)
	}
	return nil
}

func (r *RedisStorage) Reset(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}
	return nil
}
