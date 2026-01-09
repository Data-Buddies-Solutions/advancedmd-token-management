package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"advancedmd-token-management/pkg/advancedmd"
	"advancedmd-token-management/pkg/redis"
)

// CronResponse is the JSON response for the cron endpoint
type CronResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	WebserverURL string `json:"webserverUrl,omitempty"`
}

// CronErrorResponse is the JSON response for cron errors
type CronErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// CronHandler is the Vercel serverless function handler for /api/cron
// This endpoint is triggered by Vercel Cron every 20 hours to refresh the token
func CronHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow GET requests (Vercel Cron uses GET)
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CronErrorResponse{Error: "Method not allowed"})
		return
	}

	// Verify cron secret
	// Vercel sends the secret in the Authorization header
	auth := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + os.Getenv("CRON_SECRET")
	if auth != expectedAuth {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(CronErrorResponse{Error: "Unauthorized"})
		return
	}

	// Perform 2-step AdvancedMD authentication
	token, webserverURL, err := advancedmd.Authenticate()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CronErrorResponse{
			Error:   "Authentication failed",
			Details: err.Error(),
		})
		return
	}

	// Save token to Redis with 23-hour TTL
	if err := redis.SaveToken(token, webserverURL); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CronErrorResponse{
			Error:   "Failed to save token to cache",
			Details: err.Error(),
		})
		return
	}

	// Success response
	json.NewEncoder(w).Encode(CronResponse{
		Success:      true,
		Message:      "Token refreshed successfully",
		WebserverURL: webserverURL,
	})
}
