// Package clients contains external service clients.
package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"advancedmd-token-management/internal/domain"
)

const (
	// tokenKey is the Redis key where AdvancedMD token data is stored.
	tokenKey = "advancedmd:token"

	// ttlDuration sets the token cache expiration to 23 hours.
	ttlDuration = 23 * time.Hour
)

// RedisClient wraps a Redis connection pool.
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client with connection pooling.
func NewRedisClient(redisURL string) (*RedisClient, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	// Configure connection pool
	opt.PoolSize = 10
	opt.MinIdleConns = 2

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// SaveToken stores the token data in Redis with a 23-hour TTL.
func (r *RedisClient) SaveToken(ctx context.Context, tokenData *domain.TokenData) error {
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	if err := r.client.Set(ctx, tokenKey, jsonData, ttlDuration).Err(); err != nil {
		return fmt.Errorf("failed to save token to Redis: %w", err)
	}

	return nil
}

// GetToken retrieves the cached token data from Redis.
// Returns nil (without error) if no token is cached.
func (r *RedisClient) GetToken(ctx context.Context) (*domain.TokenData, error) {
	val, err := r.client.Get(ctx, tokenKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Redis: %w", err)
	}

	var data domain.TokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &data, nil
}
