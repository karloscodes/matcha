package matcha

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSSHConfigHas(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "sshd_config")

	t.Run("finds directive set to expected value", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("PasswordAuthentication no\n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Error("expected true, got false")
		}
	})

	t.Run("returns false when set to wrong value", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("PasswordAuthentication yes\n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Error("expected false, got true")
		}
	})

	t.Run("ignores commented lines", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("# PasswordAuthentication no\nPasswordAuthentication yes\n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Error("expected false, got true")
		}
	})

	t.Run("returns false when directive not found", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("SomeOtherSetting yes\n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Error("expected false, got true")
		}
	})

	t.Run("case insensitive key match", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("passwordauthentication no\n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Error("expected true, got false")
		}
	})

	t.Run("handles extra whitespace", func(t *testing.T) {
		os.WriteFile(sshConfig, []byte("  PasswordAuthentication   no  \n"), 0644)

		got, err := sshConfigHasFrom(sshConfig, "PasswordAuthentication", "no")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Error("expected true, got false")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := sshConfigHasFrom("/nonexistent/sshd_config", "PasswordAuthentication", "no")

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestParseUnexpectedPorts(t *testing.T) {
	t.Run("no unexpected ports", func(t *testing.T) {
		output := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
`

		got := parseUnexpectedPorts(output)

		if len(got) != 0 {
			t.Errorf("expected no unexpected ports, got %v", got)
		}
	})

	t.Run("detects unexpected port", func(t *testing.T) {
		output := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
8080/tcp                   ALLOW       Anywhere
`

		got := parseUnexpectedPorts(output)

		if len(got) != 1 {
			t.Fatalf("expected 1 unexpected port, got %d", len(got))
		}
		if got[0] != "8080/tcp" {
			t.Errorf("expected 8080/tcp, got %s", got[0])
		}
	})

	t.Run("detects multiple unexpected ports", func(t *testing.T) {
		output := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
3000/tcp                   ALLOW       Anywhere
9090/tcp                   ALLOW       Anywhere
`

		got := parseUnexpectedPorts(output)

		if len(got) != 2 {
			t.Fatalf("expected 2 unexpected ports, got %d", len(got))
		}
	})

	t.Run("handles ports without protocol suffix", func(t *testing.T) {
		output := `Status: active

To                         Action      From
--                         ------      ----
22                         ALLOW       Anywhere
80                         ALLOW       Anywhere
443                        ALLOW       Anywhere
`

		got := parseUnexpectedPorts(output)

		if len(got) != 0 {
			t.Errorf("expected no unexpected ports, got %v", got)
		}
	})

	t.Run("handles udp ports", func(t *testing.T) {
		output := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
5000/udp                   ALLOW       Anywhere
`

		got := parseUnexpectedPorts(output)

		if len(got) != 1 {
			t.Fatalf("expected 1 unexpected port, got %d", len(got))
		}
		if got[0] != "5000/udp" {
			t.Errorf("expected 5000/udp, got %s", got[0])
		}
	})
}

func TestIsCloudflareIP(t *testing.T) {
	t.Run("recognizes Cloudflare IP", func(t *testing.T) {
		ip := net.ParseIP("104.16.1.1")

		if !isCloudflareIP(ip) {
			t.Error("expected 104.16.1.1 to be recognized as Cloudflare")
		}
	})

	t.Run("recognizes another Cloudflare range", func(t *testing.T) {
		ip := net.ParseIP("173.245.48.10")

		if !isCloudflareIP(ip) {
			t.Error("expected 173.245.48.10 to be recognized as Cloudflare")
		}
	})

	t.Run("rejects non-Cloudflare IP", func(t *testing.T) {
		ip := net.ParseIP("8.8.8.8")

		if isCloudflareIP(ip) {
			t.Error("expected 8.8.8.8 to NOT be recognized as Cloudflare")
		}
	})

	t.Run("rejects private IP", func(t *testing.T) {
		ip := net.ParseIP("192.168.1.1")

		if isCloudflareIP(ip) {
			t.Error("expected 192.168.1.1 to NOT be recognized as Cloudflare")
		}
	})
}

func TestCheckResultCounting(t *testing.T) {
	t.Run("all required pass returns no error", func(t *testing.T) {
		results := []CheckResult{
			{Name: "check1", Passed: true},
			{Name: "check2", Passed: true},
			{Name: "optional1", Passed: false, Optional: true},
		}

		required, passed := countResults(results)

		if required != 2 {
			t.Errorf("expected 2 required, got %d", required)
		}
		if passed != 2 {
			t.Errorf("expected 2 passed, got %d", passed)
		}
	})

	t.Run("failed required counted correctly", func(t *testing.T) {
		results := []CheckResult{
			{Name: "check1", Passed: true},
			{Name: "check2", Passed: false},
			{Name: "optional1", Passed: true, Optional: true},
		}

		required, passed := countResults(results)

		if required != 2 {
			t.Errorf("expected 2 required, got %d", required)
		}
		if passed != 1 {
			t.Errorf("expected 1 passed, got %d", passed)
		}
	})

	t.Run("optional checks not counted as required", func(t *testing.T) {
		results := []CheckResult{
			{Name: "optional1", Passed: false, Optional: true},
			{Name: "optional2", Passed: false, Optional: true},
		}

		required, passed := countResults(results)

		if required != 0 {
			t.Errorf("expected 0 required, got %d", required)
		}
		if passed != 0 {
			t.Errorf("expected 0 passed, got %d", passed)
		}
	})
}
