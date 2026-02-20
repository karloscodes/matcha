package matcha

import (
	"fmt"
	"strings"
)

// showStatus displays the current deployment status.
func (m *Matcha) showStatus() error {
	fmt.Printf("\n%s Status\n", m.config.Name)
	fmt.Println(strings.Repeat("=", 40))

	// Check proxy
	proxyName := m.ProxyContainerName()
	if m.isRunning(proxyName) {
		fmt.Printf("  Proxy:    %s✓ running%s\n", "\033[0;32m", "\033[0m")
		m.showContainerInfo(proxyName)
	} else {
		fmt.Printf("  Proxy:    %s✗ not running%s\n", "\033[0;31m", "\033[0m")
	}

	// Check app container
	name := m.AppContainerName()
	if m.isRunning(name) {
		fmt.Printf("  App:      %s✓ running%s\n", "\033[0;32m", "\033[0m")
		m.showContainerInfo(name)
	} else {
		fmt.Printf("  App:      %s✗ not running%s\n", "\033[0;31m", "\033[0m")
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
