package matcha

import (
	"os"
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
		CaddyImage: "caddy:2.9-alpine",
		InstallDir: tmpDir,
	})

	data := &envData{
		Domain:     "test.example.com",
		PrivateKey: "abc123def456",
		AppImage:   "test:latest",
		CaddyImage: "caddy:2.9-alpine",
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
	if got.AppImage != data.AppImage {
		t.Errorf("AppImage = %q, want %q", got.AppImage, data.AppImage)
	}
	if got.CaddyImage != data.CaddyImage {
		t.Errorf("CaddyImage = %q, want %q", got.CaddyImage, data.CaddyImage)
	}
}

func TestLoadConfigOverridesFromEnv(t *testing.T) {
	tmpDir := t.TempDir()

	// Create matcha with default images
	m := New(Config{
		Name:       "testapp",
		AppImage:   "oss:latest",
		CaddyImage: "caddy:2.7-alpine",
		InstallDir: tmpDir,
	})

	// Write .env with different images (simulating upgrade scenario)
	envPath := tmpDir + "/.env"
	envContent := "TESTAPP_DOMAIN=upgraded.example.com\nAPP_IMAGE=pro:latest\nCADDY_IMAGE=caddy:2.9-alpine\nTESTAPP_PRIVATE_KEY=key123\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}

	// loadConfig should override config values from .env
	err := m.loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if m.config.AppImage != "pro:latest" {
		t.Errorf("AppImage = %q, want 'pro:latest'", m.config.AppImage)
	}
	if m.config.CaddyImage != "caddy:2.9-alpine" {
		t.Errorf("CaddyImage = %q, want 'caddy:2.9-alpine'", m.config.CaddyImage)
	}
	if m.domain != "upgraded.example.com" {
		t.Errorf("domain = %q, want 'upgraded.example.com'", m.domain)
	}
}

func TestSaveImagePersistsChange(t *testing.T) {
	tmpDir := t.TempDir()

	m := New(Config{
		Name:       "testapp",
		AppImage:   "oss:latest",
		CaddyImage: "caddy:2.9-alpine",
		InstallDir: tmpDir,
	})

	// Create initial .env
	envPath := tmpDir + "/.env"
	envContent := "TESTAPP_DOMAIN=test.example.com\nAPP_IMAGE=oss:latest\nCADDY_IMAGE=caddy:2.9-alpine\nTESTAPP_PRIVATE_KEY=key456\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Change image and save
	m.SetImage("pro:latest")
	if err := m.SaveImage(); err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}

	// Read back and verify
	data, err := m.readEnv()
	if err != nil {
		t.Fatalf("readEnv() error = %v", err)
	}

	if data.AppImage != "pro:latest" {
		t.Errorf("AppImage after SaveImage = %q, want 'pro:latest'", data.AppImage)
	}
	// Other fields should be preserved
	if data.Domain != "test.example.com" {
		t.Errorf("Domain should be preserved, got %q", data.Domain)
	}
	if data.PrivateKey != "key456" {
		t.Errorf("PrivateKey should be preserved, got %q", data.PrivateKey)
	}
}

func TestReadEnvWithLegacyFields(t *testing.T) {
	tmpDir := t.TempDir()

	m := New(Config{
		Name:       "testapp",
		InstallDir: tmpDir,
	})

	// Write .env with legacy fields from old installer
	envPath := tmpDir + "/.env"
	envContent := `TESTAPP_DOMAIN=legacy.example.com
APP_IMAGE=legacyapp:v1
CADDY_IMAGE=caddy:2.7-alpine
INSTALL_DIR=/opt/testapp
BACKUP_PATH=/opt/testapp/storage/backups
VERSION=latest
INSTALLER_URL=https://github.com/example/releases/latest
TESTAPP_PRIVATE_KEY=legacykey
`
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Should read essential fields and ignore legacy ones
	data, err := m.readEnv()
	if err != nil {
		t.Fatalf("readEnv() error = %v", err)
	}

	if data.Domain != "legacy.example.com" {
		t.Errorf("Domain = %q, want 'legacy.example.com'", data.Domain)
	}
	if data.AppImage != "legacyapp:v1" {
		t.Errorf("AppImage = %q, want 'legacyapp:v1'", data.AppImage)
	}
	if data.CaddyImage != "caddy:2.7-alpine" {
		t.Errorf("CaddyImage = %q, want 'caddy:2.7-alpine'", data.CaddyImage)
	}
}
