package matcha

import (
	"fmt"
	"strings"
)

// showStatus displays the current deployment status.
func (m *Matcha) showStatus() error {
	fmt.Printf("\n%s Status\n", m.config.Name)
	fmt.Println(strings.Repeat("=", 40))

	// Check Caddy
	caddyName := m.CaddyContainerName()
	if m.isRunning(caddyName) {
		fmt.Printf("  Caddy:    %s✓ running%s\n", "\033[0;32m", "\033[0m")
		m.showContainerInfo(caddyName)
	} else {
		fmt.Printf("  Caddy:    %s✗ not running%s\n", "\033[0;31m", "\033[0m")
	}

	// Check app container(s)
	if m.config.BlueGreen {
		for slot := 1; slot <= 2; slot++ {
			name := m.AppContainerName(slot)
			if m.isRunning(name) {
				fmt.Printf("  App (%d):  %s✓ running%s\n", slot, "\033[0;32m", "\033[0m")
				m.showContainerInfo(name)
			} else {
				fmt.Printf("  App (%d):  %s· not running%s\n", slot, "\033[0;33m", "\033[0m")
			}
		}
	} else {
		name := m.AppContainerName(0)
		if m.isRunning(name) {
			fmt.Printf("  App:      %s✓ running%s\n", "\033[0;32m", "\033[0m")
			m.showContainerInfo(name)
		} else {
			fmt.Printf("  App:      %s✗ not running%s\n", "\033[0;31m", "\033[0m")
		}
	}

	// Show config
	if data, err := m.readEnv(); err == nil {
		fmt.Println()
		fmt.Printf("  Domain:   https://%s\n", data.Domain)
	}

	fmt.Println()
	return nil
}

// showContainerInfo displays details for a container.
func (m *Matcha) showContainerInfo(name string) {
	// Get image
	if out, err := m.runDocker("inspect", "--format", "{{.Config.Image}}", name); err == nil {
		fmt.Printf("            Image: %s\n", strings.TrimSpace(out))
	}

	// Get uptime
	if out, err := m.runDocker("inspect", "--format", "{{.State.StartedAt}}", name); err == nil {
		fmt.Printf("            Started: %s\n", strings.TrimSpace(out)[:19])
	}
}
