package matcha

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	maxRetries       = 3
	healthCheckTries = 5
)

// runDocker executes a docker command and returns output.
func (m *Matcha) runDocker(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// ensureDocker installs Docker if not present.
func (m *Matcha) ensureDocker() error {
	m.logger.Step("Docker", "running")

	// Check if already installed
	if out, err := m.runDocker("version"); err == nil {
		m.logger.Step("Docker", "done")
		m.logger.Info("Docker already installed: %s", strings.Split(out, "\n")[0])
		return nil
	}

	// Install Docker
	m.logger.Info("Installing Docker...")
	cmd := exec.Command("bash", "-c", "curl -fsSL https://get.docker.com | sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		m.logger.Step("Docker", "failed")
		return fmt.Errorf("docker install failed: %w\n%s", err, string(out))
	}

	// Start and enable
	for _, c := range [][]string{
		{"systemctl", "start", "docker"},
		{"systemctl", "enable", "docker"},
	} {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			return fmt.Errorf("%s failed: %w", c[1], err)
		}
	}

	m.logger.Step("Docker", "done")
	return nil
}

// createNetwork creates the Docker network if it doesn't exist.
func (m *Matcha) createNetwork() error {
	name := m.NetworkName()
	if _, err := m.runDocker("network", "inspect", name); err == nil {
		return nil // already exists
	}

	_, err := m.runDocker("network", "create", name)
	return err
}

// pullImages pulls the app and caddy images.
func (m *Matcha) pullImages() error {
	images := []string{m.config.AppImage, m.config.CaddyImage}

	for _, image := range images {
		m.logger.Info("Pulling %s...", image)
		for i := 0; i < maxRetries; i++ {
			if _, err := m.runDocker("pull", image); err == nil {
				m.logger.Success("Pulled %s", image)
				break
			} else if i == maxRetries-1 {
				return fmt.Errorf("failed to pull %s after %d retries", image, maxRetries)
			}
			m.logger.Warn("Pull failed, retrying (%d/%d)", i+1, maxRetries)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
		}
	}
	return nil
}

// isRunning checks if a container is running.
func (m *Matcha) isRunning(name string) bool {
	out, err := m.runDocker("ps", "-q", "-f", "name="+name)
	return err == nil && strings.TrimSpace(out) != ""
}

// stopAndRemove stops and removes a container.
func (m *Matcha) stopAndRemove(name string) error {
	m.runDocker("stop", name)
	m.runDocker("rm", "-f", name)
	return nil
}

// deployApp deploys an app container.
func (m *Matcha) deployApp(name string, data *envData) error {
	m.stopAndRemove(name)

	prefix := m.EnvPrefix()
	args := []string{
		"run", "-d",
		"--name", name,
		"--network", m.NetworkName(),
		"--pull", "always",
		"-v", m.config.InstallDir + "/storage:/app/storage",
		"-v", m.config.InstallDir + "/logs:/app/logs",
		"-e", fmt.Sprintf("%s_DOMAIN=%s", prefix, data.Domain),
		"-e", fmt.Sprintf("%s_PRIVATE_KEY=%s", prefix, data.PrivateKey),
		"-e", fmt.Sprintf("%s_APP_PORT=%d", prefix, m.config.AppPort),
		"-e", fmt.Sprintf("%s_ENV=production", prefix),
		"--memory=512m",
		"--restart", "unless-stopped",
		m.config.AppImage,
	}

	_, err := m.runDocker(args...)
	return err
}

// deployCaddy deploys the Caddy container.
func (m *Matcha) deployCaddy(data *envData) error {
	name := m.CaddyContainerName()
	m.stopAndRemove(name)

	caddyFile := m.config.InstallDir + "/Caddyfile"

	args := []string{
		"run", "-d",
		"--name", name,
		"--network", m.NetworkName(),
		"--pull", "always",
		"-p", "80:80",
		"-p", "443:443",
		"-p", "443:443/udp",
		"-v", caddyFile + ":/etc/caddy/Caddyfile:ro",
		"-v", m.config.InstallDir + "/caddy:/data",
		"-v", m.config.InstallDir + "/caddy/config:/config",
		"-v", m.config.InstallDir + "/logs:/data/logs",
		"-e", "DOMAIN=" + data.Domain,
		"--memory=256m",
		"--restart", "unless-stopped",
		m.config.CaddyImage,
	}

	_, err := m.runDocker(args...)
	return err
}

// waitForHealth waits for a container to become healthy.
func (m *Matcha) waitForHealth(name string) error {
	healthURL := fmt.Sprintf("http://localhost:%d%s", m.config.AppPort, m.config.HealthPath)

	for i := 0; i < healthCheckTries; i++ {
		if _, err := m.runDocker("exec", name, "curl", "-sf", healthURL); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("container %s not healthy after %d attempts", name, healthCheckTries)
}

// reloadCaddy reloads Caddy configuration.
func (m *Matcha) reloadCaddy() error {
	name := m.CaddyContainerName()
	_, err := m.runDocker("exec", name, "caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	return err
}

// pruneImages removes unused images.
func (m *Matcha) pruneImages() error {
	_, err := m.runDocker("image", "prune", "-f")
	return err
}

// getActiveSlot returns which slot (1 or 2) is currently running.
func (m *Matcha) getActiveSlot() int {
	if m.isRunning(m.AppContainerName(1)) {
		return 1
	}
	if m.isRunning(m.AppContainerName(2)) {
		return 2
	}
	return 0
}
