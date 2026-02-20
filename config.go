package matcha

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

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

// setupConfig saves configuration to YAML config after user confirms.
func (m *Matcha) setupConfig() error {
	privateKey, err := GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	app := AppConfig{
		Image:      m.config.AppImage,
		Domain:     m.domain,
		Port:       m.config.AppPort,
		HealthPath: m.config.HealthPath,
		Volumes:    m.config.Volumes,
		Env: map[string]string{
			"PRIVATE_KEY": privateKey,
		},
	}

	if err := SaveAppTo(m.configPath(), m.config.Name, app); err != nil {
		return fmt.Errorf("failed to save app config: %w", err)
	}

	// Create data directories for volumes
	for _, v := range m.resolveVolumes(m.config.Volumes) {
		hostPath := strings.SplitN(v, ":", 2)[0]
		if err := os.MkdirAll(hostPath, 0755); err != nil {
			return fmt.Errorf("failed to create volume dir %s: %w", hostPath, err)
		}
	}

	return nil
}

// loadConfig loads existing configuration from YAML config.
// If the app isn't in YAML yet but an old layout exists, auto-migrates first.
func (m *Matcha) loadConfig() error {
	app, err := LoadAppFrom(m.configPath(), m.config.Name)
	if err != nil {
		// App not in YAML — try auto-migration from old layouts
		if migrated := m.tryAutoMigrate(); migrated {
			// Retry loading from YAML after migration
			app, err = LoadAppFrom(m.configPath(), m.config.Name)
			if err != nil {
				return fmt.Errorf("failed to load config after migration: %w", err)
			}
		} else {
			return fmt.Errorf("app %q not found in config", m.config.Name)
		}
	}

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
	m.config.Volumes = app.Volumes
	return nil
}

// tryAutoMigrate attempts to migrate from old layouts to YAML config.
// Returns true if migration was performed.
func (m *Matcha) tryAutoMigrate() bool {
	// Try multi-app layout: /etc/matcha/apps/{name}/app.json
	multiAppDir := "/etc/matcha/apps/" + m.config.Name
	if _, err := os.Stat(multiAppDir + "/app.json"); err == nil {
		fmt.Printf("Auto-migrating %s from %s to YAML config\n", m.config.Name, multiAppDir)
		if err := m.migrateFromMultiApp(multiAppDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: auto-migration failed: %v\n", err)
			return false
		}
		return true
	}

	// Try legacy layout: /opt/{name}/.env
	oldDir := "/opt/" + m.config.Name
	if _, err := os.Stat(oldDir + "/.env"); err == nil {
		fmt.Printf("Auto-migrating %s from %s to YAML config\n", m.config.Name, oldDir)
		if err := m.migrateFromLegacy(oldDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: auto-migration failed: %v\n", err)
			return false
		}
		return true
	}

	return false
}

// readPrivateKey reads the private key from YAML config.
func (m *Matcha) readPrivateKey() string {
	app, err := LoadAppFrom(m.configPath(), m.config.Name)
	if err != nil {
		return ""
	}
	if v, ok := app.Env["PRIVATE_KEY"]; ok {
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
