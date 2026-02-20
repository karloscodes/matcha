package matcha

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// Read old .env file
	vars, err := readEnvFile(filepath.Join(oldDir, ".env"))
	if err != nil {
		return fmt.Errorf("failed to read old config: %w", err)
	}

	prefix := m.EnvPrefix()
	domain := vars[prefix+"_DOMAIN"]
	appImage := vars["APP_IMAGE"]
	if v, ok := vars[prefix+"_APP_IMAGE"]; ok {
		appImage = v
	}
	if appImage == "" {
		appImage = m.config.AppImage
	}

	// Get private key
	privateKey := vars["PRIVATE_KEY"]
	if privateKey == "" {
		privateKey = vars[prefix+"_PRIVATE_KEY"]
	}

	// Register in new registry
	reg := &Registry{BaseDir: newBase}
	entry := AppEntry{
		Name:       m.config.Name,
		Image:      appImage,
		Domain:     domain,
		Port:       m.config.AppPort,
		HealthPath: m.config.HealthPath,
		Backups:    m.config.Backups,
		Volumes:    m.config.Volumes,
	}
	if err := reg.Save(entry); err != nil {
		return fmt.Errorf("failed to register app: %w", err)
	}

	// Write new .env with just the private key (plus any user vars)
	var envLines []string
	envLines = append(envLines, "PRIVATE_KEY="+privateKey)
	// Carry over non-managed vars
	managedKeys := map[string]bool{
		prefix + "_DOMAIN":      true,
		prefix + "_PRIVATE_KEY": true,
		"APP_IMAGE":             true,
		"PROXY_IMAGE":           true,
		"PRIVATE_KEY":           true,
		"INSTALL_DIR":           true,
		"BACKUP_PATH":           true,
		"VERSION":               true,
		"INSTALLER_URL":         true,
	}
	managedKeys[prefix+"_APP_IMAGE"] = true
	for k, v := range vars {
		if managedKeys[k] {
			continue
		}
		envLines = append(envLines, k+"="+v)
	}
	envContent := strings.Join(envLines, "\n") + "\n"
	os.WriteFile(filepath.Join(newAppDir, ".env"), []byte(envContent), 0600)

	// Move storage and logs
	for _, sub := range []string{"storage", "logs"} {
		oldPath := filepath.Join(oldDir, sub)
		newPath := filepath.Join(newAppDir, sub)
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}
		os.RemoveAll(newPath)
		if err := os.Rename(oldPath, newPath); err != nil {
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
