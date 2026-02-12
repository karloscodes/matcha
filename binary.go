package matcha

import (
	"fmt"
	"os"
)

// installBinary copies the current executable to the binary path.
func (m *Matcha) installBinary() error {
	if os.Getenv("ENV") == "test" {
		return nil
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	data, err := os.ReadFile(currentExe)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	if err := os.WriteFile(m.config.BinaryPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	m.logger.Info("Binary installed to %s", m.config.BinaryPath)
	return nil
}

// upgradeBinary downloads and installs the latest binary.
func (m *Matcha) upgradeBinary() error {
	// For now, just copy the current binary
	// In the future, this could download from GitHub releases
	return m.installBinary()
}
