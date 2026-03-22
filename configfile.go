package matcha

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// MatchaConfig is the top-level structure of /etc/matcha/config.yml.
type MatchaConfig struct {
	Apps map[string]AppConfig `yaml:"apps"`
}

// AppConfig holds the configuration for a single deployed application.
type AppConfig struct {
	Image      string            `yaml:"image"`
	Domain     string            `yaml:"domain"`
	Port       int               `yaml:"port,omitempty"`
	HealthPath string            `yaml:"health_path,omitempty"`
	Volumes    []string          `yaml:"volumes,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`
	HealthTimeout  int               `yaml:"health_timeout,omitempty"`
}

// ConfigPath returns the path to the matcha config file.
func ConfigPath() string {
	return "/etc/matcha/config.yml"
}

// DataDir returns the data directory for an app.
func DataDir(name string) string {
	return "/var/matcha/" + name
}

// LoadMatchaConfig reads the YAML config file.
func LoadMatchaConfig() (*MatchaConfig, error) {
	return LoadMatchaConfigFrom(ConfigPath())
}

// LoadMatchaConfigFrom reads a YAML config from a specific path.
func LoadMatchaConfigFrom(path string) (*MatchaConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MatchaConfig{Apps: make(map[string]AppConfig)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg MatchaConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Apps == nil {
		cfg.Apps = make(map[string]AppConfig)
	}
	return &cfg, nil
}

// SaveMatchaConfig writes the YAML config file.
func SaveMatchaConfig(cfg *MatchaConfig) error {
	return SaveMatchaConfigTo(cfg, ConfigPath())
}

// SaveMatchaConfigTo writes the YAML config to a specific path.
func SaveMatchaConfigTo(cfg *MatchaConfig, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// LoadApp loads a single app from the YAML config.
func LoadApp(name string) (AppConfig, error) {
	return LoadAppFrom(ConfigPath(), name)
}

// LoadAppFrom loads a single app from a specific config path.
func LoadAppFrom(path, name string) (AppConfig, error) {
	cfg, err := LoadMatchaConfigFrom(path)
	if err != nil {
		return AppConfig{}, err
	}

	app, ok := cfg.Apps[name]
	if !ok {
		return AppConfig{}, fmt.Errorf("app %q not found", name)
	}
	return app, nil
}

// SaveApp updates a single app in the YAML config (read-modify-write).
func SaveApp(name string, app AppConfig) error {
	return SaveAppTo(ConfigPath(), name, app)
}

// SaveAppTo updates a single app in a specific config file.
func SaveAppTo(path, name string, app AppConfig) error {
	cfg, err := LoadMatchaConfigFrom(path)
	if err != nil {
		return err
	}

	cfg.Apps[name] = app
	return SaveMatchaConfigTo(cfg, path)
}

// ListApps returns all apps from the YAML config, sorted by name.
func ListApps() (map[string]AppConfig, error) {
	return ListAppsFrom(ConfigPath())
}

// ListAppsFrom returns all apps from a specific config path.
func ListAppsFrom(path string) (map[string]AppConfig, error) {
	cfg, err := LoadMatchaConfigFrom(path)
	if err != nil {
		return nil, err
	}
	return cfg.Apps, nil
}

// ListAppsSorted returns app names sorted alphabetically.
func ListAppsSorted(apps map[string]AppConfig) []string {
	names := make([]string, 0, len(apps))
	for name := range apps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RemoveApp removes an app from the YAML config.
func RemoveApp(name string) error {
	return RemoveAppFrom(ConfigPath(), name)
}

// RemoveAppFrom removes an app from a specific config file.
func RemoveAppFrom(path, name string) error {
	cfg, err := LoadMatchaConfigFrom(path)
	if err != nil {
		return err
	}

	if _, ok := cfg.Apps[name]; !ok {
		return fmt.Errorf("app %q not found", name)
	}

	delete(cfg.Apps, name)
	return SaveMatchaConfigTo(cfg, path)
}

// ResolveVolumes converts container paths to docker -v args using default data dir.
// "/app/storage" → "/var/matcha/{name}/storage:/app/storage"
// "/data" → "/var/matcha/{name}/data:/data"
// "/var/lib/plausible" → "/var/matcha/{name}/plausible:/var/lib/plausible"
func ResolveVolumes(name string, volumes []string) []string {
	return resolveVolumesWithBase(DataDir(name), volumes)
}

func resolveVolumesWithBase(baseDir string, volumes []string) []string {
	var resolved []string
	for _, containerPath := range volumes {
		lastSegment := filepath.Base(containerPath)
		hostPath := filepath.Join(baseDir, lastSegment)
		resolved = append(resolved, hostPath+":"+containerPath)
	}
	return resolved
}
