package matcha

import "fmt"

// proxyDeployArgs builds the docker exec args for kamal-proxy deploy.
func (m *Matcha) proxyDeployArgs(domain string) []string {
	serviceName := m.config.Name
	proxyContainer := m.ProxyContainerName()
	target := fmt.Sprintf("%s:%d", m.AppContainerName(), m.config.AppPort)

	args := []string{
		"exec", proxyContainer, "kamal-proxy", "deploy", serviceName,
		"--target", target,
	}

	if !isLocalhost(domain) {
		args = append(args, "--host", domain, "--tls")
	}

	args = append(args, "--health-check-path", m.config.HealthPath)

	return args
}

// deployToProxy registers the app container with kamal-proxy.
func (m *Matcha) deployToProxy(domain string) error {
	args := m.proxyDeployArgs(domain)
	_, err := m.runDocker(args...)
	if err != nil {
		return fmt.Errorf("kamal-proxy deploy failed: %w", err)
	}
	return nil
}

// removeFromProxy removes the service from kamal-proxy.
func (m *Matcha) removeFromProxy() error {
	proxyContainer := m.ProxyContainerName()
	_, err := m.runDocker("exec", proxyContainer, "kamal-proxy", "remove", m.config.Name)
	return err
}
