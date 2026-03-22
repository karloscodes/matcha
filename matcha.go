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
	BinaryPath string // default: /usr/local/bin/{Name}
	ProxyImage string // default: basecamp/kamal-proxy:latest
	HealthPath string // default: /up
	AppPort    int    // default: 8080

	// Feature flags
	CronUpdates bool // daily 3 AM auto-update cron job
	Backups     bool // SQLite backup with retention policy

	// Custom configuration
	Volumes        []string // Container paths to mount (e.g., /app/storage)
	HealthTimeout  int      // Health check timeout in seconds (default: 30)

	// Self-update configuration (see selfupdate.go for conventions)
	ManagerRepo    string // GitHub repo for releases, e.g., "karloscodes/fusionaly"
	ManagerVersion string // current version, e.g., "v1.4.37" (set via ldflags at build time)

	// Internal: override paths (for testing)
	ConfigPath  string
	DataDirBase string
}

// Matcha is the main orchestrator for deployments.
type Matcha struct {
	config Config
	// Temporary state during installation
	domain    string
	dnsStatus *dnsStatus
}

// New creates a new Matcha instance with the given configuration.
func New(cfg Config) *Matcha {
	// Apply defaults
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = "/usr/local/bin/" + cfg.Name
	}
	if cfg.ProxyImage == "" {
		cfg.ProxyImage = "basecamp/kamal-proxy:latest"
	}
	if cfg.HealthPath == "" {
		cfg.HealthPath = "/up"
	}
	if cfg.AppPort == 0 {
		cfg.AppPort = 8080
	}
	if cfg.HealthTimeout == 0 {
		cfg.HealthTimeout = 30
	}

	return &Matcha{
		config: cfg,
	}
}

// NewFromApp creates a Matcha instance from an AppConfig and a base Config.
// The base config provides paths and other shared settings; app-specific
// fields (image, port, health, volumes) come from the AppConfig.
func NewFromApp(name string, app AppConfig, baseCfg Config) *Matcha {
	baseCfg.Name = name
	baseCfg.AppImage = app.Image
	baseCfg.AppPort = app.Port
	baseCfg.HealthPath = app.HealthPath
	baseCfg.HealthTimeout = app.HealthTimeout
	baseCfg.Volumes = app.Volumes
	return New(baseCfg)
}

// configPath returns the config file path, using override if set.
func (m *Matcha) configPath() string {
	if m.config.ConfigPath != "" {
		return m.config.ConfigPath
	}
	return ConfigPath()
}

// DataDir returns the data directory for this app.
func (m *Matcha) DataDir() string {
	if m.config.DataDirBase != "" {
		return m.config.DataDirBase + "/" + m.config.Name
	}
	return DataDir(m.config.Name)
}

// resolveVolumes converts container paths to docker -v args using this instance's data dir.
func (m *Matcha) resolveVolumes(volumes []string) []string {
	return resolveVolumesWithBase(m.DataDir(), volumes)
}

// EnvPrefix returns the uppercase name used for environment variables.
func (m *Matcha) EnvPrefix() string {
	return strings.ToUpper(m.config.Name)
}

// NetworkName returns the Docker network name.
func (m *Matcha) NetworkName() string {
	return "matcha-network"
}

// ProxyContainerName returns the proxy container name.
func (m *Matcha) ProxyContainerName() string {
	return "matcha-proxy"
}

// AppContainerName returns the app container name.
func (m *Matcha) AppContainerName() string {
	return m.config.Name
}

// Setup installs shared infrastructure: Docker, network, and proxy.
func Setup() error {
	m := &Matcha{
		config: Config{
			ProxyImage: "basecamp/kamal-proxy:latest",
		},
	}

	fmt.Println()
	fmt.Println(bold("Setting up Matcha"))
	fmt.Println()

	sp := m.StartSpinner("Docker")
	if err := m.ensureDocker(); err != nil {
		sp.Stop(false)
		return err
	}
	sp.Stop(true)

	sp = m.StartSpinner("Network")
	if err := m.createNetwork(); err != nil {
		sp.Stop(false)
		return err
	}
	sp.Stop(true)

	sp = m.StartSpinner("Proxy")
	if err := m.deployProxy(); err != nil {
		sp.Stop(false)
		return err
	}
	sp.Stop(true)

	fmt.Println()
	return nil
}

// Logs streams logs from the app container.
func (m *Matcha) Logs() error {
	cmd := exec.Command("docker", "logs", "--tail", "100", "-f", m.findActiveContainer())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install runs the full installation process.
func (m *Matcha) Install() error {
	m.printWelcome()

	// Check root (skip in test environment)
	if os.Geteuid() != 0 && os.Getenv("ENV") != "test" {
		return fmt.Errorf("installation requires root privileges")
	}

	// Collect config from user FIRST (before installing anything)
	if err := m.collectConfig(); err != nil {
		return fmt.Errorf("failed to collect configuration: %w", err)
	}

	// Now start the installation with spinners
	fmt.Println()
	fmt.Println(bold("Installing"))
	fmt.Println()

	// Check system requirements (ports)
	sp := m.StartSpinner("Checking system")
	if err := m.checkPorts(); err != nil {
		sp.Stop(false)
		return err
	}
	sp.Stop(true)

	// Install Docker
	sp = m.StartSpinner("Docker")
	if err := m.ensureDocker(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("docker setup failed: %w", err)
	}
	sp.Stop(true)

	// Configure (save to YAML config)
	sp = m.StartSpinner("Configuring")
	if err := m.setupConfig(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("failed to configure system: %w", err)
	}
	sp.Stop(true)

	// Deploy
	sp = m.StartSpinner("Deploying")
	if err := m.deploy(); err != nil {
		sp.Stop(false)
		return fmt.Errorf("deployment failed: %w", err)
	}
	sp.Stop(true)

	// Maintenance (cron + binary install)
	sp = m.StartSpinner("Maintenance")
	var maintenanceErr error
	if m.config.CronUpdates {
		if err := m.setupCron(); err != nil {
			maintenanceErr = err
		}
	}
	if err := m.installBinary(); err != nil && maintenanceErr == nil {
		maintenanceErr = err
	}
	if maintenanceErr != nil {
		sp.Stop(false)
		printWarn("maintenance setup: %v", maintenanceErr)
	} else {
		sp.Stop(true)
	}

	// Show completion message with DNS warning if needed
	dnsWarning := m.dnsStatus != nil && (!m.dnsStatus.Found || !m.dnsStatus.MatchIP)
	serverIP := ""
	if m.dnsStatus != nil {
		serverIP = m.dnsStatus.ServerIP
	}
	m.printComplete(m.domain, dnsWarning, serverIP)

	return nil
}

// Update pulls the latest images and deploys all apps from the YAML config.
// Self-update runs once. Each app gets its own image pull and deploy.
// Individual app failures are warned but don't stop other apps.
func (m *Matcha) Update() error {
	// Self-update manager binary if configured (once)
	if m.config.ManagerRepo != "" {
		printHeader("Updating " + m.config.Name)
		sp := m.StartSpinner("Checking for updates")
		updated, err := m.SelfUpdate()
		if err != nil {
			sp.Stop(false)
			printWarn("self-update check failed: %v", err)
		} else if updated {
			sp.Stop(true)
		} else {
			sp.Stop(true)
		}
	}

	// Load all apps from YAML config
	apps, err := ListAppsFrom(m.configPath())
	if err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	// If no apps in config, fall back to updating just the primary app
	if len(apps) == 0 {
		return m.updateSingle()
	}

	for _, name := range ListAppsSorted(apps) {
		app := apps[name]
		am := NewFromApp(name, app, Config{
			ConfigPath:     m.config.ConfigPath,
			DataDirBase:    m.config.DataDirBase,
			ManagerVersion: m.config.ManagerVersion,
		})
		if err := am.updateSingle(); err != nil {
			printWarn("failed to update %s: %v", name, err)
		}
	}

	return nil
}

// updateSingle pulls images and deploys a single app.
func (m *Matcha) updateSingle() error {
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

// SaveImage persists the current app image to the YAML config.
func (m *Matcha) SaveImage() error {
	app, err := LoadAppFrom(m.configPath(), m.config.Name)
	if err != nil {
		return err
	}
	app.Image = m.config.AppImage
	return SaveAppTo(m.configPath(), m.config.Name, app)
}

// GetDomain reads the domain from config or YAML.
func (m *Matcha) GetDomain() (string, error) {
	if m.domain != "" {
		return m.domain, nil
	}
	app, err := LoadAppFrom(m.configPath(), m.config.Name)
	if err != nil {
		return "", err
	}
	return app.Domain, nil
}

// BackupDB creates a backup of the database and returns the backup path.
func (m *Matcha) BackupDB() (string, error) {
	return m.createBackup()
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

// Exec runs a command inside the app container.
func (m *Matcha) Exec(args ...string) error {
	execArgs := append([]string{"exec", m.findActiveContainer()}, args...)
	_, err := m.runDocker(execArgs...)
	return err
}

// Deploy triggers a deployment with current configuration.
func (m *Matcha) Deploy() error {
	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	return m.deploy()
}
