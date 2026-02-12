package matcha

import (
	"fmt"
	"net"
)

// checkPorts verifies that required ports are available.
func (m *Matcha) checkPorts() error {
	m.logger.Step("Checking ports", "running")

	ports := []int{80, 443}
	var unavailable []int

	for _, port := range ports {
		if !isPortAvailable(port) {
			unavailable = append(unavailable, port)
		}
	}

	if len(unavailable) > 0 {
		m.logger.Step("Checking ports", "failed")
		return fmt.Errorf("ports %v are not available", unavailable)
	}

	m.logger.Step("Checking ports", "done")
	return nil
}

// isPortAvailable checks if a port is available for binding.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
