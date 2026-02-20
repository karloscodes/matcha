package matcha

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// envData is kept minimal — only secrets that need to persist.
type envData struct {
	PrivateKey string
	// Domain, AppImage, ProxyImage now come from app.json via registry
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
	privateKey, err := GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Register app in registry (creates dirs, writes app.json)
	reg := &Registry{BaseDir: "/etc/matcha"}
	entry := AppEntry{
		Name:       m.config.Name,
		Image:      m.config.AppImage,
		Domain:     m.domain,
		Port:       m.config.AppPort,
		HealthPath: m.config.HealthPath,
		Backups:    m.config.Backups,
		Volumes:    m.config.Volumes,
	}
	if err := reg.Save(entry); err != nil {
		return fmt.Errorf("failed to register app: %w", err)
	}

	// Write .env with only the private key
	envPath := m.config.InstallDir + "/.env"
	content := fmt.Sprintf("PRIVATE_KEY=%s\n", privateKey)
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	return nil
}

// loadConfig loads existing configuration from registry or .env fallback.
func (m *Matcha) loadConfig() error {
	// Try registry first (new multi-app layout)
	reg := &Registry{BaseDir: "/etc/matcha"}
	if app, err := reg.Load(m.config.Name); err == nil {
		if app.Image != "" {
			m.config.AppImage = app.Image
		}
		if app.Domain != "" {
			m.domain = app.Domain
		}
		if app.Port != 0 {
			m.config.AppPort = app.Port
		}
		if app.HealthPath != "" {
			m.config.HealthPath = app.HealthPath
		}
		m.config.Backups = app.Backups
		m.config.Volumes = app.Volumes
		return nil
	}

	// Fall back to old .env format (pre-migration installs)
	return m.loadConfigFromEnv()
}

// loadConfigFromEnv reads the old-style .env for backward compat.
func (m *Matcha) loadConfigFromEnv() error {
	envPath := m.config.InstallDir + "/.env"
	vars, err := readEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	prefix := m.EnvPrefix()
	if v, ok := vars[prefix+"_DOMAIN"]; ok {
		m.domain = v
	}
	if v, ok := vars["APP_IMAGE"]; ok {
		m.config.AppImage = v
	}
	if v, ok := vars[prefix+"_APP_IMAGE"]; ok {
		m.config.AppImage = v
	}
	if v, ok := vars["PROXY_IMAGE"]; ok {
		m.config.ProxyImage = v
	}

	return nil
}

// readPrivateKey reads the private key from .env file.
func (m *Matcha) readPrivateKey() string {
	envPath := m.config.InstallDir + "/.env"
	vars, _ := readEnvFile(envPath)
	if v, ok := vars["PRIVATE_KEY"]; ok {
		return v
	}
	// Fall back to prefixed key (old format)
	prefix := m.EnvPrefix()
	if v, ok := vars[prefix+"_PRIVATE_KEY"]; ok {
		return v
	}
	return ""
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

// GeneratePrivateKey creates a secure random key.
func GeneratePrivateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
