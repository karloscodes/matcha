package matcha

import (
	"fmt"
	"os"
)

const cronTemplate = `# Matcha auto-update for %s
# Runs daily at 3 AM
0 3 * * * root %s update >> %s/logs/update.log 2>&1
`

// setupCron creates a cron job for automatic updates.
func (m *Matcha) setupCron() error {
	logsDir := m.DataDir() + "/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	cronFile := fmt.Sprintf("/etc/cron.d/%s-update", m.config.Name)
	content := fmt.Sprintf(cronTemplate, m.config.Name, m.config.BinaryPath, m.DataDir())

	if err := os.WriteFile(cronFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write cron file: %w", err)
	}

	return nil
}
