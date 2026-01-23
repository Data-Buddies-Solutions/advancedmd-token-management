package domain

import (
	"strings"
	"testing"
)

func TestBuildTokenData(t *testing.T) {
	token := "test-token-abc123"
	webserverURL := "https://providerapi.advancedmd.com/processrequest/api-801/myapp"

	data := BuildTokenData(token, webserverURL)

	t.Run("Token has Bearer prefix", func(t *testing.T) {
		if data.Token != "Bearer test-token-abc123" {
			t.Errorf("Expected 'Bearer test-token-abc123', got %q", data.Token)
		}
	})

	t.Run("CookieToken has token= prefix", func(t *testing.T) {
		if data.CookieToken != "token=test-token-abc123" {
			t.Errorf("Expected 'token=test-token-abc123', got %q", data.CookieToken)
		}
	})

	t.Run("WebserverURL has https stripped", func(t *testing.T) {
		expected := "providerapi.advancedmd.com/processrequest/api-801/myapp"
		if data.WebserverURL != expected {
			t.Errorf("Expected %q, got %q", expected, data.WebserverURL)
		}
	})

	t.Run("XmlrpcURL is correct", func(t *testing.T) {
		expected := "providerapi.advancedmd.com/processrequest/api-801/myapp/xmlrpc/processrequest.aspx"
		if data.XmlrpcURL != expected {
			t.Errorf("Expected %q, got %q", expected, data.XmlrpcURL)
		}
	})

	t.Run("RestApiBase replaces processrequest with api", func(t *testing.T) {
		expected := "providerapi.advancedmd.com/api/api-801/myapp"
		if data.RestApiBase != expected {
			t.Errorf("Expected %q, got %q", expected, data.RestApiBase)
		}
	})

	t.Run("EhrApiBase replaces processrequest with ehr-api", func(t *testing.T) {
		expected := "providerapi.advancedmd.com/ehr-api/api-801/myapp"
		if data.EhrApiBase != expected {
			t.Errorf("Expected %q, got %q", expected, data.EhrApiBase)
		}
	})

	t.Run("CreatedAt is set", func(t *testing.T) {
		if data.CreatedAt == "" {
			t.Error("CreatedAt should not be empty")
		}
	})
}

func TestTokenData_ToResponse(t *testing.T) {
	data := &TokenData{
		Token:        "Bearer test",
		CookieToken:  "token=test",
		WebserverURL: "example.com/processrequest/api-801/app",
		XmlrpcURL:    "example.com/processrequest/api-801/app/xmlrpc/processrequest.aspx",
		RestApiBase:  "example.com/api/api-801/app",
		EhrApiBase:   "example.com/ehr-api/api-801/app",
		CreatedAt:    "2024-01-15T10:30:00Z",
	}

	resp := data.ToResponse()

	if resp.Token != data.Token {
		t.Errorf("Token mismatch: got %q, want %q", resp.Token, data.Token)
	}
	if resp.CookieToken != data.CookieToken {
		t.Errorf("CookieToken mismatch: got %q, want %q", resp.CookieToken, data.CookieToken)
	}
	if resp.WebserverURL != data.WebserverURL {
		t.Errorf("WebserverURL mismatch: got %q, want %q", resp.WebserverURL, data.WebserverURL)
	}
	if resp.XmlrpcURL != data.XmlrpcURL {
		t.Errorf("XmlrpcURL mismatch: got %q, want %q", resp.XmlrpcURL, data.XmlrpcURL)
	}
	if resp.RestApiBase != data.RestApiBase {
		t.Errorf("RestApiBase mismatch: got %q, want %q", resp.RestApiBase, data.RestApiBase)
	}
	if resp.EhrApiBase != data.EhrApiBase {
		t.Errorf("EhrApiBase mismatch: got %q, want %q", resp.EhrApiBase, data.EhrApiBase)
	}
	if resp.CreatedAt != data.CreatedAt {
		t.Errorf("CreatedAt mismatch: got %q, want %q", resp.CreatedAt, data.CreatedAt)
	}
}

func TestTokenData_RawToken(t *testing.T) {
	data := &TokenData{Token: "Bearer my-secret-token"}

	raw := data.RawToken()
	if raw != "my-secret-token" {
		t.Errorf("Expected 'my-secret-token', got %q", raw)
	}

	// Test without Bearer prefix
	data2 := &TokenData{Token: "plain-token"}
	raw2 := data2.RawToken()
	if raw2 != "plain-token" {
		t.Errorf("Expected 'plain-token', got %q", raw2)
	}
}

func TestStripProtocol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com/path", "example.com/path"},
		{"http://example.com/path", "http://example.com/path"}, // Only strips https
		{"example.com/path", "example.com/path"},               // No prefix
	}

	for _, tt := range tests {
		got := stripProtocol(tt.input)
		if got != tt.expected {
			t.Errorf("stripProtocol(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestURLTransformations(t *testing.T) {
	webserverURL := "https://providerapi.advancedmd.com/processrequest/api-801/testapp"

	t.Run("buildXmlrpcURL", func(t *testing.T) {
		got := buildXmlrpcURL(webserverURL)
		if !strings.HasSuffix(got, "/xmlrpc/processrequest.aspx") {
			t.Errorf("Expected suffix '/xmlrpc/processrequest.aspx', got %q", got)
		}
		if strings.HasPrefix(got, "https://") {
			t.Error("Should not have https:// prefix")
		}
	})

	t.Run("buildRestApiBase", func(t *testing.T) {
		got := buildRestApiBase(webserverURL)
		if !strings.Contains(got, "/api/api-801/") {
			t.Errorf("Expected '/api/api-801/' in URL, got %q", got)
		}
		if strings.Contains(got, "/processrequest/") {
			t.Error("Should not contain /processrequest/")
		}
	})

	t.Run("buildEhrApiBase", func(t *testing.T) {
		got := buildEhrApiBase(webserverURL)
		if !strings.Contains(got, "/ehr-api/api-801/") {
			t.Errorf("Expected '/ehr-api/api-801/' in URL, got %q", got)
		}
		if strings.Contains(got, "/processrequest/") {
			t.Error("Should not contain /processrequest/")
		}
	})
}
