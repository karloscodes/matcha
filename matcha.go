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

	// Custom configuration
	Volumes []string // Container paths to mount (e.g., /app/storage)

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

	return &Matcha{
		config: cfg,
	}
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
	cmd := exec.Command("docker", "logs", "--tail", "100", "-f", m.AppContainerName())
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

// Update pulls the latest image and performs a deployment.
func (m *Matcha) Update() error {
	printHeader("Updating " + m.config.Name)

	// Self-update manager binary if configured
	if m.config.ManagerRepo != "" {
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

// Exec runs a command inside the app container.
func (m *Matcha) Exec(args ...string) error {
	execArgs := append([]string{"exec", m.AppContainerName()}, args...)
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
