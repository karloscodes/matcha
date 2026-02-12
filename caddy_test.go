package matcha

import (
	"os"
	"strings"
	"testing"
)

func TestExtractBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{"simple domain", "example.com", "example.com"},
		{"subdomain", "app.example.com", "example.com"},
		{"deep subdomain", "a.b.c.example.com", "example.com"},
		{"localhost", "localhost", "localhost"},
		{"localhost with port", "localhost:8080", "localhost:8080"},
		{"single label", "intranet", "intranet"},
		{"mixed case", "App.Example.COM", "example.com"},
		{"with whitespace", "  example.com  ", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBaseDomain(tt.domain)

			if got != tt.expected {
				t.Errorf("extractBaseDomain(%q) = %q, want %q", tt.domain, got, tt.expected)
			}
		})
	}
}

func TestGenerateAdminEmail(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{"simple domain", "example.com", "admin@example.com"},
		{"subdomain", "app.example.com", "admin@example.com"},
		{"deep subdomain", "a.b.c.example.com", "admin@example.com"},
		{"localhost", "localhost", "admin@localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateAdminEmail(tt.domain)

			if got != tt.expected {
				t.Errorf("generateAdminEmail(%q) = %q, want %q", tt.domain, got, tt.expected)
			}
		})
	}
}

func TestGenerateCaddyfileForContainer(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{
		Name:       "testapp",
		AppImage:   "test:latest",
		InstallDir: tmpDir,
		HealthPath: "/_health",
		AppPort:    8080,
	})

	data := &envData{
		Domain:     "app.example.com",
		PrivateKey: "testkey",
	}

	err := m.generateCaddyfileForContainer(data, "testapp-app-1")
	if err != nil {
		t.Fatalf("generateCaddyfileForContainer() error = %v", err)
	}

	// Read generated Caddyfile
	content, err := os.ReadFile(tmpDir + "/Caddyfile")
	if err != nil {
		t.Fatalf("failed to read Caddyfile: %v", err)
	}

	contentStr := string(content)

	// Verify content
	if !strings.Contains(contentStr, "app.example.com") {
		t.Error("Caddyfile missing domain")
	}

	if !strings.Contains(contentStr, "testapp-app-1:8080") {
		t.Error("Caddyfile missing container reference")
	}

	if !strings.Contains(contentStr, "/_health") {
		t.Error("Caddyfile missing health path")
	}

	if !strings.Contains(contentStr, "admin@example.com") {
		t.Error("Caddyfile missing admin email for TLS")
	}
}
