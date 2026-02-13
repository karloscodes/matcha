package matcha

import (
	"fmt"
	"os/exec"
)

// ensureSQLite installs SQLite if not already present.
func (m *Matcha) ensureSQLite() error {
	// Check if already installed
	if err := exec.Command("sqlite3", "--version").Run(); err == nil {
		return nil
	}

	// Install via apt-get (Debian/Ubuntu)
	cmd := exec.Command("apt-get", "install", "-y", "sqlite3")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install SQLite: %w\n%s", err, string(out))
	}

	// Verify installation
	if err := exec.Command("sqlite3", "--version").Run(); err != nil {
		return fmt.Errorf("SQLite installation verification failed: %w", err)
	}

	return nil
}
