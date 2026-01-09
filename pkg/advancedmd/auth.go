package advancedmd

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// PPMDResults represents the XML response from AdvancedMD API
type PPMDResults struct {
	XMLName xml.Name `xml:"PPMDResults"`
	Results Results  `xml:"Results"`
	Error   Error    `xml:"Error"`
}

// Results contains the login response data
type Results struct {
	Success     string      `xml:"success,attr"`
	UserContext UserContext `xml:"usercontext"`
}

// UserContext contains the webserver URL and token
type UserContext struct {
	Webserver string `xml:"webserver,attr"`
	Token     string `xml:",chardata"`
}

// Error contains error information from failed requests
type Error struct {
	Fault Fault `xml:"Fault"`
}

// Fault contains detailed error information
type Fault struct {
	Code        string `xml:"detail>code"`
	Description string `xml:"detail>description"`
}

// buildLoginXML creates the XML payload for AdvancedMD login
func buildLoginXML() string {
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

// GetWebserver performs Step 1 of the login process to get the redirect URL
// Note: This returns success="0" with error code -2147220476, but contains the webserver URL
func GetWebserver() (string, error) {
	url := "https://partnerlogin.advancedmd.com/practicemanager/xmlrpc/processrequest.aspx"

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

	var result PPMDResults
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Extract webserver from usercontext (even though success="0")
	webserver := result.Results.UserContext.Webserver
	if webserver == "" {
		return "", fmt.Errorf("no webserver URL in response. Error: %s", result.Error.Fault.Description)
	}

	return webserver, nil
}

// GetAuthToken performs Step 2 of the login process to get the session token
func GetAuthToken(webserverURL string) (string, error) {
	// Append /xmlrpc/processrequest.aspx to the webserver URL
	url := webserverURL + "/xmlrpc/processrequest.aspx"

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

	var result PPMDResults
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Check for success="1"
	if result.Results.Success != "1" {
		return "", fmt.Errorf("login failed: success=%s, error=%s",
			result.Results.Success, result.Error.Fault.Description)
	}

	// Token is the TEXT content of <usercontext>
	token := strings.TrimSpace(result.Results.UserContext.Token)
	if token == "" {
		return "", fmt.Errorf("no token in response")
	}

	return token, nil
}

// Authenticate performs the full 2-step AdvancedMD authentication
// Step 1: Login to get webserver URL (returns "error" with redirect info)
// Step 2: Login to webserver to get actual session token
func Authenticate() (token, webserverURL string, err error) {
	// Step 1: Get webserver URL
	webserverURL, err = GetWebserver()
	if err != nil {
		return "", "", fmt.Errorf("step 1 (get webserver) failed: %w", err)
	}

	// Step 2: Get token from webserver
	token, err = GetAuthToken(webserverURL)
	if err != nil {
		return "", "", fmt.Errorf("step 2 (get token) failed: %w", err)
	}

	return token, webserverURL, nil
}
