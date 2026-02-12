package matcha

import (
	"fmt"
	"os"
	"strings"
)

// Config defines how Matcha deploys your application.
type Config struct {
	// Required
	Name     string // "fusionaly" → env prefix FUSIONALY_, container names, etc.
	AppImage string // "karloscodes/fusionaly:latest"

	// Optional with defaults
	InstallDir string // default: /opt/{Name}
	BinaryPath string // default: /usr/local/bin/{Name}
	CaddyImage string // default: caddy:2-alpine
	HealthPath string // default: /_health
	AppPort    int    // default: 8080

	// Feature flags
	BlueGreen   bool // dual containers, zero-downtime switch
	CronUpdates bool // daily 3 AM auto-update cron job
	Backups     bool // SQLite backup with retention policy
}

// Matcha is the main orchestrator for deployments.
type Matcha struct {
	config Config
	logger *Logger
}

// New creates a new Matcha instance with the given configuration.
func New(cfg Config) *Matcha {
	// Apply defaults
	if cfg.InstallDir == "" {
		cfg.InstallDir = "/opt/" + cfg.Name
	}
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = "/usr/local/bin/" + cfg.Name
	}
	if cfg.CaddyImage == "" {
		cfg.CaddyImage = "caddy:2-alpine"
	}
	if cfg.HealthPath == "" {
		cfg.HealthPath = "/_health"
	}
	if cfg.AppPort == 0 {
		cfg.AppPort = 8080
	}

	return &Matcha{
		config: cfg,
		logger: NewLogger(),
	}
}

// EnvPrefix returns the uppercase name used for environment variables.
func (m *Matcha) EnvPrefix() string {
	return strings.ToUpper(m.config.Name)
}

// NetworkName returns the Docker network name.
func (m *Matcha) NetworkName() string {
	return m.config.Name + "-network"
}

// CaddyContainerName returns the Caddy container name.
func (m *Matcha) CaddyContainerName() string {
	return m.config.Name + "-caddy"
}

// AppContainerName returns the app container name(s).
func (m *Matcha) AppContainerName(slot int) string {
	if m.config.BlueGreen {
		return fmt.Sprintf("%s-app-%d", m.config.Name, slot)
	}
	return m.config.Name + "-app"
}

// Install runs the full installation process.
func (m *Matcha) Install() error {
	m.logger.Info("Installing %s", m.config.Name)

	// Check root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation requires root privileges")
	}

	// Check ports
	if err := m.checkPorts(); err != nil {
		return err
	}

	// Install Docker
	if err := m.ensureDocker(); err != nil {
		return fmt.Errorf("docker setup failed: %w", err)
	}

	// Collect config from user
	if err := m.collectConfig(); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}

	// Deploy
	if err := m.deploy(); err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Setup cron if enabled
	if m.config.CronUpdates {
		if err := m.setupCron(); err != nil {
			m.logger.Warn("cron setup failed: %v", err)
		}
	}

	// Install binary
	if err := m.installBinary(); err != nil {
		m.logger.Warn("binary install failed: %v", err)
	}

	m.logger.Success("Installation complete")
	return nil
}

// Update pulls the latest image and performs a deployment.
func (m *Matcha) Update() error {
	m.logger.Info("Updating %s", m.config.Name)

	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := m.pullImages(); err != nil {
		return fmt.Errorf("failed to pull images: %w", err)
	}

	if err := m.deploy(); err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	if err := m.upgradeBinary(); err != nil {
		m.logger.Warn("binary upgrade failed: %v", err)
	}

	if err := m.pruneImages(); err != nil {
		m.logger.Warn("image prune failed: %v", err)
	}

	m.logger.Success("Update complete")
	return nil
}

// Reload restarts containers with current config (no image pull).
func (m *Matcha) Reload() error {
	m.logger.Info("Reloading %s", m.config.Name)

	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := m.deploy(); err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	m.logger.Success("Reload complete")
	return nil
}

// RestoreDB lists backups and restores the selected one.
func (m *Matcha) RestoreDB() error {
	if !m.config.Backups {
		return fmt.Errorf("backups not enabled for this application")
	}

	m.logger.Info("Restoring database for %s", m.config.Name)

	backups, err := m.listBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		return fmt.Errorf("no backups found")
	}

	selected, err := m.promptBackupSelection(backups)
	if err != nil {
		return fmt.Errorf("backup selection failed: %w", err)
	}

	if err := m.restoreBackup(selected); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	m.logger.Success("Database restored")
	return nil
}

// Status shows the current state of the deployment.
func (m *Matcha) Status() error {
	return m.showStatus()
}

// GetConfig returns the current configuration.
func (m *Matcha) GetConfig() Config {
	return m.config
}
