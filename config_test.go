package matcha

import (
	"os"
	"strings"
	"testing"
)

func TestValidateDomain(t *testing.T) {
	m := New(Config{Name: "test", AppImage: "test:latest"})

	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "example.com", false},
		{"valid subdomain", "app.example.com", false},
		{"localhost", "localhost", false},
		{"with spaces", "example .com", true},
		{"with http", "http://example.com", true},
		{"with https", "https://example.com", true},
		{"no dot", "examplecom", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.validateDomain(tt.domain)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	t.Run("generates 64 char hex string", func(t *testing.T) {
		key, err := generatePrivateKey()

		if err != nil {
			t.Fatalf("generatePrivateKey() error = %v", err)
		}

		if len(key) != 64 {
			t.Errorf("generatePrivateKey() length = %d, want 64", len(key))
		}
	})

	t.Run("generates unique keys", func(t *testing.T) {
		keys := make(map[string]bool)

		for i := 0; i < 100; i++ {
			key, err := generatePrivateKey()
			if err != nil {
				t.Fatalf("generatePrivateKey() error = %v", err)
			}

			if keys[key] {
				t.Errorf("generatePrivateKey() produced duplicate key")
			}
			keys[key] = true
		}
	})
}

func TestSaveAndReadEnv(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{
		Name:       "testapp",
		AppImage:   "test:latest",
		InstallDir: tmpDir,
	})

	data := &envData{
		Domain:     "test.example.com",
		PrivateKey: "abc123def456",
	}

	err := m.saveEnv(data)
	if err != nil {
		t.Fatalf("saveEnv() error = %v", err)
	}

	got, err := m.readEnv()
	if err != nil {
		t.Fatalf("readEnv() error = %v", err)
	}

	if got.Domain != data.Domain {
		t.Errorf("Domain = %q, want %q", got.Domain, data.Domain)
	}

	if got.PrivateKey != data.PrivateKey {
		t.Errorf("PrivateKey = %q, want %q", got.PrivateKey, data.PrivateKey)
	}
}

func TestSaveEnvPreservesUnknownLines(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{
		Name:       "testapp",
		AppImage:   "test:latest",
		InstallDir: tmpDir,
	})

	// Write initial .env with custom line
	envPath := tmpDir + "/.env"
	initial := "TESTAPP_DOMAIN=old.example.com\nTESTAPP_PRIVATE_KEY=oldkey123\nCUSTOM_VAR=preserved\n"
	if err := os.WriteFile(envPath, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}

	// Save new data
	data := &envData{
		Domain:     "new.example.com",
		PrivateKey: "newkey456",
	}
	if err := m.saveEnv(data); err != nil {
		t.Fatalf("saveEnv() error = %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)

	// Domain should be updated
	if !strings.Contains(contentStr, "TESTAPP_DOMAIN=new.example.com") {
		t.Error("Domain was not updated")
	}

	// Private key should be preserved (not overwritten)
	if !strings.Contains(contentStr, "TESTAPP_PRIVATE_KEY=oldkey123") {
		t.Error("Private key was overwritten instead of preserved")
	}

	// Custom var should be preserved
	if !strings.Contains(contentStr, "CUSTOM_VAR=preserved") {
		t.Error("Custom variable was not preserved")
	}
}
