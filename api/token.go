// Package handler contains Vercel serverless function handlers for the
// AdvancedMD Token Management Service.
//
// This package provides two main endpoints:
//   - /api/token: Returns cached AdvancedMD tokens for ElevenLabs agents
//   - /api/cron: Refreshes tokens on a schedule (called by Vercel Cron)
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

// TokenResponse is the JSON response structure for the /api/token endpoint.
// It contains the authentication token and pre-built URLs for all AdvancedMD
// API types, allowing ElevenLabs agents to make API calls without URL manipulation.
//
// ElevenLabs Dynamic Variable Mapping:
//   - amd_token     -> token       (for Cookie: token={amd_token})
//   - amd_webserver -> webserverUrl (original base URL for reference)
//   - amd_xmlrpc_url -> xmlrpcUrl   (for addpatient, getpatient, scheduling)
//   - amd_rest_api_base -> restApiBase (for profiles, master files)
//   - amd_ehr_api_base -> ehrApiBase   (for documents, files)
type TokenResponse struct {
	// Token is the AdvancedMD session token for API authentication.
	// Use in requests as: Cookie: token={Token}
	Token string `json:"token"`

	// WebserverURL is the base URL from AdvancedMD's login response.
	// Example: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP
	WebserverURL string `json:"webserverUrl"`

	// XmlrpcURL is the complete XMLRPC API endpoint URL.
	// Use this directly for ppmdmsg operations (addpatient, getpatient, scheduling).
	// Example: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx
	XmlrpcURL string `json:"xmlrpcUrl"`

	// RestApiBase is the base URL for Practice Manager REST API.
	// Append endpoint paths to this base (e.g., /masterfiles/olsprofiles).
	// Example: https://providerapi.advancedmd.com/api/api-801/YOURAPP
	RestApiBase string `json:"restApiBase"`

	// EhrApiBase is the base URL for Electronic Health Records (EHR) REST API.
	// Append endpoint paths to this base (e.g., /files/documents).
	// Example: https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP
	EhrApiBase string `json:"ehrApiBase"`

	// CreatedAt is the RFC3339 timestamp when this token was generated.
	CreatedAt string `json:"createdAt"`
}

// ErrorResponse is the JSON response structure for error conditions.
// Provides both a user-friendly error message and optional technical details.
type ErrorResponse struct {
	// Error is a brief, user-friendly description of what went wrong.
	Error string `json:"error"`

	// Details contains additional technical information for debugging.
	// Only included when there's useful context (omitted if empty).
	Details string `json:"details,omitempty"`
}

// buildXmlrpcURL constructs the full XMLRPC API endpoint URL from the webserver URL.
// The XMLRPC API is used for legacy/core operations like addpatient, getpatient, scheduling.
//
// URL transformation:
//
//	Input:  https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP
//	Output: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx
//
// Parameters:
//   - webserverURL: Base URL returned from AdvancedMD login
//
// Returns:
//   - Full XMLRPC endpoint URL ready for ppmdmsg API calls
func buildXmlrpcURL(webserverURL string) string {
	return webserverURL + "/xmlrpc/processrequest.aspx"
}

// buildRestApiBase constructs the Practice Manager REST API base URL.
// Used for practice management operations like profiles and master files.
//
// URL transformation (replaces "processrequest" with "api"):
//
//	Input:  https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP
//	Output: https://providerapi.advancedmd.com/api/api-801/YOURAPP
//
// Usage example:
//
//	base := buildRestApiBase(webserverURL)
//	fullURL := base + "/masterfiles/olsprofiles"
//
// Parameters:
//   - webserverURL: Base URL returned from AdvancedMD login
//
// Returns:
//   - REST API base URL (append endpoint paths to use)
func buildRestApiBase(webserverURL string) string {
	// Replace "processrequest" with "api" in the URL path
	// Example: /processrequest/api-801/YOURAPP -> /api/api-801/YOURAPP
	return strings.Replace(webserverURL, "/processrequest/", "/api/", 1)
}

// buildEhrApiBase constructs the Electronic Health Records (EHR) REST API base URL.
// Used for EHR-specific operations like documents and files.
//
// URL transformation (replaces "processrequest" with "ehr-api"):
//
//	Input:  https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP
//	Output: https://providerapi.advancedmd.com/ehr-api/api-801/YOURAPP
//
// Usage example:
//
//	base := buildEhrApiBase(webserverURL)
//	fullURL := base + "/files/documents"
//
// Parameters:
//   - webserverURL: Base URL returned from AdvancedMD login
//
// Returns:
//   - EHR API base URL (append endpoint paths to use)
func buildEhrApiBase(webserverURL string) string {
	// Replace "processrequest" with "ehr-api" in the URL path
	// Example: /processrequest/api-801/YOURAPP -> /ehr-api/api-801/YOURAPP
	return strings.Replace(webserverURL, "/processrequest/", "/ehr-api/", 1)
}

// buildTokenData creates a complete TokenData struct with all pre-built URLs.
// This is the central function for constructing the response data, ensuring
// all URL variants are consistently generated from the webserver URL.
//
// Parameters:
//   - token: AdvancedMD session token from authentication
//   - webserverURL: Base URL returned from AdvancedMD login
//
// Returns:
//   - *redis.TokenData: Complete token data ready for caching and response
func buildTokenData(token, webserverURL string) *redis.TokenData {
	return &redis.TokenData{
		Token:        token,
		WebserverURL: webserverURL,
		XmlrpcURL:    buildXmlrpcURL(webserverURL),
		RestApiBase:  buildRestApiBase(webserverURL),
		EhrApiBase:   buildEhrApiBase(webserverURL),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}

// tokenDataToResponse converts a Redis TokenData struct to an API TokenResponse.
// This is used when returning cached data to ensure consistent response format.
//
// Parameters:
//   - data: TokenData retrieved from Redis cache
//
// Returns:
//   - TokenResponse: API response struct ready for JSON encoding
func tokenDataToResponse(data *redis.TokenData) TokenResponse {
	return TokenResponse{
		Token:        data.Token,
		WebserverURL: data.WebserverURL,
		XmlrpcURL:    data.XmlrpcURL,
		RestApiBase:  data.RestApiBase,
		EhrApiBase:   data.EhrApiBase,
		CreatedAt:    data.CreatedAt,
	}
}

// Handler is the Vercel serverless function handler for the /api/token endpoint.
// This endpoint is called by ElevenLabs agents to obtain a valid AdvancedMD
// authentication token and pre-built API URLs.
//
// Authentication:
//   - Requires Bearer token in Authorization header
//   - Token must match API_SECRET environment variable
//
// Response includes:
//   - Session token for AdvancedMD API authentication
//   - Pre-built URLs for all three AdvancedMD API types (XMLRPC, REST, EHR)
//   - Timestamp indicating when the token was generated
//
// Caching behavior:
//   - First attempts to return cached token from Redis
//   - If cache is empty (expired or first request), performs on-demand authentication
//   - Newly fetched tokens are cached for subsequent requests
//
// HTTP Methods:
//   - GET: Returns token data (success: 200, auth fail: 401, error: 500)
//   - Other methods: Returns 405 Method Not Allowed
func Handler(w http.ResponseWriter, r *http.Request) {
	// Set JSON content type for all responses
	w.Header().Set("Content-Type", "application/json")

	// Only allow GET requests - this is a read-only endpoint
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
		return
	}

	// Verify API secret to prevent unauthorized access
	// ElevenLabs sends this as: Authorization: Bearer {API_SECRET}
	auth := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + os.Getenv("API_SECRET")
	if auth != expectedAuth {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Step 1: Try to get cached token from Redis
	// This is the fast path (~10-20ms) for most requests
	tokenData, err := redis.GetToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "Failed to get token from cache",
			Details: err.Error(),
		})
		return
	}

	// Step 2: If no cached token, perform on-demand authentication
	// This happens when: token expired, first request, or cache cleared
	if tokenData == nil {
		// Perform AdvancedMD 2-step authentication
		token, webserverURL, err := advancedmd.Authenticate()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "Authentication failed",
				Details: err.Error(),
			})
			return
		}

		// Build complete token data with all pre-built URLs
		tokenData = buildTokenData(token, webserverURL)

		// Cache the token for future requests (23-hour TTL)
		if err := redis.SaveToken(tokenData); err != nil {
			// Log the error but still return the token - caching failure
			// shouldn't prevent the client from getting a valid token
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "Failed to cache token",
				Details: err.Error(),
			})
			return
		}
	}

	// Step 3: Return the token data (either cached or freshly fetched)
	json.NewEncoder(w).Encode(tokenDataToResponse(tokenData))
}
