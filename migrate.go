package matcha

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// detectOldInstall checks if there's an old-style install at /opt/{name}.
func (m *Matcha) detectOldInstall() bool {
	oldDir := "/opt/" + m.config.Name
	_, err := os.Stat(filepath.Join(oldDir, ".env"))
	return err == nil
}

// Migrate moves from old per-app layout (/opt/{name}/) to new shared layout (/etc/matcha/apps/{name}/).
func (m *Matcha) Migrate() error {
	oldDir := "/opt/" + m.config.Name
	newBase := "/etc/matcha"
	newAppDir := filepath.Join(newBase, "apps", m.config.Name)

	if _, err := os.Stat(filepath.Join(oldDir, ".env")); os.IsNotExist(err) {
		return fmt.Errorf("no old install found at %s", oldDir)
	}

	fmt.Printf("Migrating %s from %s to %s\n", m.config.Name, oldDir, newAppDir)

	return m.migrateToNewLayout(oldDir, newAppDir, newBase)
}

// migrateToNewLayout performs the actual migration from old to new directory layout.
func (m *Matcha) migrateToNewLayout(oldDir, newAppDir, newBase string) error {
	// Read old config
	origInstallDir := m.config.InstallDir
	m.config.InstallDir = oldDir
	data, err := m.readEnv()
	if err != nil {
		m.config.InstallDir = origInstallDir
		return fmt.Errorf("failed to read old config: %w", err)
	}
	m.config.InstallDir = origInstallDir

	// Register in new registry
	reg := &Registry{BaseDir: newBase}
	entry := AppEntry{
		Name:       m.config.Name,
		Image:      data.AppImage,
		Domain:     data.Domain,
		Port:       m.config.AppPort,
		HealthPath: m.config.HealthPath,
		Backups:    m.config.Backups,
		Volumes:    m.config.Volumes,
	}
	if err := reg.Save(entry); err != nil {
		return fmt.Errorf("failed to register app: %w", err)
	}

	// Copy .env to new location (preserves secrets)
	if err := copyFile(filepath.Join(oldDir, ".env"), filepath.Join(newAppDir, ".env")); err != nil {
		return fmt.Errorf("failed to copy .env: %w", err)
	}

	// Move storage and logs
	for _, sub := range []string{"storage", "logs"} {
		oldPath := filepath.Join(oldDir, sub)
		newPath := filepath.Join(newAppDir, sub)

		if _, err := os.Stat(oldPath); err != nil {
			continue
		}

		// Remove empty dir created by registry.Save
		os.RemoveAll(newPath)

		// Try rename (fast, same filesystem)
		if err := os.Rename(oldPath, newPath); err != nil {
			// Fall back to copy (cross-device)
			if err := copyDir(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to migrate %s: %w", sub, err)
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target)
	})
}
