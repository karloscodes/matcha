package matcha

import (
	"fmt"
	"os"
	"path/filepath"
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

	// Resolve symlinks to compare real paths.
	realExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		realExe = currentExe
	}
	realTarget, err := filepath.EvalSymlinks(m.config.BinaryPath)
	if err != nil {
		realTarget = m.config.BinaryPath
	}

	if realExe == realTarget {
		return nil
	}

	data, err := os.ReadFile(currentExe)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	if err := os.WriteFile(m.config.BinaryPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	return nil
}
