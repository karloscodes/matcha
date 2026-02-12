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
	m.logger.Step("Cron", "running")

	cronFile := fmt.Sprintf("/etc/cron.d/%s-update", m.config.Name)
	content := fmt.Sprintf(cronTemplate, m.config.Name, m.config.BinaryPath, m.config.InstallDir)

	if err := os.WriteFile(cronFile, []byte(content), 0644); err != nil {
		m.logger.Step("Cron", "failed")
		return fmt.Errorf("failed to write cron file: %w", err)
	}

	m.logger.Step("Cron", "done")
	m.logger.Info("Auto-update scheduled daily at 3:00 AM")
	return nil
}
