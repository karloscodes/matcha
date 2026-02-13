package matcha

import "fmt"

// deploy handles the deployment logic.
func (m *Matcha) deploy() error {
	// Read config
	data, err := m.readEnv()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Create network
	if err := m.createNetwork(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Generate Caddyfile
	if err := m.generateCaddyfile(data); err != nil {
		return fmt.Errorf("failed to generate Caddyfile: %w", err)
	}

	if m.config.BlueGreen {
		if err := m.deployBlueGreen(data); err != nil {
			return err
		}
	} else {
		if err := m.deploySingle(data); err != nil {
			return err
		}
	}

	return nil
}

// deployBlueGreen performs a blue-green deployment.
func (m *Matcha) deployBlueGreen(data *envData) error {
	activeSlot := m.getActiveSlot()
	newSlot := 1
	if activeSlot == 1 {
		newSlot = 2
	}

	newContainer := m.AppContainerName(newSlot)

	// Deploy new container
	if err := m.deployApp(newContainer, data); err != nil {
		return fmt.Errorf("failed to deploy %s: %w", newContainer, err)
	}

	// Wait for health
	if err := m.waitForHealth(newContainer); err != nil {
		m.stopAndRemove(newContainer)
		return fmt.Errorf("health check failed for %s: %w", newContainer, err)
	}

	// Update Caddyfile to point to new container
	if err := m.generateCaddyfileForContainer(data, newContainer); err != nil {
		m.stopAndRemove(newContainer)
		return fmt.Errorf("failed to update Caddyfile: %w", err)
	}

	// Deploy or reload Caddy
	caddyName := m.CaddyContainerName()
	if m.isRunning(caddyName) {
		if err := m.reloadCaddy(); err != nil {
			// Fallback: redeploy Caddy
			if err := m.deployCaddy(data); err != nil {
				return fmt.Errorf("failed to redeploy Caddy: %w", err)
			}
		}
	} else {
		if err := m.deployCaddy(data); err != nil {
			return fmt.Errorf("failed to deploy Caddy: %w", err)
		}
	}

	// Cleanup old container
	if activeSlot > 0 {
		oldContainer := m.AppContainerName(activeSlot)
		m.stopAndRemove(oldContainer)
	}

	return nil
}

// deploySingle performs a single-container deployment.
func (m *Matcha) deploySingle(data *envData) error {
	containerName := m.AppContainerName(0)

	// Deploy app
	if err := m.deployApp(containerName, data); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	// Wait for health
	if err := m.waitForHealth(containerName); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Deploy Caddy if not running
	caddyName := m.CaddyContainerName()
	if !m.isRunning(caddyName) {
		if err := m.deployCaddy(data); err != nil {
			return fmt.Errorf("failed to deploy Caddy: %w", err)
		}
	}

	return nil
}
