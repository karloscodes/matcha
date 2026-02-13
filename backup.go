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

// BackupType represents the type of backup (daily, weekly, monthly).
type BackupType string

const (
	BackupDaily   BackupType = "daily"
	BackupWeekly  BackupType = "weekly"
	BackupMonthly BackupType = "monthly"
)

// BackupFile represents a database backup file.
type BackupFile struct {
	Name       string
	Path       string
	BackupType BackupType
	CreatedAt  time.Time
	Size       int64
}

// RetentionConfig defines the retention period for each backup type.
type RetentionConfig struct {
	DailyDays   int // Keep daily backups for N days (default: 7)
	WeeklyDays  int // Keep weekly backups for N days (default: 14)
	MonthlyDays int // Keep monthly backups for N days (default: 90)
}

// DefaultRetention returns sensible default retention values.
func DefaultRetention() RetentionConfig {
	return RetentionConfig{
		DailyDays:   7,
		WeeklyDays:  14,
		MonthlyDays: 90,
	}
}

// determineBackupType determines the type of backup based on creation time.
func determineBackupType(t time.Time) BackupType {
	if t.Day() == 1 {
		return BackupMonthly
	}
	if t.Weekday() == time.Sunday {
		return BackupWeekly
	}
	return BackupDaily
}

// listBackups returns available backup files sorted by date (newest first).
func (m *Matcha) listBackups() ([]BackupFile, error) {
	backupDir := m.config.InstallDir + "/storage/backups"

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
			continue // skip files with invalid timestamp
		}

		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}

		backups = append(backups, BackupFile{
			Name:       entry.Name(),
			Path:       filepath.Join(backupDir, entry.Name()),
			BackupType: determineBackupType(createdAt),
			CreatedAt:  createdAt,
			Size:       size,
		})
	}

	// Sort by creation time, newest first
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
	dbPath := m.config.InstallDir + "/storage/" + m.config.Name + "-production.db"
	backupDir := m.config.InstallDir + "/storage/backups"

	// Check database exists
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("database not found: %w", err)
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_%s.db", timestamp))

	// Use sqlite3 .backup command (safe for live databases)
	cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".backup '%s'", backupPath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("sqlite3 backup failed: %w - %s", err, stderr.String())
	}

	// Verify backup was created and has content
	info, err := os.Stat(backupPath)
	if err != nil {
		return "", fmt.Errorf("backup file not created: %w", err)
	}
	if info.Size() == 0 {
		os.Remove(backupPath)
		return "", fmt.Errorf("backup file is empty")
	}

	// Validate backup integrity
	if err := m.validateBackup(backupPath); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("backup validation failed: %w", err)
	}

	// Cleanup old backups according to retention policy
	m.cleanupOldBackups()

	return backupPath, nil
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

	output := strings.TrimSpace(stdout.String())
	if output != "ok" {
		return fmt.Errorf("integrity issues: %s", output)
	}

	return nil
}

// restoreBackup restores a database from backup using sqlite3.
func (m *Matcha) restoreBackup(backupPath string) error {
	dbPath := m.config.InstallDir + "/storage/" + m.config.Name + "-production.db"

	// Validate backup first
	if err := m.validateBackup(backupPath); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	// Create safety backup of current database
	if _, err := os.Stat(dbPath); err == nil {
		safetyBackup := dbPath + ".bak." + time.Now().Format("20060102150405")
		if err := os.Rename(dbPath, safetyBackup); err != nil {
			return fmt.Errorf("failed to backup current database: %w", err)
		}
	}

	// Restore using sqlite3 .restore command
	cmd := exec.Command("sqlite3", dbPath, fmt.Sprintf(".restore '%s'", backupPath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 restore failed: %w - %s", err, stderr.String())
	}

	return nil
}

// cleanupOldBackups removes backups older than retention policy.
func (m *Matcha) cleanupOldBackups() {
	retention := DefaultRetention()

	backups, err := m.listBackups()
	if err != nil {
		return
	}

	now := time.Now()
	for _, backup := range backups {
		age := now.Sub(backup.CreatedAt)

		var maxAge time.Duration
		switch backup.BackupType {
		case BackupDaily:
			maxAge = time.Duration(retention.DailyDays) * 24 * time.Hour
		case BackupWeekly:
			maxAge = time.Duration(retention.WeeklyDays) * 24 * time.Hour
		case BackupMonthly:
			maxAge = time.Duration(retention.MonthlyDays) * 24 * time.Hour
		}

		if age > maxAge {
			os.Remove(backup.Path)
		}
	}
}
