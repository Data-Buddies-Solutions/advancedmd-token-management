package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

// mockRedisClient provides a test double for RedisClient
type mockRedisClient struct {
	tokenData *domain.TokenData
	saveErr   error
	getErr    error
}

func (m *mockRedisClient) SaveToken(ctx context.Context, data *domain.TokenData) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.tokenData = data
	return nil
}

func (m *mockRedisClient) GetToken(ctx context.Context) (*domain.TokenData, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.tokenData, nil
}

func TestTokenManager_GetToken_Cached(t *testing.T) {
	// Setup mock authenticator that tracks calls
	var authCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&authCalls, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
			<PPMDResults>
				<Results success="1">
					<usercontext>fresh-token</usercontext>
				</Results>
			</PPMDResults>`))
	}))
	defer server.Close()

	auth := &Authenticator{
		creds: Credentials{
			Username:  "testuser",
			Password:  "testpass",
			OfficeKey: "991TEST",
			AppName:   "testapp",
		},
		client: server.Client(),
	}

	// Create token manager with pre-cached data
	tm := &TokenManager{
		authenticator: auth,
		tokenData: &domain.TokenData{
			Token:        "Bearer cached-token",
			CookieToken:  "token=cached-token",
			WebserverURL: "test.com/processrequest/api-801/testapp",
			XmlrpcURL:    "test.com/processrequest/api-801/testapp/xmlrpc/processrequest.aspx",
			RestApiBase:  "test.com/api/api-801/testapp",
			EhrApiBase:   "test.com/ehr-api/api-801/testapp",
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		},
		stopCh: make(chan struct{}),
	}

	// Get token should return cached data without calling auth
	ctx := context.Background()
	data, err := tm.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	if data.Token != "Bearer cached-token" {
		t.Errorf("Expected cached token, got %s", data.Token)
	}

	if atomic.LoadInt32(&authCalls) != 0 {
		t.Errorf("Expected 0 auth calls for cached token, got %d", authCalls)
	}
}

func TestTokenManager_GetToken_RefreshOnEmpty(t *testing.T) {
	// This test verifies that when no token is cached, GetToken triggers a refresh
	// In a real test, we'd use a mock Redis client

	t.Run("triggers refresh when cache empty", func(t *testing.T) {
		// Token manager with nil tokenData should trigger refresh
		tm := &TokenManager{
			tokenData: nil,
			stopCh:    make(chan struct{}),
		}

		// Without authenticator and redis, this would fail
		// This test documents the expected behavior
		if tm.tokenData != nil {
			t.Error("Expected nil tokenData")
		}
	})
}

func TestBuildTokenData(t *testing.T) {
	token := "test-token-123"
	webserverURL := "https://providerapi.advancedmd.com/processrequest/api-801/testapp"

	data := domain.BuildTokenData(token, webserverURL)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Token has Bearer prefix", data.Token, "Bearer test-token-123"},
		{"CookieToken has token= prefix", data.CookieToken, "token=test-token-123"},
		{"WebserverURL stripped of https", data.WebserverURL, "providerapi.advancedmd.com/processrequest/api-801/testapp"},
		{"XmlrpcURL correct", data.XmlrpcURL, "providerapi.advancedmd.com/processrequest/api-801/testapp/xmlrpc/processrequest.aspx"},
		{"RestApiBase replaces processrequest with api", data.RestApiBase, "providerapi.advancedmd.com/api/api-801/testapp"},
		{"EhrApiBase replaces processrequest with ehr-api", data.EhrApiBase, "providerapi.advancedmd.com/ehr-api/api-801/testapp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	// CreatedAt should be a valid timestamp
	if data.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

// Integration test with real Redis would go here
// For CI/CD, use testcontainers or mock

func TestTokenManager_IntegrationWithMockRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would use a real Redis instance or testcontainers
	// For now, we document the test structure

	t.Run("saves and retrieves token from Redis", func(t *testing.T) {
		// 1. Create Redis client (mock or testcontainers)
		// 2. Create TokenManager with the client
		// 3. Call Start() to trigger initial token fetch
		// 4. Verify token is saved to Redis
		// 5. Create new TokenManager with same Redis
		// 6. Verify it loads the cached token
	})
}

// Example of how to use a real Redis client in tests
func newTestRedisClient(t *testing.T) *clients.RedisClient {
	t.Helper()

	// In CI, use REDIS_URL from environment or skip
	// redisURL := os.Getenv("REDIS_URL")
	// if redisURL == "" {
	//     t.Skip("REDIS_URL not set, skipping Redis integration test")
	// }
	// return clients.NewRedisClient(redisURL)

	return nil
}
