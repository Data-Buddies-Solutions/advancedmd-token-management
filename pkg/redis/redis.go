// Package redis provides Redis caching functionality for AdvancedMD tokens.
// It handles storing and retrieving authentication tokens with a 23-hour TTL,
// allowing tokens to be refreshed before they expire (via 20-hour cron schedule).
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenData represents the cached token information stored in Redis.
// This struct contains both the authentication token and pre-built URLs
// for all AdvancedMD API types, enabling ElevenLabs agents to make API calls
// without needing to construct URLs (since dynamic variables don't support
// string manipulation).
//
// URL Types:
//   - WebserverURL: Base URL returned from AdvancedMD login (used for reference)
//   - XmlrpcURL: Full URL for XMLRPC API calls (addpatient, getpatient, scheduling)
//   - RestApiBase: Base URL for Practice Manager REST API (profiles, master files)
//   - EhrApiBase: Base URL for EHR REST API (documents, files)
type TokenData struct {
	// Token is the AdvancedMD session token used for authentication.
	// Pass this in the Cookie header as: Cookie: token={Token}
	Token string `json:"token"`

	// WebserverURL is the base URL returned from AdvancedMD's 2-step login.
	// Example: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP
	WebserverURL string `json:"webserverUrl"`

	// XmlrpcURL is the full XMLRPC endpoint URL for legacy/core API operations.
	// Use this for: addpatient, getpatient, scheduling, and other ppmdmsg operations.
	// Example: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx
	XmlrpcURL string `json:"xmlrpcUrl"`

	// RestApiBase is the base URL for Practice Manager REST API operations.
	// Append endpoint paths like /masterfiles/olsprofiles to this base.
	// Example: https://providerapi.advancedmd.com/api/api-801/YOURAPP
	RestApiBase string `json:"restApiBase"`

	// EhrApiBase is the base URL for Electronic Health Records (EHR) REST API.
	// Append endpoint paths like /files/documents to this base.
	// Example: https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP
	EhrApiBase string `json:"ehrApiBase"`

	// CreatedAt is the RFC3339 timestamp when this token was generated.
	// Useful for debugging and monitoring token freshness.
	CreatedAt string `json:"createdAt"`
}

// Redis key and TTL configuration
const (
	// tokenKey is the Redis key where AdvancedMD token data is stored.
	// Using a namespaced key (advancedmd:token) to avoid conflicts with other data.
	tokenKey = "advancedmd:token"

	// ttlDuration sets the token cache expiration to 23 hours.
	// Combined with the 20-hour cron refresh schedule, this provides a 3-hour
	// buffer ensuring tokens are always fresh when requested.
	ttlDuration = 23 * time.Hour
)

// ctx is the background context used for all Redis operations.
// Using background context since these are fire-and-forget operations
// within the Vercel serverless function lifecycle.
var ctx = context.Background()

// getClient creates and returns a Redis client using the REDIS_URL environment variable.
// The URL format should be: redis://default:password@host:port
//
// This function creates a new client for each request rather than using a connection pool,
// which is appropriate for serverless environments where connections shouldn't persist
// between invocations.
//
// Returns:
//   - *redis.Client: Configured Redis client ready for use
//   - error: Non-nil if REDIS_URL is missing or malformed
func getClient() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable not set")
	}

	// Parse the Redis URL to extract connection parameters
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}

	// Create and return the client - caller is responsible for closing
	client := redis.NewClient(opt)
	return client, nil
}

// SaveToken stores the complete token data in Redis with a 23-hour TTL.
// The TTL is set to 23 hours while the cron job runs every 20 hours,
// ensuring tokens are refreshed before expiration with a 3-hour buffer.
//
// Parameters:
//   - tokenData: Complete TokenData struct containing token and all pre-built URLs
//
// Returns:
//   - error: Non-nil if Redis connection fails or data cannot be saved
func SaveToken(tokenData *TokenData) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Marshal the complete token data including all pre-built URLs
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	// Store with 23-hour TTL - cron refreshes every 20 hours for 3-hour buffer
	err = client.Set(ctx, tokenKey, jsonData, ttlDuration).Err()
	if err != nil {
		return fmt.Errorf("failed to save token to Redis: %w", err)
	}

	return nil
}

// GetToken retrieves the cached token data from Redis.
// Returns nil (without error) if no token is cached, allowing callers
// to trigger an on-demand token refresh.
//
// Returns:
//   - *TokenData: The cached token data, or nil if not found
//   - error: Non-nil only if Redis connection fails (not for missing keys)
func GetToken() (*TokenData, error) {
	client, err := getClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	val, err := client.Get(ctx, tokenKey).Result()
	if err == redis.Nil {
		// Key not found (expired or never set) - return nil to trigger on-demand refresh
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Redis: %w", err)
	}

	// Unmarshal the JSON data into TokenData struct
	var data TokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &data, nil
}
