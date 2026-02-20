package matcha

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AppEntry holds the configuration for a single deployed application.
type AppEntry struct {
	Name       string   `json:"name"`
	Image      string   `json:"image"`
	Domain     string   `json:"domain"`
	Port       int      `json:"port"`
	HealthPath string   `json:"health_path,omitempty"`
	Volumes    []string `json:"volumes,omitempty"`
	Backups    bool     `json:"backups,omitempty"`
}

// Registry manages app configurations on disk.
type Registry struct {
	BaseDir string // e.g., /etc/matcha
}

// DefaultRegistry returns a Registry with the standard base directory.
func DefaultRegistry() *Registry {
	return &Registry{BaseDir: "/etc/matcha"}
}

// AppDir returns the directory path for a given app.
func (r *Registry) AppDir(name string) string {
	return filepath.Join(r.BaseDir, "apps", name)
}

// EnvPath returns the .env file path for a given app.
func (r *Registry) EnvPath(name string) string {
	return filepath.Join(r.AppDir(name), ".env")
}

// Save writes an app's configuration to disk. It creates the app directory,
// subdirectories for storage and logs, and an empty .env file if one doesn't exist.
func (r *Registry) Save(app AppEntry) error {
	dir := r.AppDir(app.Name)

	for _, sub := range []string{"", "storage/logs", "storage/backups"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling app config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "app.json"), data, 0644); err != nil {
		return fmt.Errorf("writing app.json: %w", err)
	}

	envPath := r.EnvPath(app.Name)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := os.WriteFile(envPath, nil, 0644); err != nil {
			return fmt.Errorf("creating .env: %w", err)
		}
	}

	return nil
}

// Load reads an app's configuration from disk.
func (r *Registry) Load(name string) (AppEntry, error) {
	path := filepath.Join(r.AppDir(name), "app.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return AppEntry{}, fmt.Errorf("reading app config for %q: %w", name, err)
	}

	var app AppEntry
	if err := json.Unmarshal(data, &app); err != nil {
		return AppEntry{}, fmt.Errorf("parsing app config for %q: %w", name, err)
	}

	return app, nil
}

// List returns all registered apps sorted by name.
func (r *Registry) List() ([]AppEntry, error) {
	appsDir := filepath.Join(r.BaseDir, "apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading apps directory: %w", err)
	}

	var apps []AppEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		app, err := r.Load(entry.Name())
		if err != nil {
			continue
		}
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Name < apps[j].Name
	})

	return apps, nil
}

// Remove deletes an app's directory and all its contents.
func (r *Registry) Remove(name string) error {
	dir := r.AppDir(name)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing app %q: %w", name, err)
	}
	return nil
}
