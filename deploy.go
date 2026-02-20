package matcha

import "fmt"

// deploy handles the deployment logic.
func (m *Matcha) deploy() error {
	data, err := m.readEnv()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := m.createNetwork(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Ensure proxy is running
	if !m.isRunning(m.ProxyContainerName()) {
		if err := m.deployProxy(); err != nil {
			return fmt.Errorf("failed to deploy proxy: %w", err)
		}
	}

	// Deploy app container
	containerName := m.AppContainerName()
	if err := m.deployApp(containerName, data); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	// Register with kamal-proxy (handles health check + zero-downtime switch)
	if err := m.deployToProxy(data.Domain); err != nil {
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	return nil
}
