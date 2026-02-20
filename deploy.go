package matcha

import "fmt"

func (m *Matcha) deploy() error {
	if err := m.createNetwork(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	if !m.isRunning(m.ProxyContainerName()) {
		if err := m.deployProxy(); err != nil {
			return fmt.Errorf("failed to deploy proxy: %w", err)
		}
	}

	containerName := m.AppContainerName()
	if err := m.deployApp(containerName); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	if err := m.deployToProxy(m.domain); err != nil {
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	return nil
}
