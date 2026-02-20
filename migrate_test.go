package matcha

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "DATABASE_URL=sqlite:///data/app.db\nSECRET_KEY=abc123\n# comment\n\nEMPTY=\n"
	os.WriteFile(envPath, []byte(content), 0600)

	vars, err := readEnvFile(envPath)
	if err != nil {
		t.Fatalf("readEnvFile failed: %v", err)
	}

	if vars["DATABASE_URL"] != "sqlite:///data/app.db" {
		t.Errorf("expected DATABASE_URL=sqlite:///data/app.db, got %s", vars["DATABASE_URL"])
	}
	if vars["SECRET_KEY"] != "abc123" {
		t.Errorf("expected SECRET_KEY=abc123, got %s", vars["SECRET_KEY"])
	}
	if _, ok := vars["EMPTY"]; !ok {
		t.Error("expected EMPTY key to exist")
	}
}

func TestReadEnvFileNotFound(t *testing.T) {
	vars, err := readEnvFile("/nonexistent/.env")
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected empty map, got %v", vars)
	}
}

func TestMigrateFromMultiApp(t *testing.T) {
	dir := t.TempDir()

	// Create old multi-app layout
	appDir := filepath.Join(dir, "etc", "matcha", "apps", "fusionaly")
	os.MkdirAll(filepath.Join(appDir, "storage", "backups"), 0755)
	os.MkdirAll(filepath.Join(appDir, "logs"), 0755)

	// Write old app.json
	oldApp := oldAppJSON{
		Name:       "fusionaly",
		Image:      "ghcr.io/karloscodes/fusionaly:latest",
		Domain:     "app.example.com",
		Port:       8080,
		HealthPath: "/up",
	}
	data, _ := json.Marshal(oldApp)
	os.WriteFile(filepath.Join(appDir, "app.json"), data, 0644)

	// Write old .env
	os.WriteFile(filepath.Join(appDir, ".env"), []byte(
		"PRIVATE_KEY=abc123\nCUSTOM_VAR=hello\n",
	), 0600)

	// Write some data
	os.WriteFile(filepath.Join(appDir, "storage", "test.db"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(appDir, "logs", "app.log"), []byte("log data"), 0644)

	// Set up target config path and data dir
	configPath := filepath.Join(dir, "config.yml")
	dataBase := filepath.Join(dir, "var", "matcha")

	m := New(Config{
		Name:        "fusionaly",
		AppImage:    "ghcr.io/karloscodes/fusionaly:latest",
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})

	err := m.migrateFromMultiApp(appDir)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify YAML config was written
	app, err := LoadAppFrom(configPath, "fusionaly")
	if err != nil {
		t.Fatalf("failed to load from config: %v", err)
	}
	if app.Domain != "app.example.com" {
		t.Errorf("expected domain app.example.com, got %s", app.Domain)
	}
	if app.Image != "ghcr.io/karloscodes/fusionaly:latest" {
		t.Errorf("expected image ghcr.io/karloscodes/fusionaly:latest, got %s", app.Image)
	}
	if app.Env["PRIVATE_KEY"] != "abc123" {
		t.Errorf("expected PRIVATE_KEY=abc123, got %s", app.Env["PRIVATE_KEY"])
	}
	if app.Env["CUSTOM_VAR"] != "hello" {
		t.Errorf("expected CUSTOM_VAR=hello, got %s", app.Env["CUSTOM_VAR"])
	}
}

func TestMigrateFromLegacy(t *testing.T) {
	dir := t.TempDir()

	// Create old legacy layout
	oldDir := filepath.Join(dir, "opt", "fusionaly")
	os.MkdirAll(filepath.Join(oldDir, "storage"), 0755)
	os.MkdirAll(filepath.Join(oldDir, "logs"), 0755)

	os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
		"FUSIONALY_DOMAIN=app.example.com\nAPP_IMAGE=ghcr.io/karloscodes/fusionaly:latest\nFUSIONALY_PRIVATE_KEY=abc123\nCUSTOM=value\n",
	), 0600)
	os.WriteFile(filepath.Join(oldDir, "storage", "test.db"), []byte("data"), 0644)

	configPath := filepath.Join(dir, "config.yml")
	dataBase := filepath.Join(dir, "var", "matcha")

	m := New(Config{
		Name:        "fusionaly",
		AppImage:    "ghcr.io/karloscodes/fusionaly:latest",
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})

	err := m.migrateFromLegacy(oldDir)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	app, err := LoadAppFrom(configPath, "fusionaly")
	if err != nil {
		t.Fatalf("failed to load from config: %v", err)
	}
	if app.Image != "ghcr.io/karloscodes/fusionaly:latest" {
		t.Errorf("expected image, got %s", app.Image)
	}
	if app.Domain != "app.example.com" {
		t.Errorf("expected domain, got %s", app.Domain)
	}
	if app.Env["PRIVATE_KEY"] != "abc123" {
		t.Errorf("expected PRIVATE_KEY=abc123, got %s", app.Env["PRIVATE_KEY"])
	}
	if app.Env["CUSTOM"] != "value" {
		t.Errorf("expected CUSTOM=value, got %s", app.Env["CUSTOM"])
	}
}

func TestBuildMigrationEnv(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	vars := map[string]string{
		"PRIVATE_KEY":            "pk123",
		"FUSIONALY_DOMAIN":      "test.com",
		"FUSIONALY_PRIVATE_KEY": "old_pk",
		"APP_IMAGE":             "img:latest",
		"CUSTOM_VAR":            "hello",
		"DATABASE_URL":          "sqlite:///data/app.db",
	}

	env := m.buildMigrationEnv(vars)

	if env["PRIVATE_KEY"] != "pk123" {
		t.Errorf("PRIVATE_KEY = %q, want pk123", env["PRIVATE_KEY"])
	}
	if env["CUSTOM_VAR"] != "hello" {
		t.Errorf("CUSTOM_VAR = %q, want hello", env["CUSTOM_VAR"])
	}
	if env["DATABASE_URL"] != "sqlite:///data/app.db" {
		t.Errorf("DATABASE_URL = %q, want sqlite:///data/app.db", env["DATABASE_URL"])
	}
	if _, ok := env["FUSIONALY_DOMAIN"]; ok {
		t.Error("managed key FUSIONALY_DOMAIN should not be in env")
	}
	if _, ok := env["APP_IMAGE"]; ok {
		t.Error("managed key APP_IMAGE should not be in env")
	}
}
