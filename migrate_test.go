package matcha

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectOldInstall(t *testing.T) {
	t.Run("detects old install", func(t *testing.T) {
		// We can't easily test /opt detection without root,
		// so test reading old .env format directly
		dir := t.TempDir()
		oldDir := filepath.Join(dir, "opt", "fusionaly")
		os.MkdirAll(oldDir, 0755)
		os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
			"FUSIONALY_DOMAIN=app.example.com\nAPP_IMAGE=ghcr.io/karloscodes/fusionaly:latest\nPROXY_IMAGE=basecamp/kamal-proxy:latest\nFUSIONALY_PRIVATE_KEY=abc123\n",
		), 0600)

		m := New(Config{
			Name:       "fusionaly",
			AppImage:   "ghcr.io/karloscodes/fusionaly:latest",
			InstallDir: oldDir,
		})

		// loadConfigFromEnv should work with old dir
		err := m.loadConfigFromEnv()
		if err != nil {
			t.Fatalf("failed to read old .env: %v", err)
		}
		if m.domain != "app.example.com" {
			t.Errorf("expected domain app.example.com, got %s", m.domain)
		}
	})
}

func TestMigrateToNewLayout(t *testing.T) {
	dir := t.TempDir()

	// Create old-style install
	oldDir := filepath.Join(dir, "opt", "fusionaly")
	os.MkdirAll(filepath.Join(oldDir, "storage", "backups"), 0755)
	os.MkdirAll(filepath.Join(oldDir, "logs"), 0755)
	os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
		"FUSIONALY_DOMAIN=app.example.com\nAPP_IMAGE=ghcr.io/karloscodes/fusionaly:latest\nPROXY_IMAGE=basecamp/kamal-proxy:latest\nFUSIONALY_PRIVATE_KEY=abc123\n",
	), 0600)
	os.WriteFile(filepath.Join(oldDir, "storage", "test.db"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(oldDir, "logs", "app.log"), []byte("log data"), 0644)

	// Set up new paths
	newBase := filepath.Join(dir, "etc", "matcha")
	newAppDir := filepath.Join(newBase, "apps", "fusionaly")

	m := New(Config{
		Name:       "fusionaly",
		AppImage:   "ghcr.io/karloscodes/fusionaly:latest",
		InstallDir: oldDir,
	})

	err := m.migrateToNewLayout(oldDir, newAppDir, newBase)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify .env was copied
	if _, err := os.Stat(filepath.Join(newAppDir, ".env")); os.IsNotExist(err) {
		t.Error("expected .env in new location")
	}

	// Verify app.json was created
	reg := &Registry{BaseDir: newBase}
	app, err := reg.Load("fusionaly")
	if err != nil {
		t.Fatalf("failed to load from registry: %v", err)
	}
	if app.Domain != "app.example.com" {
		t.Errorf("expected domain app.example.com, got %s", app.Domain)
	}
	if app.Image != "ghcr.io/karloscodes/fusionaly:latest" {
		t.Errorf("expected image ghcr.io/karloscodes/fusionaly:latest, got %s", app.Image)
	}

	// Verify storage was preserved
	if _, err := os.Stat(filepath.Join(newAppDir, "storage", "test.db")); os.IsNotExist(err) {
		t.Error("expected storage data to be preserved")
	}

	// Verify logs were preserved
	if _, err := os.Stat(filepath.Join(newAppDir, "logs", "app.log")); os.IsNotExist(err) {
		t.Error("expected logs to be preserved")
	}
}
