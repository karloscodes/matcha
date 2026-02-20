package matcha

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupOldBackups(t *testing.T) {
	t.Run("keeps only last 3 backups", func(t *testing.T) {
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "storage", "backups")
		os.MkdirAll(backupDir, 0755)

		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		names := []string{
			"backup_20260215_100000.db",
			"backup_20260216_100000.db",
			"backup_20260217_100000.db",
			"backup_20260218_100000.db",
			"backup_20260219_100000.db",
		}
		for _, name := range names {
			os.WriteFile(filepath.Join(backupDir, name), []byte("data"), 0644)
		}

		m.cleanupOldBackups()

		entries, _ := os.ReadDir(backupDir)

		if len(entries) != 3 {
			t.Errorf("expected 3 backups, got %d", len(entries))
		}

		for _, entry := range entries {
			name := entry.Name()
			if name != "backup_20260217_100000.db" && name != "backup_20260218_100000.db" && name != "backup_20260219_100000.db" {
				t.Errorf("unexpected backup kept: %s", name)
			}
		}
	})

	t.Run("does nothing with fewer than 3 backups", func(t *testing.T) {
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "storage", "backups")
		os.MkdirAll(backupDir, 0755)

		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		os.WriteFile(filepath.Join(backupDir, "backup_20260219_100000.db"), []byte("data"), 0644)
		os.WriteFile(filepath.Join(backupDir, "backup_20260220_100000.db"), []byte("data"), 0644)

		m.cleanupOldBackups()

		entries, _ := os.ReadDir(backupDir)

		if len(entries) != 2 {
			t.Errorf("expected 2 backups, got %d", len(entries))
		}
	})

	t.Run("handles exactly 3 backups", func(t *testing.T) {
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "storage", "backups")
		os.MkdirAll(backupDir, 0755)

		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		names := []string{
			"backup_20260217_100000.db",
			"backup_20260218_100000.db",
			"backup_20260219_100000.db",
		}
		for _, name := range names {
			os.WriteFile(filepath.Join(backupDir, name), []byte("data"), 0644)
		}

		m.cleanupOldBackups()

		entries, _ := os.ReadDir(backupDir)

		if len(entries) != 3 {
			t.Errorf("expected 3 backups, got %d", len(entries))
		}
	})
}

func TestListBackups(t *testing.T) {
	t.Run("returns backups sorted newest first", func(t *testing.T) {
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "storage", "backups")
		os.MkdirAll(backupDir, 0755)

		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		names := []string{
			"backup_20260218_100000.db",
			"backup_20260220_100000.db",
			"backup_20260219_100000.db",
		}
		for _, name := range names {
			os.WriteFile(filepath.Join(backupDir, name), []byte("data"), 0644)
		}

		backups, err := m.listBackups()

		if err != nil {
			t.Fatalf("listBackups() error = %v", err)
		}
		if len(backups) != 3 {
			t.Fatalf("expected 3 backups, got %d", len(backups))
		}
		if backups[0].Name != "backup_20260220_100000.db" {
			t.Errorf("first backup should be newest, got %s", backups[0].Name)
		}
		if backups[2].Name != "backup_20260218_100000.db" {
			t.Errorf("last backup should be oldest, got %s", backups[2].Name)
		}
	})

	t.Run("ignores non-backup files", func(t *testing.T) {
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "storage", "backups")
		os.MkdirAll(backupDir, 0755)

		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		os.WriteFile(filepath.Join(backupDir, "backup_20260220_100000.db"), []byte("data"), 0644)
		os.WriteFile(filepath.Join(backupDir, "random_file.txt"), []byte("data"), 0644)
		os.WriteFile(filepath.Join(backupDir, "backup_badformat.db"), []byte("data"), 0644)

		backups, err := m.listBackups()

		if err != nil {
			t.Fatalf("listBackups() error = %v", err)
		}
		if len(backups) != 1 {
			t.Errorf("expected 1 backup, got %d", len(backups))
		}
	})
}

func TestCreateBackup(t *testing.T) {
	t.Run("fails when database does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := New(Config{Name: "testapp", AppImage: "test:latest", InstallDir: tmpDir})

		_, err := m.createBackup()

		if err == nil {
			t.Error("expected error for missing database")
		}
	})
}
