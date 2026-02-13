package matcha

import (
	"fmt"
	"os"
	"os/exec"
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
	m.printWelcome()

	// Check root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation requires root privileges")
	}

	// Check ports
	sp := m.StartSpinner("Checking ports")
	if err := m.checkPorts(); err != nil {
		sp.Stop(false)
		return err
	}
	sp.Stop(true)

	// Install SQLite
	sp = m.StartSpinner("SQLite")
	if err := m.ensureSQLite(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("SQLite setup failed: %w", err)
	}
	sp.Stop(true)

	// Install Docker
	sp = m.StartSpinner("Docker")
	if err := m.ensureDocker(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("docker setup failed: %w", err)
	}
	sp.Stop(true)

	// Collect config from user
	if err := m.collectConfig(); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}

	// Deploy
	sp = m.StartSpinner("Deploying")
	if err := m.deploy(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("deployment failed: %w", err)
	}
	sp.Stop(true)

	// Setup cron if enabled
	if m.config.CronUpdates {
		sp = m.StartSpinner("Cron")
		if err := m.setupCron(); err != nil {
			sp.Stop(false)
			printWarn("cron setup failed: %v", err)
		} else {
			sp.Stop(true)
		}
	}

	// Install binary
	if err := m.installBinary(); err != nil {
		printWarn("binary install failed: %v", err)
	}

	// Read domain for completion message
	if data, err := m.readEnv(); err == nil {
		m.printComplete(data.Domain, false, "")
	} else {
		printSuccess("Installation complete")
	}

	return nil
}

// Update pulls the latest image and performs a deployment.
func (m *Matcha) Update() error {
	printHeader("Updating " + m.config.Name)

	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sp := m.StartSpinner("Pulling images")
	if err := m.pullImages(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("failed to pull images: %w", err)
	}
	sp.Stop(true)

	sp = m.StartSpinner("Deploying")
	if err := m.deploy(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("deployment failed: %w", err)
	}
	sp.Stop(true)

	if err := m.upgradeBinary(); err != nil {
		printWarn("binary upgrade failed: %v", err)
	}

	if err := m.pruneImages(); err != nil {
		printWarn("image prune failed: %v", err)
	}

	printSuccess("Update complete")
	return nil
}

// Reload restarts containers with current config (no image pull).
func (m *Matcha) Reload() error {
	printHeader("Reloading " + m.config.Name)

	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sp := m.StartSpinner("Deploying")
	if err := m.deploy(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("deployment failed: %w", err)
	}
	sp.Stop(true)

	printSuccess("Reload complete")
	return nil
}

// RestoreDB lists backups and restores the selected one.
func (m *Matcha) RestoreDB() error {
	if !m.config.Backups {
		return fmt.Errorf("backups not enabled for this application")
	}

	printHeader("Restoring database for " + m.config.Name)

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

	sp := m.StartSpinner("Restoring")
	if err := m.restoreBackup(selected); err != nil {
		sp.Stop(false)
		return fmt.Errorf("restore failed: %w", err)
	}
	sp.Stop(true)

	printSuccess("Database restored")
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

// SetImage changes the app image for subsequent deployments.
func (m *Matcha) SetImage(image string) {
	m.config.AppImage = image
}

// GetDomain reads the domain from the .env file.
func (m *Matcha) GetDomain() (string, error) {
	data, err := m.readEnv()
	if err != nil {
		return "", err
	}
	return data.Domain, nil
}

// Exec runs a command inside the app container.
func (m *Matcha) Exec(args ...string) error {
	// Find the running app container
	containerName := m.AppContainerName(0)
	if m.config.BlueGreen {
		// Find which slot is active
		if m.isRunning(m.AppContainerName(1)) {
			containerName = m.AppContainerName(1)
		} else if m.isRunning(m.AppContainerName(2)) {
			containerName = m.AppContainerName(2)
		}
	}

	execArgs := append([]string{"exec", containerName}, args...)
	_, err := m.runDocker(execArgs...)
	return err
}

// BackupDB creates a backup of the database and returns the backup path.
func (m *Matcha) BackupDB() (string, error) {
	backupDir := m.config.InstallDir + "/storage/backups"
	dbPath := m.config.InstallDir + "/storage/" + m.config.Name + "-production.db"

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Generate backup filename with timestamp
	backupPath := fmt.Sprintf("%s/backup_upgrade_%d.db", backupDir, os.Getpid())

	// Use sqlite3 .backup command (database is on host filesystem)
	cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".backup '%s'", backupPath))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}

	return backupPath, nil
}

// Deploy triggers a deployment with current configuration.
func (m *Matcha) Deploy() error {
	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	return m.deploy()
}
