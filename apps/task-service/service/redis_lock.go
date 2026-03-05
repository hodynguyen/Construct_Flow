package service

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hodynguyen/construct-flow/apps/task-service/domain"
)

// redisLockClient is the concrete implementation of domain.LockClient backed by Redis.
// Wrapping *redis.Client here keeps the dependency out of the domain and use-case layers,
// making them fully testable with mocks.
type redisLockClient struct {
	client *redis.Client
}

func NewRedisLockClient(client *redis.Client) domain.LockClient {
	return &redisLockClient{client: client}
}

func (r *redisLockClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

func (r *redisLockClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}
