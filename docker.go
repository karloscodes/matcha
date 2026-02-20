package matcha

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	maxRetries = 3
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
	// Check if already installed
	if _, err := m.runDocker("version"); err == nil {
		return nil
	}

	// Install Docker
	cmd := exec.Command("bash", "-c", "curl -fsSL https://get.docker.com | sh")
	if out, err := cmd.CombinedOutput(); err != nil {
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

// pullImages pulls the app and proxy images.
func (m *Matcha) pullImages() error {
	images := []string{m.config.AppImage, m.config.ProxyImage}

	for _, image := range images {
		for i := 0; i < maxRetries; i++ {
			if _, err := m.runDocker("pull", image); err == nil {
				break
			} else if i == maxRetries-1 {
				return fmt.Errorf("failed to pull %s after %d retries", image, maxRetries)
			}
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

// deployProxy starts the kamal-proxy container.
func (m *Matcha) deployProxy() error {
	name := m.ProxyContainerName()
	m.stopAndRemove(name)

	args := []string{
		"run", "-d",
		"--name", name,
		"--network", m.NetworkName(),
		"-p", "80:80",
		"-p", "443:443",
		"-p", "443:443/udp",
		"-v", m.config.InstallDir + "/kamal-proxy-data:/home/kamal-proxy/.config/kamal-proxy",
		"--memory=128m",
		"--restart", "unless-stopped",
		m.config.ProxyImage,
	}

	_, err := m.runDocker(args...)
	return err
}

// pruneImages removes unused images.
func (m *Matcha) pruneImages() error {
	_, err := m.runDocker("image", "prune", "-f")
	return err
}
