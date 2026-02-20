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

	containerName := m.AppContainerName()
	if err := m.deployApp(containerName); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	if err := m.deployToProxy(m.domain); err != nil {
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	return nil
}

