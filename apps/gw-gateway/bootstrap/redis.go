package bootstrap

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg *Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("connecting to redis at %s: %w", cfg.RedisAddr, err)
	}

	return client, nil
}
