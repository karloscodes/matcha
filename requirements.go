package matcha

import (
	"fmt"
	"net"
)

// checkPorts verifies that required ports are available.
func (m *Matcha) checkPorts() error {
	ports := []int{80, 443}
	var unavailable []int

	for _, port := range ports {
		if !isPortAvailable(port) {
			unavailable = append(unavailable, port)
		}
	}

	if len(unavailable) > 0 {
		return fmt.Errorf("ports %v are not available", unavailable)
	}

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
