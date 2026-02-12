package matcha

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// listBackups returns available backup files.
func (m *Matcha) listBackups() ([]string, error) {
	backupDir := m.config.InstallDir + "/storage/backups"

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".db") {
			backups = append(backups, filepath.Join(backupDir, entry.Name()))
		}
	}

	// Sort by modification time, newest first
	sort.Slice(backups, func(i, j int) bool {
		infoI, _ := os.Stat(backups[i])
		infoJ, _ := os.Stat(backups[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return backups, nil
}

// promptBackupSelection asks the user to select a backup.
func (m *Matcha) promptBackupSelection(backups []string) (string, error) {
	fmt.Println("\nAvailable backups:")
	for i, backup := range backups {
		info, _ := os.Stat(backup)
		name := filepath.Base(backup)
		fmt.Printf("  %d. %s (%s)\n", i+1, name, info.ModTime().Format("2006-01-02 15:04"))
	}

	fmt.Print("\nSelect backup number: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	var selection int
	if _, err := fmt.Sscanf(strings.TrimSpace(input), "%d", &selection); err != nil {
		return "", fmt.Errorf("invalid selection")
	}

	if selection < 1 || selection > len(backups) {
		return "", fmt.Errorf("selection out of range")
	}

	return backups[selection-1], nil
}

// restoreBackup restores a database from backup.
func (m *Matcha) restoreBackup(backupPath string) error {
	dbPath := m.config.InstallDir + "/storage/" + m.config.Name + "-production.db"

	// Read backup
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Write to database location
	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write database: %w", err)
	}

	return nil
}
