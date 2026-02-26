package matcha

import (
	"fmt"
	"path/filepath"
)

func (m *Matcha) deploy() error {
	if err := m.createNetwork(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	if !m.isRunning(m.ProxyContainerName()) {
		if err := m.deployProxy(); err != nil {
			return fmt.Errorf("failed to deploy proxy: %w", err)
		}
	}

	// Pre-deploy backup
	if m.config.Backups {
		if path, err := m.createBackup(); err == nil {
			printSuccess("Backup: %s", filepath.Base(path))
		}
		// Best-effort: don't fail deploy on backup error
	}

	// Zero-downtime deploy:
	// 1. Find current container, pick alternate name for new one
	// 2. Start new container (old still serves traffic)
	// 3. Register new with proxy (health-checks, then switches traffic)
	// 4. Remove old container
	oldContainer := m.findActiveContainer()
	newContainer := m.nextContainerName(oldContainer)

	if err := m.deployApp(newContainer); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	if err := m.deployToProxy(m.domain, newContainer); err != nil {
		// New container failed health check — clean it up, old is still running
		m.stopAndRemove(newContainer)
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	// Traffic has switched — safe to remove old container
	if oldContainer != newContainer {
		m.stopAndRemove(oldContainer)
	}

	return nil
}

