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
