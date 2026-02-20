package matcha

import (
	"os"
	"path/filepath"
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
		key, err := GeneratePrivateKey()
		if err != nil {
			t.Fatalf("GeneratePrivateKey() error = %v", err)
		}
		if len(key) != 64 {
			t.Errorf("GeneratePrivateKey() length = %d, want 64", len(key))
		}
	})

	t.Run("generates unique keys", func(t *testing.T) {
		keys := make(map[string]bool)
		for i := 0; i < 100; i++ {
			key, _ := GeneratePrivateKey()
			if keys[key] {
				t.Errorf("GeneratePrivateKey() produced duplicate key")
			}
			keys[key] = true
		}
	})
}

func TestReadPrivateKey(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

	// Write new-format .env with just PRIVATE_KEY
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("PRIVATE_KEY=abc123\n"), 0600)

	got := m.readPrivateKey()
	if got != "abc123" {
		t.Errorf("readPrivateKey() = %q, want abc123", got)
	}
}

func TestReadPrivateKeyLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

	// Write old-format .env with prefixed key
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("TESTAPP_PRIVATE_KEY=legacy123\n"), 0600)

	got := m.readPrivateKey()
	if got != "legacy123" {
		t.Errorf("readPrivateKey() = %q, want legacy123", got)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{
		Name:       "testapp",
		AppImage:   "oss:latest",
		ProxyImage: "basecamp/kamal-proxy:v0.8.1",
		InstallDir: tmpDir,
	})

	envPath := tmpDir + "/.env"
	envContent := "TESTAPP_DOMAIN=upgraded.example.com\nAPP_IMAGE=pro:latest\nPROXY_IMAGE=basecamp/kamal-proxy:v0.9.0\nTESTAPP_PRIVATE_KEY=key123\n"
	os.WriteFile(envPath, []byte(envContent), 0600)

	err := m.loadConfigFromEnv()
	if err != nil {
		t.Fatalf("loadConfigFromEnv() error = %v", err)
	}

	if m.config.AppImage != "pro:latest" {
		t.Errorf("AppImage = %q, want 'pro:latest'", m.config.AppImage)
	}
	if m.config.ProxyImage != "basecamp/kamal-proxy:v0.9.0" {
		t.Errorf("ProxyImage = %q, want 'basecamp/kamal-proxy:v0.9.0'", m.config.ProxyImage)
	}
	if m.domain != "upgraded.example.com" {
		t.Errorf("domain = %q, want 'upgraded.example.com'", m.domain)
	}
}

func TestSaveImageUpdatesRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	reg := &Registry{BaseDir: tmpDir}
	reg.Save(AppEntry{
		Name:   "testapp",
		Image:  "oss:latest",
		Domain: "test.example.com",
		Port:   8080,
	})

	// SaveImage hardcodes /etc/matcha, so we test registry directly
	app, err := reg.Load("testapp")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	app.Image = "pro:latest"
	if err := reg.Save(app); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	updated, err := reg.Load("testapp")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if updated.Image != "pro:latest" {
		t.Errorf("Image = %q, want 'pro:latest'", updated.Image)
	}
	if updated.Domain != "test.example.com" {
		t.Errorf("Domain should be preserved, got %q", updated.Domain)
	}
}

func TestReadEnvWithLegacyFields(t *testing.T) {
	tmpDir := t.TempDir()
	m := New(Config{
		Name:       "testapp",
		AppImage:   "test:latest",
		InstallDir: tmpDir,
	})

	envPath := tmpDir + "/.env"
	envContent := "TESTAPP_DOMAIN=legacy.example.com\nAPP_IMAGE=legacyapp:v1\nPROXY_IMAGE=basecamp/kamal-proxy:v0.8.0\nINSTALL_DIR=/opt/testapp\nTESTAPP_PRIVATE_KEY=legacykey\n"
	os.WriteFile(envPath, []byte(envContent), 0600)

	err := m.loadConfigFromEnv()
	if err != nil {
		t.Fatalf("loadConfigFromEnv() error = %v", err)
	}

	if m.domain != "legacy.example.com" {
		t.Errorf("Domain = %q, want 'legacy.example.com'", m.domain)
	}
	if m.config.AppImage != "legacyapp:v1" {
		t.Errorf("AppImage = %q, want 'legacyapp:v1'", m.config.AppImage)
	}
}
