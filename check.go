package matcha

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// CheckResult holds the result of a single security check.
type CheckResult struct {
	Name     string
	Passed   bool
	Optional bool
	Fixes    []string
}

// Check runs security checks and prints results.
// Returns nil if all required checks pass, error otherwise.
func Check() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("security check requires root privileges (try: sudo %s check)", os.Args[0])
	}

	fmt.Println()
	fmt.Println(bold("Security check"))
	fmt.Println()

	var results []CheckResult

	results = append(results, checkSSHPasswordAuth())
	results = append(results, checkSSHRootLogin())
	results = append(results, checkFirewall())
	results = append(results, checkFirewallPorts())
	results = append(results, checkUnattendedUpgrades())
	results = append(results, checkTailscale())
	results = append(results, checkCloudflare()...)

	requiredTotal, requiredPassed := countResults(results)
	optionalTotal, optionalPassed := countOptionalResults(results)

	for _, r := range results {

		if r.Passed {
			fmt.Printf("  %s %s\n", green("[x]"), r.Name)
		} else if r.Optional {
			fmt.Printf("  %s %s %s\n", dim("[ ]"), r.Name, dim("(optional)"))
		} else {
			fmt.Printf("  %s %s\n", "[ ]", r.Name)
		}

		for _, fix := range r.Fixes {
			fmt.Printf("      %s %s\n", dim("→"), fix)
		}
	}

	fmt.Println()
	fmt.Printf("Result: %d/%d required checks passed.", requiredPassed, requiredTotal)
	if optionalTotal > 0 {
		fmt.Printf(" %d/%d optional.", optionalPassed, optionalTotal)
	}
	fmt.Println()

	if requiredPassed < requiredTotal {
		return fmt.Errorf("%d required check(s) failed", requiredTotal-requiredPassed)
	}
	return nil
}

func checkSSHPasswordAuth() CheckResult {
	r := CheckResult{Name: "SSH password authentication disabled"}

	passed, err := sshConfigHas("PasswordAuthentication", "no")
	if err != nil {
		r.Fixes = append(r.Fixes, fmt.Sprintf("Could not read sshd_config: %v", err))
		return r
	}

	r.Passed = passed
	if !passed {
		r.Fixes = append(r.Fixes, "Edit /etc/ssh/sshd_config: set PasswordAuthentication no")
		r.Fixes = append(r.Fixes, "Then: systemctl restart sshd")
	}
	return r
}

func checkSSHRootLogin() CheckResult {
	r := CheckResult{Name: "SSH root login disabled"}

	passed, err := sshConfigHas("PermitRootLogin", "no")
	if err != nil {
		r.Fixes = append(r.Fixes, fmt.Sprintf("Could not read sshd_config: %v", err))
		return r
	}

	r.Passed = passed
	if !passed {
		r.Fixes = append(r.Fixes, "Edit /etc/ssh/sshd_config: set PermitRootLogin no")
		r.Fixes = append(r.Fixes, "Then: systemctl restart sshd")
	}
	return r
}

func checkFirewall() CheckResult {
	r := CheckResult{Name: "Firewall (ufw) active"}

	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		r.Fixes = append(r.Fixes, "Install and enable: apt install ufw && ufw allow 22 && ufw allow 80 && ufw allow 443 && ufw enable")
		return r
	}

	r.Passed = strings.Contains(string(out), "Status: active")
	if !r.Passed {
		r.Fixes = append(r.Fixes, "Enable: ufw allow 22 && ufw allow 80 && ufw allow 443 && ufw enable")
	}
	return r
}

func checkFirewallPorts() CheckResult {
	r := CheckResult{Name: "No unexpected open ports"}

	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		r.Passed = true
		return r
	}

	if !strings.Contains(string(out), "Status: active") {
		r.Passed = true
		return r
	}

	unexpected := parseUnexpectedPorts(string(out))
	r.Passed = len(unexpected) == 0
	for _, port := range unexpected {
		r.Fixes = append(r.Fixes, fmt.Sprintf("Unexpected port %s open. Run: ufw delete allow %s", port, port))
	}
	return r
}

// parseUnexpectedPorts extracts ports from ufw status output that aren't 22, 80, or 443.
func parseUnexpectedPorts(ufwOutput string) []string {
	allowed := map[string]bool{
		"22": true, "80": true, "443": true,
		"22/tcp": true, "80/tcp": true, "443/tcp": true,
	}

	var unexpected []string
	for _, line := range strings.Split(ufwOutput, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "ALLOW") {
			continue
		}

		port := strings.Fields(line)[0]
		base := strings.TrimSuffix(port, "/tcp")
		base = strings.TrimSuffix(base, "/udp")
		if !allowed[base] && !allowed[port] {
			unexpected = append(unexpected, port)
		}
	}
	return unexpected
}

func checkUnattendedUpgrades() CheckResult {
	r := CheckResult{Name: "Unattended security upgrades enabled"}

	_, err := exec.Command("dpkg", "-s", "unattended-upgrades").CombinedOutput()
	r.Passed = err == nil
	if !r.Passed {
		r.Fixes = append(r.Fixes, "Install: apt install -y unattended-upgrades")
	}
	return r
}

func checkTailscale() CheckResult {
	r := CheckResult{Name: "Tailscale installed", Optional: true}

	_, err := exec.LookPath("tailscale")
	r.Passed = err == nil
	if !r.Passed {
		r.Fixes = append(r.Fixes, "Install: curl -fsSL https://tailscale.com/install.sh | sh")
	}
	return r
}

// cloudflareRanges lists known Cloudflare IPv4 CIDR ranges.
var cloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22",
	"103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18",
	"190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22",
	"198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
}

// isCloudflareIP checks if an IP belongs to a known Cloudflare range.
func isCloudflareIP(ip net.IP) bool {
	for _, cidr := range cloudflareRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func checkCloudflare() []CheckResult {
	apps, err := ListApps()
	if err != nil || len(apps) == 0 {
		return nil
	}

	var results []CheckResult
	for _, name := range ListAppsSorted(apps) {
		app := apps[name]
		if isLocalhost(app.Domain) {
			continue
		}

		r := CheckResult{
			Name:     fmt.Sprintf("Cloudflare proxy active for %s", app.Domain),
			Optional: true,
		}

		ips, err := net.LookupIP(app.Domain)
		if err != nil || len(ips) == 0 {
			r.Fixes = append(r.Fixes, fmt.Sprintf("DNS not resolving for %s", app.Domain))
			results = append(results, r)
			continue
		}

		behindCF := false
		for _, ip := range ips {
			if isCloudflareIP(ip) {
				behindCF = true
				break
			}
		}

		r.Passed = behindCF
		if !r.Passed {
			r.Fixes = append(r.Fixes, "Enable Cloudflare proxy (orange cloud) for this domain")
			r.Fixes = append(r.Fixes, "See: https://github.com/karloscodes/matcha#recommended-cloudflare-proxy")
		}
		results = append(results, r)
	}

	return results
}

// countResults returns (total, passed) for required checks only.
func countResults(results []CheckResult) (int, int) {
	total, passed := 0, 0
	for _, r := range results {
		if !r.Optional {
			total++
			if r.Passed {
				passed++
			}
		}
	}
	return total, passed
}

// countOptionalResults returns (total, passed) for optional checks only.
func countOptionalResults(results []CheckResult) (int, int) {
	total, passed := 0, 0
	for _, r := range results {
		if r.Optional {
			total++
			if r.Passed {
				passed++
			}
		}
	}
	return total, passed
}

// sshConfigHas checks if an SSH config directive is set to the expected value.
func sshConfigHas(key, expected string) (bool, error) {
	return sshConfigHasFrom("/etc/ssh/sshd_config", key, expected)
}

func sshConfigHasFrom(path, key, expected string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], key) {
			return strings.EqualFold(fields[1], expected), nil
		}
	}

	return false, nil
}
