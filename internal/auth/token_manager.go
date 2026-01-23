package auth

import (
	"context"
	"log"
	"sync"
	"time"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

const (
	// refreshInterval is how often the background refresh runs (20 hours)
	refreshInterval = 20 * time.Hour
)

// TokenManager handles token caching and background refresh.
type TokenManager struct {
	authenticator *Authenticator
	redis         *clients.RedisClient

	mu        sync.RWMutex
	tokenData *domain.TokenData

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager(auth *Authenticator, redis *clients.RedisClient) *TokenManager {
	return &TokenManager{
		authenticator: auth,
		redis:         redis,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background token refresh goroutine.
// It immediately loads the cached token and starts periodic refresh.
func (tm *TokenManager) Start(ctx context.Context) error {
	// Try to load existing token from Redis
	if err := tm.loadFromCache(ctx); err != nil {
		log.Printf("Warning: failed to load token from cache: %v", err)
	}

	// If no cached token, get a fresh one
	if tm.tokenData == nil {
		if err := tm.refresh(ctx); err != nil {
			return err
		}
	}

	// Start background refresh goroutine
	tm.wg.Add(1)
	go tm.backgroundRefresh()

	return nil
}

// Stop gracefully stops the background refresh.
func (tm *TokenManager) Stop() {
	close(tm.stopCh)
	tm.wg.Wait()
}

// GetToken returns the current token data.
// If no token is cached, it performs an on-demand refresh.
func (tm *TokenManager) GetToken(ctx context.Context) (*domain.TokenData, error) {
	tm.mu.RLock()
	data := tm.tokenData
	tm.mu.RUnlock()

	if data != nil {
		return data, nil
	}

	// On-demand refresh if no token in memory
	if err := tm.refresh(ctx); err != nil {
		return nil, err
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tokenData, nil
}

// loadFromCache loads the token from Redis into memory.
func (tm *TokenManager) loadFromCache(ctx context.Context) error {
	data, err := tm.redis.GetToken(ctx)
	if err != nil {
		return err
	}

	if data != nil {
		tm.mu.Lock()
		tm.tokenData = data
		tm.mu.Unlock()
		log.Printf("Loaded token from cache (created: %s)", data.CreatedAt)
	}

	return nil
}

// refresh performs authentication and updates both memory and Redis cache.
func (tm *TokenManager) refresh(ctx context.Context) error {
	log.Println("Refreshing AdvancedMD token...")

	token, webserverURL, err := tm.authenticator.Authenticate()
	if err != nil {
		return err
	}

	data := domain.BuildTokenData(token, webserverURL)

	// Update memory cache
	tm.mu.Lock()
	tm.tokenData = data
	tm.mu.Unlock()

	// Update Redis cache
	if err := tm.redis.SaveToken(ctx, data); err != nil {
		log.Printf("Warning: failed to save token to Redis: %v", err)
		// Don't return error - we still have the token in memory
	}

	log.Printf("Token refreshed successfully (created: %s)", data.CreatedAt)
	return nil
}

// backgroundRefresh runs the periodic token refresh.
func (tm *TokenManager) backgroundRefresh() {
	defer tm.wg.Done()

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopCh:
			log.Println("Background token refresh stopped")
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if err := tm.refresh(ctx); err != nil {
				log.Printf("Background token refresh failed: %v", err)
			}
			cancel()
		}
	}
}
