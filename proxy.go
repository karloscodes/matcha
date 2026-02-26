package matcha

import "fmt"

// proxyDeployArgs builds the docker exec args for kamal-proxy deploy.
func (m *Matcha) proxyDeployArgs(domain string, targetContainer string) []string {
	serviceName := m.config.Name
	proxyContainer := m.ProxyContainerName()
	target := fmt.Sprintf("%s:%d", targetContainer, m.config.AppPort)

	args := []string{
		"exec", proxyContainer, "kamal-proxy", "deploy", serviceName,
		"--target", target,
	}

	if domain == "localhost" {
		// Bare localhost: skip --host (catches all requests) and --tls
	} else if isLocalhost(domain) {
		// Localhost subdomains (e.g., app.localhost): set --host for routing, skip --tls
		args = append(args, "--host", domain)
	} else {
		args = append(args, "--host", domain, "--tls")
	}

	args = append(args, "--health-check-path", m.config.HealthPath)

	return args
}

// deployToProxy registers the app container with kamal-proxy.
func (m *Matcha) deployToProxy(domain string, targetContainer string) error {
	args := m.proxyDeployArgs(domain, targetContainer)
	_, err := m.runDocker(args...)
	if err != nil {
		return fmt.Errorf("kamal-proxy deploy failed: %w", err)
	}
	return nil
}

// RemoveFromProxy removes the service from kamal-proxy.
func (m *Matcha) RemoveFromProxy() error {
	proxyContainer := m.ProxyContainerName()
	_, err := m.runDocker("exec", proxyContainer, "kamal-proxy", "remove", m.config.Name)
	return err
}
