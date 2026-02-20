package matcha

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxBackups = 3

// BackupFile represents a database backup file.
type BackupFile struct {
	Name      string
	Path      string
	CreatedAt time.Time
	Size      int64
}

// listBackups returns available backup files sorted by date (newest first).
func (m *Matcha) listBackups() ([]BackupFile, error) {
	backupDir := filepath.Join(m.DataDir(), "backups")

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var backups []BackupFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup_") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}

		// Parse timestamp from filename: backup_20060102_150405.db
		timePart := strings.TrimPrefix(strings.TrimSuffix(entry.Name(), ".db"), "backup_")
		createdAt, err := time.Parse("20060102_150405", timePart)
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}

		backups = append(backups, BackupFile{
			Name:      entry.Name(),
			Path:      filepath.Join(backupDir, entry.Name()),
			CreatedAt: createdAt,
			Size:      size,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// promptBackupSelection asks the user to select a backup.
func (m *Matcha) promptBackupSelection(backups []BackupFile) (string, error) {
	fmt.Println("\nAvailable backups:")
	for i, backup := range backups {
		fmt.Printf("  %d. %s (%s, %d bytes)\n", i+1, backup.Name, backup.CreatedAt.Format("2006-01-02 15:04"), backup.Size)
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

	return backups[selection-1].Path, nil
}

// createBackup creates a backup using SQLite's .backup command.
func (m *Matcha) createBackup() (string, error) {
	dataDir := m.DataDir()
	backupDir := filepath.Join(dataDir, "backups")

	// Find the database file
	dbPath := m.findDatabase()
	if dbPath == "" {
		return "", fmt.Errorf("no database found in %s", dataDir)
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_%s.db", timestamp))

	cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".backup '%s'", backupPath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("sqlite3 backup failed: %w - %s", err, stderr.String())
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return "", fmt.Errorf("backup file not created: %w", err)
	}
	if info.Size() == 0 {
		os.Remove(backupPath)
		return "", fmt.Errorf("backup file is empty")
	}

	if err := m.validateBackup(backupPath); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("backup validation failed: %w", err)
	}

	m.cleanupOldBackups()

	return backupPath, nil
}

// findDatabase returns the path to the first .db file in the data dir.
func (m *Matcha) findDatabase() string {
	dataDir := m.DataDir()
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".db") {
			return filepath.Join(dataDir, entry.Name())
		}
	}
	return ""
}

// validateBackup checks backup integrity using SQLite PRAGMA.
func (m *Matcha) validateBackup(backupPath string) error {
	cmd := exec.Command("sqlite3", backupPath, "PRAGMA integrity_check;")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("integrity check failed: %w - %s", err, stderr.String())
	}

	if output := strings.TrimSpace(stdout.String()); output != "ok" {
		return fmt.Errorf("integrity issues: %s", output)
	}

	return nil
}

// restoreBackup restores a database from backup using sqlite3.
func (m *Matcha) restoreBackup(backupPath string) error {
	dbPath := m.findDatabase()
	if dbPath == "" {
		return fmt.Errorf("no database found to restore into")
	}

	if err := m.validateBackup(backupPath); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	safetyBackup := dbPath + ".bak." + time.Now().Format("20060102150405")
	if err := os.Rename(dbPath, safetyBackup); err != nil {
		return fmt.Errorf("failed to backup current database: %w", err)
	}

	cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".restore '%s'", backupPath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 restore failed: %w - %s", err, stderr.String())
	}

	return nil
}

// cleanupOldBackups keeps only the last 3 backups, removes the rest.
func (m *Matcha) cleanupOldBackups() {
	backups, err := m.listBackups()
	if err != nil {
		return
	}

	// backups are sorted newest-first; remove everything past maxBackups
	for i := maxBackups; i < len(backups); i++ {
		os.Remove(backups[i].Path)
	}
}
