package redis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// TokenData represents the cached token information
type TokenData struct {
	Token        string `json:"token"`
	WebserverURL string `json:"webserverUrl"`
	CreatedAt    string `json:"createdAt"`
}

const tokenKey = "advancedmd:token"
const ttlSeconds = 23 * 60 * 60 // 23 hours

// SaveToken stores token in Upstash Redis with TTL
func SaveToken(token, webserverURL string) error {
	data := TokenData{
		Token:        token,
		WebserverURL: webserverURL,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	// URL-encode the JSON value for the REST API
	encodedValue := url.PathEscape(string(jsonData))

	// Upstash REST API: SET key value EX seconds
	apiURL := fmt.Sprintf("%s/set/%s/%s/ex/%d",
		os.Getenv("UPSTASH_REDIS_REST_URL"),
		tokenKey,
		encodedValue,
		ttlSeconds,
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("UPSTASH_REDIS_REST_TOKEN"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("redis error: status %d", resp.StatusCode)
	}

	return nil
}

// GetToken retrieves token from Upstash Redis
func GetToken() (*TokenData, error) {
	apiURL := fmt.Sprintf("%s/get/%s",
		os.Getenv("UPSTASH_REDIS_REST_URL"),
		tokenKey,
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("UPSTASH_REDIS_REST_TOKEN"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result *string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Key not found - return nil without error
	if result.Result == nil {
		return nil, nil
	}

	var data TokenData
	if err := json.Unmarshal([]byte(*result.Result), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &data, nil
}
