package matcha

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// envData holds the environment configuration.
type envData struct {
	Domain       string
	PrivateKey   string
	AppImage     string
	CaddyImage   string
	InstallDir   string
	BackupPath   string
	Version      string
	InstallerURL string
	User         string // Admin user email (optional)
	// DNS status (not saved to .env, used for display)
	dnsStatus *dnsStatus
}

// collectConfig prompts the user for configuration with DNS check and confirmation.
func (m *Matcha) collectConfig() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		// Prompt for domain
		fmt.Printf("%s (e.g., analytics.example.com): ", bold("Domain"))
		domain, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read domain: %w", err)
		}
		domain = strings.TrimSpace(domain)

		if domain == "" {
			fmt.Println("Error: Domain cannot be empty.")
			continue
		}

		if err := m.validateDomain(domain); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Check DNS - handle localhost specially
		fmt.Println()
		dnsReady := false
		if isLocalhost(domain) {
			fmt.Printf("%s %s (localhost, skipped)\n", bold("Checking DNS..."), domain)
			fmt.Println()
			dnsReady = true
			m.dnsStatus = &dnsStatus{Found: true, MatchIP: true}
		} else {
			fmt.Print(bold("Checking DNS... "))
			dns := m.checkDNS(domain)
			m.dnsStatus = dns

			if !dns.Found {
				fmt.Println("not found")
				m.printDNSInstructions(domain, dns.ServerIP)
			} else if !dns.MatchIP {
				fmt.Printf("%s (wrong server)\n", dns.DomainIP)
				fmt.Println()
				fmt.Printf("%s %s -> %s\n", dim("Update A record:"), domain, dns.ServerIP)
				fmt.Println(dim("SSL activates automatically once DNS propagates."))
				fmt.Println()
			} else {
				fmt.Printf("%s (this server)\n", green("✓ "+dns.DomainIP))
				fmt.Println()
				dnsReady = true
			}
		}

		// Show summary
		fmt.Println(bold("Summary"))
		fmt.Println()
		fmt.Printf("  Domain:  %s\n", domain)
		if dnsReady {
			fmt.Printf("  DNS:     %s\n", green("✓ Ready"))
		} else {
			fmt.Printf("  DNS:     %s\n", dim("Not ready (will continue anyway)"))
		}
		fmt.Println()

		// Confirm
		fmt.Printf("%s [Y/n] ", bold("Proceed?"))
		confirm, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm == "" || confirm == "y" || confirm == "yes" {
			m.domain = domain
			return nil
		}

		fmt.Println()
		fmt.Println("Cancelled. Starting over.")
		fmt.Println()
	}
}

// setupConfig creates directories and saves configuration after user confirms.
func (m *Matcha) setupConfig() error {
	// Generate private key
	privateKey, err := generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create install directory
	if err := os.MkdirAll(m.config.InstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install dir: failed to create directory: %w", err)
	}

	// Create subdirectories
	for _, dir := range []string{"storage", "logs", "caddy", "caddy/config", "storage/backups"} {
		if err := os.MkdirAll(m.config.InstallDir+"/"+dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Save config
	data := &envData{
		Domain:       m.domain,
		PrivateKey:   privateKey,
		AppImage:     m.config.AppImage,
		CaddyImage:   m.config.CaddyImage,
		InstallDir:   m.config.InstallDir,
		BackupPath:   m.config.InstallDir + "/storage/backups",
		Version:      "latest",
		InstallerURL: "https://github.com/" + m.config.ManagerRepo + "/releases/latest",
	}

	if err := m.saveEnv(data); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Generate Caddyfile
	if err := m.generateCaddyfile(data); err != nil {
		return fmt.Errorf("failed to generate Caddyfile: %w", err)
	}

	return nil
}

// loadConfig loads existing configuration from .env.
func (m *Matcha) loadConfig() error {
	data, err := m.readEnv()
	if err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}

	// Override config with values from .env
	if data.AppImage != "" {
		m.config.AppImage = data.AppImage
	}
	if data.CaddyImage != "" {
		m.config.CaddyImage = data.CaddyImage
	}
	// Store domain for GetDomain()
	m.domain = data.Domain

	return nil
}

// readEnv reads the .env file and returns envData.
func (m *Matcha) readEnv() (*envData, error) {
	envPath := m.config.InstallDir + "/.env"
	prefix := m.EnvPrefix()

	data := &envData{}

	file, err := os.Open(envPath)
	if err != nil {
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

		switch key {
		case prefix + "_DOMAIN":
			data.Domain = value
		case prefix + "_PRIVATE_KEY":
			data.PrivateKey = value
		case prefix + "_APP_IMAGE", "APP_IMAGE":
			data.AppImage = value
		case "CADDY_IMAGE":
			data.CaddyImage = value
		case "INSTALL_DIR":
			data.InstallDir = value
		case "BACKUP_PATH":
			data.BackupPath = value
		case "VERSION":
			data.Version = value
		case "INSTALLER_URL":
			data.InstallerURL = value
		case prefix + "_USER":
			data.User = value
		}
	}

	return data, scanner.Err()
}

// saveEnv writes the .env file in the same format as the original installer.
func (m *Matcha) saveEnv(data *envData) error {
	envPath := m.config.InstallDir + "/.env"
	prefix := m.EnvPrefix()

	// For fresh install, write all fields in order
	var lines []string
	lines = append(lines, fmt.Sprintf("%s_DOMAIN=%s", prefix, data.Domain))
	lines = append(lines, fmt.Sprintf("APP_IMAGE=%s", data.AppImage))
	lines = append(lines, fmt.Sprintf("CADDY_IMAGE=%s", data.CaddyImage))
	lines = append(lines, fmt.Sprintf("INSTALL_DIR=%s", data.InstallDir))
	lines = append(lines, fmt.Sprintf("BACKUP_PATH=%s", data.BackupPath))
	lines = append(lines, fmt.Sprintf("VERSION=%s", data.Version))
	lines = append(lines, fmt.Sprintf("INSTALLER_URL=%s", data.InstallerURL))
	lines = append(lines, fmt.Sprintf("%s_PRIVATE_KEY=%s", prefix, data.PrivateKey))
	if data.User != "" {
		lines = append(lines, fmt.Sprintf("%s_USER=%s", prefix, data.User))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(envPath, []byte(content), 0600)
}

// validateDomain performs basic domain validation.
func (m *Matcha) validateDomain(domain string) error {
	if strings.Contains(domain, " ") {
		return fmt.Errorf("validation failed for field 'domain' with value '%s': invalid domain format", domain)
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return fmt.Errorf("validation failed for field 'domain' with value '%s': invalid domain format", domain)
	}
	if !strings.Contains(domain, ".") && domain != "localhost" {
		return fmt.Errorf("invalid domain format")
	}
	return nil
}

// generatePrivateKey creates a secure random key.
func generatePrivateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
