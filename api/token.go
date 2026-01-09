package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"advancedmd-token-management/pkg/advancedmd"
	"advancedmd-token-management/pkg/redis"
)

// TokenResponse is the JSON response for the token endpoint
type TokenResponse struct {
	Token        string `json:"token"`
	WebserverURL string `json:"webserverUrl"`
	CreatedAt    string `json:"createdAt"`
}

// ErrorResponse is the JSON response for errors
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// Handler is the Vercel serverless function handler for /api/token
// This endpoint is called by ElevenLabs agents to get a valid AdvancedMD token
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Verify API secret
	auth := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + os.Getenv("API_SECRET")
	if auth != expectedAuth {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Try to get cached token from Redis
	tokenData, err := redis.GetToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get token from cache",
			Details: err.Error(),
		})
		return
	}

	// If no cached token, refresh on-demand
	if tokenData == nil {
		token, webserverURL, err := advancedmd.Authenticate()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "Authentication failed",
				Details: err.Error(),
			})
			return
		}

		// Save to Redis
		if err := redis.SaveToken(token, webserverURL); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "Failed to cache token",
				Details: err.Error(),
			})
			return
		}

		// Return the newly fetched token
		json.NewEncoder(w).Encode(TokenResponse{
			Token:        token,
			WebserverURL: webserverURL,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// Return cached token
	json.NewEncoder(w).Encode(TokenResponse{
		Token:        tokenData.Token,
		WebserverURL: tokenData.WebserverURL,
		CreatedAt:    tokenData.CreatedAt,
	})
}
