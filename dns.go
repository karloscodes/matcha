package matcha

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// dnsStatus holds DNS check results.
type dnsStatus struct {
	Found    bool
	MatchIP  bool
	DomainIP string
	ServerIP string
}

// checkDNS looks up the domain and compares with server IP.
func (m *Matcha) checkDNS(domain string) *dnsStatus {
	status := &dnsStatus{}

	// Skip for localhost
	if isLocalhost(domain) {
		status.Found = true
		status.MatchIP = true
		return status
	}

	// Get server's public IP
	status.ServerIP = getServerIP()

	// Lookup domain
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		status.Found = false
		return status
	}

	status.Found = true
	status.DomainIP = ips[0].String()

	// Check if any IP matches server IP
	for _, ip := range ips {
		if ip.String() == status.ServerIP {
			status.MatchIP = true
			break
		}
	}

	return status
}

// getServerIP gets this server's public IP address.
func getServerIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(body))
		if ip != "" {
			return ip
		}
	}

	return "unknown"
}

// isLocalhost checks if domain is localhost.
func isLocalhost(domain string) bool {
	domain = strings.ToLower(domain)
	return domain == "localhost" ||
		strings.HasPrefix(domain, "localhost:") ||
		strings.HasSuffix(domain, ".localhost") ||
		domain == "127.0.0.1" ||
		strings.HasPrefix(domain, "127.0.0.1:")
}

// extractSubdomain gets the subdomain part for DNS instructions.
// e.g., "analytics.example.com" -> "analytics", "example.com" -> "@"
func extractSubdomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return "@"
	}
	return parts[0]
}

// printDNSInstructions shows how to configure DNS.
func (m *Matcha) printDNSInstructions(domain string, serverIP string) {
	fmt.Println()
	fmt.Println(dim("Add an A record at your DNS provider:"))
	fmt.Println()
	fmt.Printf("    Name:   %s\n", extractSubdomain(domain))
	fmt.Println("    Type:   A")
	fmt.Printf("    Value:  %s  (this server)\n", serverIP)
	fmt.Println()
	fmt.Println(dim("SSL activates automatically once DNS propagates."))
	fmt.Println()
}
