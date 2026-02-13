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
	Domain     string
	PrivateKey string
}

// collectConfig prompts the user for configuration.
func (m *Matcha) collectConfig() error {
	reader := bufio.NewReader(os.Stdin)

	// Prompt for domain
	fmt.Println()
	fmt.Print("  Domain (e.g., app.example.com): ")
	domain, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read domain: %w", err)
	}
	domain = strings.TrimSpace(domain)

	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	if err := m.validateDomain(domain); err != nil {
		return err
	}

	// Generate private key
	privateKey, err := generatePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create install directory
	if err := os.MkdirAll(m.config.InstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}

	// Create subdirectories
	for _, dir := range []string{"storage", "logs", "caddy", "caddy/config", "storage/backups"} {
		if err := os.MkdirAll(m.config.InstallDir+"/"+dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Save config
	data := &envData{
		Domain:     domain,
		PrivateKey: privateKey,
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
	envPath := m.config.InstallDir + "/.env"

	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}

	// We just need to verify it exists and is readable
	// The actual values are loaded when needed
	if len(data) == 0 {
		return fmt.Errorf(".env is empty")
	}

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
		}
	}

	return data, scanner.Err()
}

// saveEnv writes the .env file, preserving unknown lines.
func (m *Matcha) saveEnv(data *envData) error {
	envPath := m.config.InstallDir + "/.env"
	prefix := m.EnvPrefix()

	var lines []string
	foundDomain, foundKey := false, false

	// Read existing file if it exists
	if existing, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, prefix+"_DOMAIN=") {
				lines = append(lines, fmt.Sprintf("%s_DOMAIN=%s", prefix, data.Domain))
				foundDomain = true
			} else if strings.HasPrefix(trimmed, prefix+"_PRIVATE_KEY=") {
				// Preserve existing private key
				lines = append(lines, line)
				foundKey = true
			} else if trimmed != "" {
				// Preserve all other lines
				lines = append(lines, line)
			}
		}
	}

	// Add missing lines
	if !foundDomain {
		lines = append(lines, fmt.Sprintf("%s_DOMAIN=%s", prefix, data.Domain))
	}
	if !foundKey {
		lines = append(lines, fmt.Sprintf("%s_PRIVATE_KEY=%s", prefix, data.PrivateKey))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(envPath, []byte(content), 0600)
}

// validateDomain performs basic domain validation.
func (m *Matcha) validateDomain(domain string) error {
	if strings.Contains(domain, " ") {
		return fmt.Errorf("domain cannot contain spaces")
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return fmt.Errorf("domain should not include protocol")
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
