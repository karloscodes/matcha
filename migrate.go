package matcha

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// readEnvFile reads a .env file into a map (kept for migration).
func readEnvFile(path string) (map[string]string, error) {
	vars := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return vars, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		vars[key] = value
	}

	return vars, scanner.Err()
}

// oldAppJSON is the structure of the old app.json files.
type oldAppJSON struct {
	Name       string   `json:"name"`
	Image      string   `json:"image"`
	Domain     string   `json:"domain"`
	Port       int      `json:"port"`
	HealthPath string   `json:"health_path"`
	Volumes    []string `json:"volumes"`
}

// Migrate moves from old layouts to new YAML config + /var/matcha/{name}/ data dir.
// Supports migration from:
//   - /opt/{name}/.env (legacy layout)
//   - /etc/matcha/apps/{name}/app.json + .env (previous multi-app layout)
func (m *Matcha) Migrate() error {
	// Try current multi-app layout first
	multiAppDir := "/etc/matcha/apps/" + m.config.Name
	if _, err := os.Stat(filepath.Join(multiAppDir, "app.json")); err == nil {
		fmt.Printf("Migrating %s from %s to YAML config\n", m.config.Name, multiAppDir)
		return m.migrateFromMultiApp(multiAppDir)
	}

	// Try legacy /opt/{name} layout
	oldDir := "/opt/" + m.config.Name
	if _, err := os.Stat(filepath.Join(oldDir, ".env")); err == nil {
		fmt.Printf("Migrating %s from %s to YAML config\n", m.config.Name, oldDir)
		return m.migrateFromLegacy(oldDir)
	}

	return fmt.Errorf("no old install found for %s", m.config.Name)
}

// migrateFromMultiApp migrates from /etc/matcha/apps/{name}/ layout.
func (m *Matcha) migrateFromMultiApp(appDir string) error {
	data, err := os.ReadFile(filepath.Join(appDir, "app.json"))
	if err != nil {
		return fmt.Errorf("reading app.json: %w", err)
	}

	var oldApp oldAppJSON
	if err := json.Unmarshal(data, &oldApp); err != nil {
		return fmt.Errorf("parsing app.json: %w", err)
	}

	// Read .env
	vars, _ := readEnvFile(filepath.Join(appDir, ".env"))

	// Build env map
	env := m.buildMigrationEnv(vars)

	// Save to YAML config
	app := AppConfig{
		Image:      oldApp.Image,
		Domain:     oldApp.Domain,
		Port:       oldApp.Port,
		HealthPath: oldApp.HealthPath,
		Volumes:    oldApp.Volumes,
		Env:        env,
	}

	if err := SaveAppTo(m.configPath(), m.config.Name, app); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Move data to /var/matcha/{name}/
	return m.migrateDataDirs(appDir)
}

// migrateFromLegacy migrates from /opt/{name}/ layout.
func (m *Matcha) migrateFromLegacy(oldDir string) error {
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

	env := m.buildMigrationEnv(vars)

	app := AppConfig{
		Image:      appImage,
		Domain:     domain,
		Port:       m.config.AppPort,
		HealthPath: m.config.HealthPath,
		Volumes:    m.config.Volumes,
		Env:        env,
	}

	if err := SaveAppTo(m.configPath(), m.config.Name, app); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return m.migrateDataDirs(oldDir)
}

// buildMigrationEnv extracts env vars from old .env file, keeping private key and user vars.
func (m *Matcha) buildMigrationEnv(vars map[string]string) map[string]string {
	prefix := m.EnvPrefix()
	managedKeys := map[string]bool{
		prefix + "_DOMAIN":      true,
		prefix + "_PRIVATE_KEY": true,
		prefix + "_APP_IMAGE":   true,
		"APP_IMAGE":             true,
		"PROXY_IMAGE":           true,
		"INSTALL_DIR":           true,
		"BACKUP_PATH":           true,
		"VERSION":               true,
		"INSTALLER_URL":         true,
	}

	env := make(map[string]string)

	// Keep private key
	if v, ok := vars["PRIVATE_KEY"]; ok {
		env["PRIVATE_KEY"] = v
	} else if v, ok := vars[prefix+"_PRIVATE_KEY"]; ok {
		env["PRIVATE_KEY"] = v
	}

	// Carry over non-managed env vars
	for k, v := range vars {
		if managedKeys[k] || k == "PRIVATE_KEY" {
			continue
		}
		env[k] = v
	}

	return env
}

// migrateDataDirs moves storage and logs from old dir to /var/matcha/{name}/.
func (m *Matcha) migrateDataDirs(oldDir string) error {
	dataDir := m.DataDir()
	for _, sub := range []string{"storage", "logs"} {
		oldPath := filepath.Join(oldDir, sub)
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}
		newPath := filepath.Join(dataDir, sub)
		os.MkdirAll(filepath.Dir(newPath), 0755)
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
