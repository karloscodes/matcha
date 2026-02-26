package matcha

import (
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

// TestMigrateFromLegacy simulates a v0.11.x → v0.12 upgrade end-to-end.
//
// v0.11.x layout:
//
//	/opt/fusionaly/.env           (config: domain, image, env vars)
//	/opt/fusionaly/storage/app.db (the database)
//	/opt/fusionaly/logs/app.log
//
// After migration:
//
//	/etc/matcha/config.yml                  (YAML config)
//	/var/matcha/fusionaly/storage/app.db    (DB at same relative path)
//	/var/matcha/fusionaly/logs/app.log
//
// The container volume mount changes from /opt/fusionaly/storage:/app/storage
// to /var/matcha/fusionaly/storage:/app/storage — the container path stays the
// same so the app keeps working with its database.
func TestMigrateFromLegacy(t *testing.T) {
	dir := t.TempDir()

	// Simulate v0.11.x install at /opt/fusionaly/
	oldDir := filepath.Join(dir, "opt", "fusionaly")
	os.MkdirAll(filepath.Join(oldDir, "storage"), 0755)
	os.MkdirAll(filepath.Join(oldDir, "logs"), 0755)

	os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
		"FUSIONALY_DOMAIN=app.example.com\nAPP_IMAGE=ghcr.io/karloscodes/fusionaly:latest\nFUSIONALY_PRIVATE_KEY=abc123\nCUSTOM=value\nDATABASE_URL=sqlite:///app/storage/app.db\n",
	), 0600)
	os.WriteFile(filepath.Join(oldDir, "storage", "app.db"), []byte("sqlite-data"), 0644)
	os.WriteFile(filepath.Join(oldDir, "logs", "app.log"), []byte("log data"), 0644)

	configPath := filepath.Join(dir, "config.yml")
	dataBase := filepath.Join(dir, "var", "matcha")

	m := New(Config{
		Name:        "fusionaly",
		AppImage:    "ghcr.io/karloscodes/fusionaly:latest",
		Volumes:     []string{"/app/storage"},
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})

	err := m.migrateFromLegacy(oldDir)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// --- Config preserved in YAML ---

	app, err := LoadAppFrom(configPath, "fusionaly")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if app.Image != "ghcr.io/karloscodes/fusionaly:latest" {
		t.Errorf("image = %q, want ghcr.io/karloscodes/fusionaly:latest", app.Image)
	}
	if app.Domain != "app.example.com" {
		t.Errorf("domain = %q, want app.example.com", app.Domain)
	}
	if app.Env["PRIVATE_KEY"] != "abc123" {
		t.Errorf("PRIVATE_KEY = %q, want abc123", app.Env["PRIVATE_KEY"])
	}
	if app.Env["CUSTOM"] != "value" {
		t.Errorf("CUSTOM = %q, want value", app.Env["CUSTOM"])
	}
	if app.Env["DATABASE_URL"] != "sqlite:///app/storage/app.db" {
		t.Errorf("DATABASE_URL = %q, want sqlite:///app/storage/app.db", app.Env["DATABASE_URL"])
	}

	// --- DB file lands at the right place ---

	dbPath := filepath.Join(dataBase, "fusionaly", "storage", "app.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("DB not found at %s", dbPath)
	}

	// --- findDatabase() can locate it ---

	if found := m.findDatabase(); found != dbPath {
		t.Errorf("findDatabase() = %q, want %q", found, dbPath)
	}

	// --- Volume mount resolves so container sees DB at same path ---

	vols := m.resolveVolumes([]string{"/app/storage"})
	wantVol := filepath.Join(dataBase, "fusionaly", "storage") + ":/app/storage"
	if len(vols) != 1 || vols[0] != wantVol {
		t.Errorf("resolveVolumes = %v, want [%s]", vols, wantVol)
	}

	// --- Logs migrated ---

	logPath := filepath.Join(dataBase, "fusionaly", "logs", "app.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file not found at %s", logPath)
	}
}

// TestMigrateFromLegacyDetectsVolumes verifies that when Config.Volumes is empty
// (as happens during auto-migration), volumes are detected from existing directories.
func TestMigrateFromLegacyDetectsVolumes(t *testing.T) {
	dir := t.TempDir()

	oldDir := filepath.Join(dir, "opt", "fusionaly")
	os.MkdirAll(filepath.Join(oldDir, "storage"), 0755)
	os.MkdirAll(filepath.Join(oldDir, "logs"), 0755)

	os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
		"FUSIONALY_DOMAIN=app.example.com\nAPP_IMAGE=ghcr.io/karloscodes/fusionaly:latest\nPRIVATE_KEY=abc123\n",
	), 0600)
	os.WriteFile(filepath.Join(oldDir, "storage", "app.db"), []byte("sqlite-data"), 0644)
	os.WriteFile(filepath.Join(oldDir, "logs", "app.log"), []byte("log data"), 0644)

	configPath := filepath.Join(dir, "config.yml")
	dataBase := filepath.Join(dir, "var", "matcha")

	// No Volumes in Config — simulates matchaFromConfig("fusionaly") during auto-migration
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
		t.Fatalf("failed to load config: %v", err)
	}

	// Volumes should be detected from existing storage/ and logs/ dirs
	if len(app.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d: %v", len(app.Volumes), app.Volumes)
	}

	hasStorage := false
	hasLogs := false
	for _, v := range app.Volumes {
		if v == "/app/storage" {
			hasStorage = true
		}
		if v == "/app/logs" {
			hasLogs = true
		}
	}
	if !hasStorage {
		t.Errorf("expected /app/storage volume, got %v", app.Volumes)
	}
	if !hasLogs {
		t.Errorf("expected /app/logs volume, got %v", app.Volumes)
	}
}

func TestMigrateMovesBackups(t *testing.T) {
	dir := t.TempDir()

	// Create old layout with backups inside storage/
	oldDir := filepath.Join(dir, "opt", "myapp")
	os.MkdirAll(filepath.Join(oldDir, "storage", "backups"), 0755)
	os.MkdirAll(filepath.Join(oldDir, "logs"), 0755)

	os.WriteFile(filepath.Join(oldDir, ".env"), []byte(
		"MYAPP_DOMAIN=app.example.com\nAPP_IMAGE=test:latest\nMYAPP_PRIVATE_KEY=key123\n",
	), 0600)
	os.WriteFile(filepath.Join(oldDir, "storage", "myapp.db"), []byte("db"), 0644)
	os.WriteFile(filepath.Join(oldDir, "storage", "backups", "backup_20260220_100000.db"), []byte("backup"), 0644)

	configPath := filepath.Join(dir, "config.yml")
	dataBase := filepath.Join(dir, "var", "matcha")

	m := New(Config{
		Name:        "myapp",
		AppImage:    "test:latest",
		ConfigPath:  configPath,
		DataDirBase: dataBase,
	})

	err := m.migrateFromLegacy(oldDir)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// DB should be in storage/
	dbPath := filepath.Join(dataBase, "myapp", "storage", "myapp.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB not found at %s", dbPath)
	}

	// Backups should be moved from storage/backups/ to backups/
	backupPath := filepath.Join(dataBase, "myapp", "backups", "backup_20260220_100000.db")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup not found at %s", backupPath)
	}

	// Old storage/backups/ should be gone
	oldBackupDir := filepath.Join(dataBase, "myapp", "storage", "backups")
	if _, err := os.Stat(oldBackupDir); err == nil {
		t.Error("old storage/backups/ dir should have been removed")
	}

	// findDatabase should find the DB
	if found := m.findDatabase(); found == "" {
		t.Error("findDatabase() returned empty")
	}

	// listBackups should find the migrated backup
	backups, err := m.listBackups()
	if err != nil {
		t.Fatalf("listBackups() error: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
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
