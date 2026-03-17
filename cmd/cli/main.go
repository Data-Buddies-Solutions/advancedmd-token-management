package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"advancedmd-token-management/internal/auth"
	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"
)

// app holds shared dependencies initialized once for all commands.
var app struct {
	authenticator *auth.Authenticator
	tokenData     *domain.TokenData
	amdClient     *clients.AdvancedMDClient
	amdRestClient *clients.AdvancedMDRestClient
}

var (
	bootstrapOnce sync.Once
	bootstrapErr  error
)

// eastern is the America/New_York timezone.
var eastern *time.Location

// tokenCachePath is where the CLI caches tokens locally.
var tokenCachePath string

const tokenMaxAge = 20 * time.Hour

func init() {
	var err error
	eastern, err = time.LoadLocation("America/New_York")
	if err != nil {
		eastern = time.FixedZone("EST", -5*3600)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	tokenCachePath = filepath.Join(home, ".config", "amd", "token.json")
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	rootCmd := &cobra.Command{
		Use:   "amd",
		Short: "AdvancedMD CLI — interact with AdvancedMD APIs from the terminal",
		Long: `A command-line interface for AdvancedMD practice management operations.

Requires environment variables:
  ADVANCEDMD_USERNAME, ADVANCEDMD_PASSWORD, ADVANCEDMD_OFFICE_KEY, ADVANCEDMD_APP_NAME`,
		SilenceUsage: true,
	}

	rootCmd.AddCommand(tokenCmd())
	rootCmd.AddCommand(verifyCmd())
	rootCmd.AddCommand(addPatientCmd())
	rootCmd.AddCommand(availabilityCmd())
	rootCmd.AddCommand(appointmentsCmd())
	rootCmd.AddCommand(cancelCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func mustBootstrap() error {
	bootstrapOnce.Do(func() {
		bootstrapErr = bootstrap()
	})
	return bootstrapErr
}

func bootstrap() error {
	username := os.Getenv("ADVANCEDMD_USERNAME")
	password := os.Getenv("ADVANCEDMD_PASSWORD")
	officeKey := os.Getenv("ADVANCEDMD_OFFICE_KEY")
	appName := os.Getenv("ADVANCEDMD_APP_NAME")

	if username == "" || password == "" || officeKey == "" || appName == "" {
		return fmt.Errorf("missing required env vars: ADVANCEDMD_USERNAME, ADVANCEDMD_PASSWORD, ADVANCEDMD_OFFICE_KEY, ADVANCEDMD_APP_NAME")
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	app.authenticator = auth.NewAuthenticator(auth.Credentials{
		Username:  username,
		Password:  password,
		OfficeKey: officeKey,
		AppName:   appName,
	}, httpClient)

	// Load token from file cache or authenticate fresh
	tokenData, err := loadCachedToken()
	if err != nil || tokenData == nil {
		tokenData, err = refreshToken()
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}
	app.tokenData = tokenData

	app.amdClient = clients.NewAdvancedMDClient(httpClient)
	app.amdRestClient = clients.NewAdvancedMDRestClient(httpClient)

	return nil
}

// getToken returns the current token (used by all commands).
func getToken() *domain.TokenData {
	return app.tokenData
}

// loadCachedToken reads a token from the local file cache.
// Returns nil if the cache is missing, unreadable, or expired.
func loadCachedToken() (*domain.TokenData, error) {
	data, err := os.ReadFile(tokenCachePath)
	if err != nil {
		return nil, err
	}

	var tokenData domain.TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, err
	}

	createdAt, err := time.Parse(time.RFC3339, tokenData.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid cache timestamp")
	}

	if time.Since(createdAt) > tokenMaxAge {
		log.Printf("cached token expired, re-authenticating")
		return nil, nil
	}

	log.Printf("using cached token (created %s)", tokenData.CreatedAt)
	return &tokenData, nil
}

// refreshToken authenticates with AMD and saves the token to the local file cache.
func refreshToken() (*domain.TokenData, error) {
	log.Printf("authenticating with AdvancedMD...")

	token, webserverURL, err := app.authenticator.Authenticate()
	if err != nil {
		return nil, err
	}

	tokenData := domain.BuildTokenData(token, webserverURL)

	// Save to file cache
	if err := saveCachedToken(tokenData); err != nil {
		log.Printf("warning: could not cache token: %v", err)
	}

	return tokenData, nil
}

// saveCachedToken writes the token to the local file cache.
func saveCachedToken(tokenData *domain.TokenData) error {
	dir := filepath.Dir(tokenCachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.Marshal(tokenData)
	if err != nil {
		return err
	}

	return os.WriteFile(tokenCachePath, data, 0600)
}

// printJSON outputs a value as pretty-printed JSON to stdout.
func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

// friendlyProviderName maps AMD provider names to friendly display names.
func friendlyProviderName(amdName string) string {
	upper := strings.ToUpper(amdName)
	for _, entry := range []struct {
		match   string
		display string
	}{
		{"BACH", "Dr. Austin Bach"},
		{"LICHT", "Dr. J. Licht"},
		{"NOEL", "Dr. D. Noel"},
	} {
		if strings.Contains(upper, entry.match) {
			return entry.display
		}
	}
	return amdName
}
