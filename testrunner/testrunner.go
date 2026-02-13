// Package testrunner provides utilities for running integration tests
// in isolated environments using OrbStack VMs.
package testrunner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Environment represents where tests are running
type Environment string

const (
	LocalEnvironment Environment = "local"
	CIEnvironment    Environment = "ci"
)

// Config holds configuration for the test runner
type Config struct {
	BinaryPath string
	BinaryName string
	Timeout    time.Duration
	StdinInput string
	EnvVars    map[string]string
	Args       []string
	VMName     string
	Debug      bool
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		Timeout: 10 * time.Minute,
		EnvVars: make(map[string]string),
		VMName:  "matcha-test-vm",
		Debug:   os.Getenv("DEBUG") == "1",
	}
}

// TestRunner handles running commands in different environments
type TestRunner struct {
	Config Config
	env    Environment
	stdout bytes.Buffer
	stderr bytes.Buffer
	Logger io.Writer
}

// NewTestRunner creates a new TestRunner
func NewTestRunner(config Config) *TestRunner {
	env := LocalEnvironment
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		env = CIEnvironment
	}

	return &TestRunner{
		Config: config,
		env:    env,
		Logger: os.Stdout,
	}
}

// Run executes the command in the appropriate environment
func (r *TestRunner) Run() error {
	r.logf("Starting test in %s environment", r.env)

	if r.env == CIEnvironment {
		return r.runInCI()
	}
	return r.runInVM()
}

// runInCI runs the command directly in CI
func (r *TestRunner) runInCI() error {
	r.logf("Running directly in CI environment")

	cmd := exec.Command(r.Config.BinaryPath, r.Config.Args...)
	cmd.Stdin = strings.NewReader(r.Config.StdinInput)
	cmd.Stdout = io.MultiWriter(&r.stdout, r.Logger)
	cmd.Stderr = io.MultiWriter(&r.stderr, r.Logger)

	cmd.Env = os.Environ()
	for k, v := range r.Config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	return r.runWithTimeout(cmd)
}

// runInVM provisions an OrbStack VM and runs the binary inside
func (r *TestRunner) runInVM() error {
	r.logf("Running in OrbStack VM")

	// Check orb is available
	if _, err := exec.LookPath("orb"); err != nil {
		return fmt.Errorf("orb not found: %w", err)
	}

	// Cleanup existing VM
	r.logf("Cleaning up existing VM: %s", r.Config.VMName)
	exec.Command("orb", "delete", r.Config.VMName, "-f").Run()

	// Launch VM
	r.logf("Launching VM: %s", r.Config.VMName)
	launchCmd := exec.Command("orb", "create", "ubuntu:22.04", r.Config.VMName)
	launchCmd.Stdout = r.Logger
	launchCmd.Stderr = r.Logger

	if err := launchCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch VM: %w", err)
	}

	// Wait for VM to be ready
	r.logf("Waiting for VM to be ready")
	for i := 0; i < 30; i++ {
		out, _ := exec.Command("orb", "list").CombinedOutput()
		if strings.Contains(string(out), r.Config.VMName) && strings.Contains(string(out), "running") {
			r.logf("VM ready after %d seconds", i+1)
			break
		}
		time.Sleep(time.Second)
	}

	// Copy binary to VM
	r.logf("Copying binary to VM")
	binaryName := r.Config.BinaryName
	if binaryName == "" {
		binaryName = filepath.Base(r.Config.BinaryPath)
	}

	// OrbStack allows running commands with file paths directly from host
	// Copy by reading and writing through stdin
	binaryData, err := os.ReadFile(r.Config.BinaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	copyCmd := exec.Command("orb", "-m", r.Config.VMName, "-u", "root",
		"sh", "-c", fmt.Sprintf("cat > /usr/local/bin/%s && chmod +x /usr/local/bin/%s", binaryName, binaryName))
	copyCmd.Stdin = bytes.NewReader(binaryData)
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Set environment variables
	for k, v := range r.Config.EnvVars {
		exec.Command("orb", "-m", r.Config.VMName, "-u", "root",
			"sh", "-c", fmt.Sprintf("echo 'export %s=%s' >> /etc/environment", k, v)).Run()
	}

	// Set ENV=test
	exec.Command("orb", "-m", r.Config.VMName, "-u", "root",
		"sh", "-c", "echo 'export ENV=test' >> /etc/environment").Run()

	// Build the command to run
	cmdStr := fmt.Sprintf("/usr/local/bin/%s %s", binaryName, strings.Join(r.Config.Args, " "))

	// Run the command
	cmd := exec.Command("orb", "-m", r.Config.VMName, "-u", "root", "sh", "-c", cmdStr)
	cmd.Stdin = strings.NewReader(r.Config.StdinInput)
	cmd.Stdout = io.MultiWriter(&r.stdout, r.Logger)
	cmd.Stderr = io.MultiWriter(&r.stderr, r.Logger)

	err = r.runWithTimeout(cmd)

	// Cleanup unless KEEP_VM is set
	if os.Getenv("KEEP_VM") != "1" {
		r.logf("Cleaning up VM")
		exec.Command("orb", "delete", r.Config.VMName, "-f").Run()
	} else {
		r.logf("Keeping VM for inspection: %s", r.Config.VMName)
	}

	return err
}

// GetVMIP returns the IP address of the VM
func (r *TestRunner) GetVMIP() (string, error) {
	out, err := exec.Command("orb", "-m", r.Config.VMName, "hostname", "-I").Output()
	if err != nil {
		return "", err
	}

	parts := strings.Fields(string(out))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("IP not found")
}

// RunCommand runs a command in the VM
func (r *TestRunner) RunCommand(command string, sudo bool) (string, error) {
	var cmd *exec.Cmd
	if sudo {
		cmd = exec.Command("orb", "-m", r.Config.VMName, "-u", "root", "sh", "-c", command)
	} else {
		cmd = exec.Command("orb", "-m", r.Config.VMName, "sh", "-c", command)
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CheckHealth checks if a URL returns 200
func (r *TestRunner) CheckHealth(url string, attempts int) bool {
	for i := 0; i < attempts; i++ {
		var out []byte
		var err error

		if r.env == LocalEnvironment {
			out, err = exec.Command("orb", "-m", r.Config.VMName,
				"curl", "-sf", "-o", "/dev/null", "-w", "%{http_code}", url).CombinedOutput()
		} else {
			out, err = exec.Command("curl", "-sf", "-o", "/dev/null", "-w", "%{http_code}", url).CombinedOutput()
		}

		if err == nil && strings.TrimSpace(string(out)) == "200" {
			return true
		}
		time.Sleep(5 * time.Second)
	}
	return false
}

func (r *TestRunner) runWithTimeout(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(r.Config.Timeout):
		cmd.Process.Kill()
		return fmt.Errorf("timeout after %v", r.Config.Timeout)
	}
}

// Stdout returns captured stdout
func (r *TestRunner) Stdout() string { return r.stdout.String() }

// Stderr returns captured stderr
func (r *TestRunner) Stderr() string { return r.stderr.String() }

func (r *TestRunner) logf(format string, args ...interface{}) {
	if r.Config.Debug {
		fmt.Fprintf(r.Logger, "[TestRunner] "+format+"\n", args...)
	}
}
