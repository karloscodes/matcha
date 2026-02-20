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
	configPath := filepath.Join(tmpDir, "config.yml")

	// Write a config with private key in env
	cfg := &MatchaConfig{
		Apps: map[string]AppConfig{
			"testapp": {
				Image:  "test:latest",
				Domain: "test.example.com",
				Env:    map[string]string{"PRIVATE_KEY": "abc123"},
			},
		},
	}
	SaveMatchaConfigTo(cfg, configPath)

	m := New(Config{Name: "testapp", AppImage: "test:latest", ConfigPath: configPath})

	got := m.readPrivateKey()

	if got != "abc123" {
		t.Errorf("readPrivateKey() = %q, want abc123", got)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	cfg := &MatchaConfig{
		Apps: map[string]AppConfig{
			"testapp": {
				Image:      "pro:latest",
				Domain:     "upgraded.example.com",
				Port:       3000,
				HealthPath: "/healthz",
				Volumes:    []string{"/app/storage"},
				Env:        map[string]string{"PRIVATE_KEY": "key123"},
			},
		},
	}
	SaveMatchaConfigTo(cfg, configPath)

	m := New(Config{Name: "testapp", AppImage: "oss:latest", ConfigPath: configPath})

	err := m.loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if m.config.AppImage != "pro:latest" {
		t.Errorf("AppImage = %q, want 'pro:latest'", m.config.AppImage)
	}
	if m.domain != "upgraded.example.com" {
		t.Errorf("domain = %q, want 'upgraded.example.com'", m.domain)
	}
	if m.config.AppPort != 3000 {
		t.Errorf("AppPort = %d, want 3000", m.config.AppPort)
	}
	if m.config.HealthPath != "/healthz" {
		t.Errorf("HealthPath = %q, want '/healthz'", m.config.HealthPath)
	}
}

func TestSaveImageUpdatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	cfg := &MatchaConfig{
		Apps: map[string]AppConfig{
			"testapp": {
				Image:  "oss:latest",
				Domain: "test.example.com",
				Port:   8080,
			},
		},
	}
	SaveMatchaConfigTo(cfg, configPath)

	m := New(Config{Name: "testapp", AppImage: "pro:latest", ConfigPath: configPath})

	err := m.SaveImage()
	if err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}

	updated, err := LoadAppFrom(configPath, "testapp")
	if err != nil {
		t.Fatalf("LoadApp() error = %v", err)
	}
	if updated.Image != "pro:latest" {
		t.Errorf("Image = %q, want 'pro:latest'", updated.Image)
	}
	if updated.Domain != "test.example.com" {
		t.Errorf("Domain should be preserved, got %q", updated.Domain)
	}
}

func TestSetupConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	dataBase := filepath.Join(tmpDir, "data")

	m := New(Config{
		Name:        "testapp",
		AppImage:    "test:latest",
		AppPort:     8080,
		HealthPath:  "/up",
		Volumes:     []string{"/app/storage"},
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})
	m.domain = "test.example.com"

	err := m.setupConfig()
	if err != nil {
		t.Fatalf("setupConfig() error = %v", err)
	}

	// Verify config was written
	app, err := LoadAppFrom(configPath, "testapp")
	if err != nil {
		t.Fatalf("LoadApp() error = %v", err)
	}

	if app.Image != "test:latest" {
		t.Errorf("Image = %q, want test:latest", app.Image)
	}
	if app.Domain != "test.example.com" {
		t.Errorf("Domain = %q, want test.example.com", app.Domain)
	}
	if app.Env["PRIVATE_KEY"] == "" {
		t.Error("PRIVATE_KEY should be generated")
	}

	// Verify data dir was created
	storageDir := filepath.Join(dataBase, "testapp", "storage")
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		t.Errorf("expected data dir %s to be created", storageDir)
	}
}

func TestLoadConfigAutoMigratesFromMultiApp(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	// Create old multi-app layout at /etc/matcha/apps/{name}/
	// We simulate by creating the structure and pointing Migrate to it
	// But tryAutoMigrate uses hardcoded /etc/matcha paths, so we test
	// via migrateFromMultiApp directly then verify loadConfig works
	appDir := filepath.Join(tmpDir, "apps", "testapp")
	os.MkdirAll(appDir, 0755)

	// Write old app.json
	os.WriteFile(filepath.Join(appDir, "app.json"), []byte(`{
		"name": "testapp",
		"image": "myapp:v2",
		"domain": "app.example.com",
		"port": 8080,
		"health_path": "/up"
	}`), 0644)

	// Write old .env
	os.WriteFile(filepath.Join(appDir, ".env"), []byte("PRIVATE_KEY=secret123\nDATABASE_URL=sqlite:///app/db\n"), 0600)

	// Migrate manually (tryAutoMigrate uses hardcoded paths, can't test in unit)
	dataBase := filepath.Join(tmpDir, "data")
	m := New(Config{
		Name:        "testapp",
		AppImage:    "myapp:v1",
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})
	err := m.migrateFromMultiApp(appDir)
	if err != nil {
		t.Fatalf("migrateFromMultiApp() error = %v", err)
	}

	// Now loadConfig should succeed from YAML
	m2 := New(Config{
		Name:        "testapp",
		AppImage:    "myapp:v1",
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})
	err = m2.loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() after migration error = %v", err)
	}

	if m2.config.AppImage != "myapp:v2" {
		t.Errorf("AppImage = %q, want myapp:v2", m2.config.AppImage)
	}
	if m2.domain != "app.example.com" {
		t.Errorf("domain = %q, want app.example.com", m2.domain)
	}

	// Verify env vars were migrated
	app, _ := LoadAppFrom(configPath, "testapp")
	if app.Env["PRIVATE_KEY"] != "secret123" {
		t.Errorf("PRIVATE_KEY = %q, want secret123", app.Env["PRIVATE_KEY"])
	}
	if app.Env["DATABASE_URL"] != "sqlite:///app/db" {
		t.Errorf("DATABASE_URL = %q, want sqlite:///app/db", app.Env["DATABASE_URL"])
	}
}
