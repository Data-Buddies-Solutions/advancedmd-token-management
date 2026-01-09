package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenData represents the cached token information
type TokenData struct {
	Token        string `json:"token"`
	WebserverURL string `json:"webserverUrl"`
	CreatedAt    string `json:"createdAt"`
}

const tokenKey = "advancedmd:token"
const ttlDuration = 23 * time.Hour // 23 hours

var ctx = context.Background()

// getClient creates a Redis client from the REDIS_URL environment variable
// Format: redis://default:password@host:port
func getClient() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable not set")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	client := redis.NewClient(opt)
	return client, nil
}

// SaveToken stores token in Redis with TTL
func SaveToken(token, webserverURL string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	data := TokenData{
		Token:        token,
		WebserverURL: webserverURL,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	err = client.Set(ctx, tokenKey, jsonData, ttlDuration).Err()
	if err != nil {
		return fmt.Errorf("failed to save token to Redis: %w", err)
	}

	return nil
}

// GetToken retrieves token from Redis
func GetToken() (*TokenData, error) {
	client, err := getClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	val, err := client.Get(ctx, tokenKey).Result()
	if err == redis.Nil {
		// Key not found - return nil without error
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Redis: %w", err)
	}

	var data TokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &data, nil
}
