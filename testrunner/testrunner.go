// Package testrunner provides utilities for running integration tests
// in isolated environments using Multipass VMs.
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

// runInVM provisions a Multipass VM and runs the binary inside
func (r *TestRunner) runInVM() error {
	r.logf("Running in Multipass VM")

	// Check multipass is available
	if _, err := exec.LookPath("multipass"); err != nil {
		return fmt.Errorf("multipass not found: %w", err)
	}

	// Cleanup existing VM
	r.logf("Cleaning up existing VM: %s", r.Config.VMName)
	exec.Command("multipass", "delete", r.Config.VMName, "--purge").Run()

	// Prepare SSH key for cloud-init
	sshKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub")
	if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
		privateKeyPath := strings.TrimSuffix(sshKeyPath, ".pub")
		r.logf("Generating SSH key pair...")
		if err := exec.Command("ssh-keygen", "-t", "ed25519", "-f", privateKeyPath, "-N", "").Run(); err != nil {
			return fmt.Errorf("failed to generate SSH key: %w", err)
		}
	}

	pubKey, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}

	cloudInit := fmt.Sprintf("#cloud-config\nusers:\n  - default\n  - name: ubuntu\n    ssh_authorized_keys:\n      - %s\n", strings.TrimSpace(string(pubKey)))
	cloudInitPath := filepath.Join(os.TempDir(), "matcha-cloud-init.yaml")
	if err := os.WriteFile(cloudInitPath, []byte(cloudInit), 0644); err != nil {
		return fmt.Errorf("failed to write cloud-init: %w", err)
	}

	// Launch VM
	r.logf("Launching VM: %s", r.Config.VMName)
	launchCmd := exec.Command("multipass", "launch", "22.04",
		"--name", r.Config.VMName,
		"--cpus", "2",
		"--memory", "2G",
		"--disk", "10G",
		"--cloud-init", cloudInitPath,
	)
	launchCmd.Stdout = r.Logger
	launchCmd.Stderr = r.Logger

	if err := launchCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch VM: %w", err)
	}

	// Wait for VM to be ready
	r.logf("Waiting for VM to be ready")
	for i := 0; i < 60; i++ {
		out, _ := exec.Command("multipass", "info", r.Config.VMName).CombinedOutput()
		if strings.Contains(string(out), "Running") {
			r.logf("VM ready after %d seconds", i+1)
			break
		}
		time.Sleep(time.Second)
	}

	// Set ENV=test
	exec.Command("multipass", "exec", r.Config.VMName, "--",
		"sudo", "sh", "-c", "echo 'export ENV=test' >> /etc/environment").Run()

	// Copy binary to VM
	r.logf("Copying binary to VM")
	binaryName := r.Config.BinaryName
	if binaryName == "" {
		binaryName = filepath.Base(r.Config.BinaryPath)
	}

	copyCmd := exec.Command("multipass", "transfer", r.Config.BinaryPath,
		fmt.Sprintf("%s:/home/ubuntu/%s", r.Config.VMName, binaryName))
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable and move to /usr/local/bin
	exec.Command("multipass", "exec", r.Config.VMName, "--",
		"chmod", "+x", "/home/ubuntu/"+binaryName).Run()
	exec.Command("multipass", "exec", r.Config.VMName, "--",
		"sudo", "mv", "/home/ubuntu/"+binaryName, "/usr/local/bin/"+binaryName).Run()

	// Set environment variables
	for k, v := range r.Config.EnvVars {
		exec.Command("multipass", "exec", r.Config.VMName, "--",
			"sudo", "sh", "-c", fmt.Sprintf("echo 'export %s=%s' >> /etc/environment", k, v)).Run()
	}

	// Run the command
	cmdParts := []string{"exec", r.Config.VMName, "--", "sudo", "/usr/local/bin/" + binaryName}
	cmdParts = append(cmdParts, r.Config.Args...)

	cmd := exec.Command("multipass", cmdParts...)
	cmd.Stdin = strings.NewReader(r.Config.StdinInput)
	cmd.Stdout = io.MultiWriter(&r.stdout, r.Logger)
	cmd.Stderr = io.MultiWriter(&r.stderr, r.Logger)

	err = r.runWithTimeout(cmd)

	// Cleanup unless KEEP_VM is set
	if os.Getenv("KEEP_VM") != "1" {
		r.logf("Cleaning up VM")
		exec.Command("multipass", "delete", r.Config.VMName, "--purge").Run()
	} else {
		r.logf("Keeping VM for inspection: %s", r.Config.VMName)
	}

	return err
}

// GetVMIP returns the IP address of the VM
func (r *TestRunner) GetVMIP() (string, error) {
	out, err := exec.Command("multipass", "info", r.Config.VMName).Output()
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "IPv4") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("IP not found")
}

// RunCommand runs a command in the VM
func (r *TestRunner) RunCommand(command string, sudo bool) (string, error) {
	args := []string{"exec", r.Config.VMName, "--"}
	if sudo {
		args = append(args, "sudo")
	}
	args = append(args, "sh", "-c", command)

	out, err := exec.Command("multipass", args...).CombinedOutput()
	return string(out), err
}

// CheckHealth checks if a URL returns 200
func (r *TestRunner) CheckHealth(url string, attempts int) bool {
	for i := 0; i < attempts; i++ {
		var out []byte
		var err error

		if r.env == LocalEnvironment {
			out, err = exec.Command("multipass", "exec", r.Config.VMName, "--",
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
