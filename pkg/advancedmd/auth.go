// Package advancedmd handles authentication with the AdvancedMD API.
// It implements the 2-step login process required to obtain a session token
// for subsequent API calls.
//
// Authentication Flow:
//  1. POST credentials to partnerlogin.advancedmd.com (returns webserver URL)
//  2. POST credentials to the webserver URL (returns session token)
//
// The webserver URL varies by account and is dynamically assigned by AdvancedMD.
// Tokens are typically valid for 24 hours but should be refreshed every 20-23 hours.
package advancedmd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// PPMDResults represents the XML response structure from AdvancedMD API.
// This is the root element returned by all login requests.
//
// Example successful response:
//
//	<PPMDResults>
//	  <Results success="1">
//	    <usercontext webserver="https://...">TOKEN_HERE</usercontext>
//	  </Results>
//	</PPMDResults>
type PPMDResults struct {
	XMLName xml.Name `xml:"PPMDResults"`
	Results Results  `xml:"Results"`
	Error   Error    `xml:"Error"`
}

// Results contains the login response data including success status and user context.
// The success attribute is "1" for successful authentication, "0" for step 1 redirect.
type Results struct {
	// Success is "1" for successful auth, "0" for step 1 (which still contains webserver URL)
	Success string `xml:"success,attr"`
	// UserContext contains the webserver URL and/or session token
	UserContext UserContext `xml:"usercontext"`
}

// UserContext contains authentication details returned from AdvancedMD.
// Step 1 returns the webserver URL in the attribute.
// Step 2 returns the session token as the element's text content.
type UserContext struct {
	// Webserver is the account-specific API endpoint URL (returned in step 1)
	Webserver string `xml:"webserver,attr"`
	// Token is the session token (returned as chardata in step 2)
	Token string `xml:",chardata"`
}

// Error contains error information from failed AdvancedMD requests.
// Note: Step 1 returns success="0" with an error, but this is expected behavior.
type Error struct {
	Fault Fault `xml:"Fault"`
}

// Fault contains detailed error information including code and description.
// Common error code -2147220476 in step 1 indicates redirect required (normal).
type Fault struct {
	// Code is the numeric error code from AdvancedMD
	Code string `xml:"detail>code"`
	// Description is the human-readable error message
	Description string `xml:"detail>description"`
}

// buildLoginXML creates the XML payload for AdvancedMD login requests.
// The same payload is used for both step 1 (get webserver) and step 2 (get token).
//
// Credentials are read from environment variables:
//   - ADVANCEDMD_USERNAME: API username
//   - ADVANCEDMD_PASSWORD: API password
//   - ADVANCEDMD_OFFICE_KEY: Office code (e.g., "991NNN")
//   - ADVANCEDMD_APP_NAME: Registered application name
//
// Returns XML in ppmdmsg format:
//
//	<ppmdmsg action="login" class="login" msgtime="..." username="..." psw="..." officecode="..." appname="..."/>
func buildLoginXML() string {
	// AdvancedMD requires timestamps in this specific format: M/D/YYYY H:MM:SS PM
	now := time.Now().Format("1/2/2006 3:04:05 PM")
	return fmt.Sprintf(
		`<ppmdmsg action="login" class="login" msgtime="%s" username="%s" psw="%s" officecode="%s" appname="%s"/>`,
		now,
		os.Getenv("ADVANCEDMD_USERNAME"),
		os.Getenv("ADVANCEDMD_PASSWORD"),
		os.Getenv("ADVANCEDMD_OFFICE_KEY"),
		os.Getenv("ADVANCEDMD_APP_NAME"),
	)
}

// parseXMLResponse parses AdvancedMD XML responses with charset support.
// AdvancedMD servers may return XML with ISO-8859-1 encoding declaration,
// so we use a charset reader to handle encoding conversion.
//
// Parameters:
//   - body: Raw XML response bytes from AdvancedMD
//
// Returns:
//   - *PPMDResults: Parsed response structure
//   - error: Non-nil if XML parsing fails
func parseXMLResponse(body []byte) (*PPMDResults, error) {
	var result PPMDResults
	// Create decoder with charset support for ISO-8859-1 and other encodings
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}
	return &result, nil
}

// GetWebserver performs Step 1 of the AdvancedMD login process.
// This step sends credentials to the central partner login endpoint and receives
// the account-specific webserver URL to use for step 2.
//
// Important: This step intentionally returns success="0" with error code -2147220476,
// but the response still contains the webserver URL we need. This is expected behavior
// in AdvancedMD's authentication flow.
//
// Returns:
//   - string: The webserver URL (e.g., "https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP")
//   - error: Non-nil if the request fails or no webserver URL is returned
func GetWebserver() (string, error) {
	// Central partner login endpoint - same for all AdvancedMD accounts
	url := "https://partnerlogin.advancedmd.com/practicemanager/xmlrpc/processrequest.aspx"

	// Use 30-second timeout to handle slow AdvancedMD responses
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/xml", strings.NewReader(buildLoginXML()))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	result, err := parseXMLResponse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Extract webserver URL from usercontext attribute
	// Note: success="0" is expected here - we're looking for the webserver URL, not a token
	webserver := result.Results.UserContext.Webserver
	if webserver == "" {
		return "", fmt.Errorf("no webserver URL in response. Error: %s", result.Error.Fault.Description)
	}

	return webserver, nil
}

// GetAuthToken performs Step 2 of the AdvancedMD login process.
// This step sends credentials to the account-specific webserver URL obtained
// from step 1 and receives the actual session token.
//
// Unlike step 1, this step requires success="1" for a valid token.
// The token is returned as the text content of the <usercontext> element.
//
// Parameters:
//   - webserverURL: The webserver URL from GetWebserver() (step 1)
//
// Returns:
//   - string: The session token to use for API authentication
//   - error: Non-nil if authentication fails or no token is returned
func GetAuthToken(webserverURL string) (string, error) {
	// Build the full XMLRPC endpoint URL
	// Example: https://providerapi.advancedmd.com/processrequest/api-801/YOURAPP/xmlrpc/processrequest.aspx
	url := webserverURL + "/xmlrpc/processrequest.aspx"

	// Use 30-second timeout to handle slow AdvancedMD responses
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/xml", strings.NewReader(buildLoginXML()))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	result, err := parseXMLResponse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Verify success="1" - unlike step 1, step 2 must succeed
	if result.Results.Success != "1" {
		return "", fmt.Errorf("login failed: success=%s, error=%s",
			result.Results.Success, result.Error.Fault.Description)
	}

	// Token is the TEXT content (chardata) of <usercontext>, not an attribute
	token := strings.TrimSpace(result.Results.UserContext.Token)
	if token == "" {
		return "", fmt.Errorf("no token in response")
	}

	return token, nil
}

// Authenticate performs the complete 2-step AdvancedMD authentication flow.
// This is the main entry point for obtaining authentication credentials.
//
// Flow:
//  1. GetWebserver() - Get account-specific webserver URL from partner login
//  2. GetAuthToken() - Get session token from webserver
//
// The returned webserver URL is important because it's used as the base URL
// for all subsequent API calls (XMLRPC, REST, EHR).
//
// Returns:
//   - token: Session token for API authentication (use in Cookie header)
//   - webserverURL: Base URL for building API endpoint URLs
//   - err: Non-nil if either step fails
//
// Usage:
//
//	token, webserverURL, err := advancedmd.Authenticate()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use token in Cookie header: Cookie: token={token}
//	// Build API URLs from webserverURL
func Authenticate() (token, webserverURL string, err error) {
	// Step 1: Get webserver URL from partner login endpoint
	// This tells us which server to authenticate against
	webserverURL, err = GetWebserver()
	if err != nil {
		return "", "", fmt.Errorf("step 1 (get webserver) failed: %w", err)
	}

	// Step 2: Get session token from the account-specific webserver
	// This is the actual authentication that returns a usable token
	token, err = GetAuthToken(webserverURL)
	if err != nil {
		return "", "", fmt.Errorf("step 2 (get token) failed: %w", err)
	}

	return token, webserverURL, nil
}
