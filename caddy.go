package matcha

import (
	"fmt"
	"os"
	"strings"
)

// caddyfileTemplate is the embedded Caddyfile template.
const caddyfileTemplate = `{{.Domain}} {
	tls {{.TLSConfig}}

	reverse_proxy {{.ActiveContainer}}:{{.Port}} {
		health_uri {{.HealthPath}}
		health_interval 5s
	}

	log {
		output file /data/logs/caddy.log {
			roll_size 50MiB
			roll_keep 5
			roll_keep_for 168h
		}
		format json
	}

	encode zstd gzip

	header {
		X-Content-Type-Options nosniff
		X-Frame-Options DENY
		Referrer-Policy strict-origin-when-cross-origin
	}
}
`

// generateCaddyfile creates the Caddyfile for the current active container.
func (m *Matcha) generateCaddyfile(data *envData) error {
	activeContainer := m.AppContainerName(1)
	if m.config.BlueGreen {
		if slot := m.getActiveSlot(); slot > 0 {
			activeContainer = m.AppContainerName(slot)
		}
	} else {
		activeContainer = m.AppContainerName(0)
	}

	return m.generateCaddyfileForContainer(data, activeContainer)
}

// generateCaddyfileForContainer creates the Caddyfile pointing to a specific container.
func (m *Matcha) generateCaddyfileForContainer(data *envData, containerName string) error {
	tlsConfig := generateAdminEmail(data.Domain)
	if os.Getenv("ENV") == "test" {
		tlsConfig = "internal"
	}

	content := caddyfileTemplate
	content = strings.ReplaceAll(content, "{{.Domain}}", data.Domain)
	content = strings.ReplaceAll(content, "{{.TLSConfig}}", tlsConfig)
	content = strings.ReplaceAll(content, "{{.ActiveContainer}}", containerName)
	content = strings.ReplaceAll(content, "{{.Port}}", fmt.Sprintf("%d", m.config.AppPort))
	content = strings.ReplaceAll(content, "{{.HealthPath}}", m.config.HealthPath)

	caddyPath := m.config.InstallDir + "/Caddyfile"
	return os.WriteFile(caddyPath, []byte(content), 0644)
}

// generateAdminEmail creates an admin email for Let's Encrypt.
func generateAdminEmail(domain string) string {
	baseDomain := extractBaseDomain(domain)
	return fmt.Sprintf("admin@%s", baseDomain)
}

// extractBaseDomain gets the base domain from a subdomain.
func extractBaseDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// Handle localhost
	if domain == "localhost" || strings.HasPrefix(domain, "localhost:") {
		return domain
	}

	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}

	// Return last two parts
	return strings.Join(parts[len(parts)-2:], ".")
}
