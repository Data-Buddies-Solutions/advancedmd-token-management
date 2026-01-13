// Package handler contains Vercel serverless function handlers for the
// AdvancedMD Token Management Service.
//
// This file implements the /api/cron endpoint which is triggered by Vercel Cron
// every 20 hours to proactively refresh AdvancedMD tokens before they expire.
package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"advancedmd-token-management/pkg/advancedmd"
	"advancedmd-token-management/pkg/redis"
)

// CronResponse is the JSON response structure for successful cron execution.
// Provides confirmation of token refresh along with the new webserver URL.
type CronResponse struct {
	// Success indicates whether the token refresh completed successfully.
	Success bool `json:"success"`

	// Message provides a human-readable description of the result.
	Message string `json:"message"`

	// WebserverURL is the base URL from the new token (for verification).
	// Omitted if the refresh failed.
	WebserverURL string `json:"webserverUrl,omitempty"`
}

// CronErrorResponse is the JSON response structure for cron errors.
// Provides both a brief error message and optional technical details.
type CronErrorResponse struct {
	// Error is a brief description of what went wrong.
	Error string `json:"error"`

	// Details contains additional technical information for debugging.
	// Only included when there's useful context (omitted if empty).
	Details string `json:"details,omitempty"`
}

// buildCronTokenData creates a complete TokenData struct with all pre-built URLs.
// This mirrors the buildTokenData function in token.go to ensure consistency
// between cron-refreshed and on-demand tokens.
//
// Parameters:
//   - token: AdvancedMD session token from authentication
//   - webserverURL: Base URL returned from AdvancedMD login
//
// Returns:
//   - *redis.TokenData: Complete token data ready for caching
func buildCronTokenData(token, webserverURL string) *redis.TokenData {
	return &redis.TokenData{
		Token:        token,
		WebserverURL: stripProtocol(webserverURL),
		// Build XMLRPC URL: append /xmlrpc/processrequest.aspx to webserver URL
		// Used for: addpatient, getpatient, scheduling (ppmdmsg operations)
		XmlrpcURL: stripProtocol(webserverURL + "/xmlrpc/processrequest.aspx"),
		// Build REST API base: replace "processrequest" with "api"
		// Used for: profiles, master files (Practice Manager REST API)
		RestApiBase: stripProtocol(replaceURLSegment(webserverURL, "/processrequest/", "/api/")),
		// Build EHR API base: replace "processrequest" with "ehr-api"
		// Used for: documents, files (Electronic Health Records REST API)
		EhrApiBase: stripProtocol(replaceURLSegment(webserverURL, "/processrequest/", "/ehr-api/")),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

// stripProtocol removes the https:// prefix from a URL for use in ElevenLabs templates.
func stripProtocol(url string) string {
	return strings.TrimPrefix(url, "https://")
}

// replaceURLSegment performs a string replacement in the URL path.
// This is a helper function to transform the webserver URL into different API bases.
//
// Parameters:
//   - url: The original URL to modify
//   - old: The substring to replace
//   - new: The replacement substring
//
// Returns:
//   - Modified URL with the replacement applied
func replaceURLSegment(url, old, new string) string {
	// Simple string replacement - only replaces first occurrence
	for i := 0; i <= len(url)-len(old); i++ {
		if url[i:i+len(old)] == old {
			return url[:i] + new + url[i+len(old):]
		}
	}
	return url
}

// Handler is the Vercel serverless function handler for the /api/cron endpoint.
// This endpoint is triggered by Vercel Cron every 20 hours to proactively
// refresh AdvancedMD tokens before they expire (23-hour TTL provides 3-hour buffer).
//
// Authentication:
//   - Requires Bearer token in Authorization header
//   - Token must match CRON_SECRET environment variable
//   - Vercel Cron automatically sends this header when configured
//
// Process:
//  1. Validates the cron secret to prevent unauthorized refreshes
//  2. Performs AdvancedMD 2-step authentication to get fresh token
//  3. Builds complete token data with all pre-built API URLs
//  4. Saves to Redis with 23-hour TTL
//
// HTTP Methods:
//   - GET: Refreshes token (success: 200, auth fail: 401, error: 500)
//   - Other methods: Returns 405 Method Not Allowed
//
// Schedule:
//
//	Configured in vercel.json as: "0 */20 * * *" (every 20 hours)
func Handler(w http.ResponseWriter, r *http.Request) {
	// Set JSON content type for all responses
	w.Header().Set("Content-Type", "application/json")

	// Only allow GET requests - Vercel Cron uses GET method
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CronErrorResponse{Error: "Method not allowed"})
		return
	}

	// Verify cron secret to prevent unauthorized token refreshes
	// Vercel automatically includes this header when triggering cron jobs
	auth := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + os.Getenv("CRON_SECRET")
	if auth != expectedAuth {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(CronErrorResponse{Error: "Unauthorized"})
		return
	}

	// Step 1: Perform AdvancedMD 2-step authentication
	// This gets a fresh token regardless of whether one is already cached
	token, webserverURL, err := advancedmd.Authenticate()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CronErrorResponse{
			Error:   "Authentication failed",
			Details: err.Error(),
		})
		return
	}

	// Step 2: Build complete token data with all pre-built URLs
	// This ensures ElevenLabs can use the URLs directly without string manipulation
	tokenData := buildCronTokenData(token, webserverURL)

	// Step 3: Save token to Redis with 23-hour TTL
	// The 20-hour cron schedule + 23-hour TTL provides a 3-hour buffer
	if err := redis.SaveToken(tokenData); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CronErrorResponse{
			Error:   "Failed to save token to cache",
			Details: err.Error(),
		})
		return
	}

	// Success response with the webserver URL for verification
	json.NewEncoder(w).Encode(CronResponse{
		Success:      true,
		Message:      "Token refreshed successfully",
		WebserverURL: webserverURL,
	})
}
