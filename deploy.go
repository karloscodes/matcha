package matcha

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Pre-deploy SQLite snapshot
	m.snapshotSQLiteDatabases()

	containerName := m.AppContainerName()
	if err := m.deployApp(containerName); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	if err := m.deployToProxy(m.domain); err != nil {
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	return nil
}

// snapshotSQLiteDatabases finds .db files in the data dir and creates
// pre-deploy snapshots using sqlite3 .backup. Skips gracefully if
// sqlite3 is not available or no databases are found.
func (m *Matcha) snapshotSQLiteDatabases() {
	dataDir := m.DataDir()
	if _, err := os.Stat(dataDir); err != nil {
		return
	}

	// Check if sqlite3 is available
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return
	}

	// Find .db files
	var dbFiles []string
	filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".db") {
			dbFiles = append(dbFiles, path)
		}
		return nil
	})

	if len(dbFiles) == 0 {
		return
	}

	// Create snapshot directory
	snapshotDir := filepath.Join(dataDir, ".pre-deploy")
	os.MkdirAll(snapshotDir, 0755)

	for _, dbPath := range dbFiles {
		snapshotPath := filepath.Join(snapshotDir, filepath.Base(dbPath))
		cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".backup '%s'", snapshotPath))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Run() // best-effort, don't fail deploy
	}
}
