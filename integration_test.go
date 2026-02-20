package matcha_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karloscodes/matcha/testrunner"
)

// TestInstallInVM runs the full install process in a Multipass VM.
// This test is skipped in short mode and requires orb to be installed.
func TestInstallInVM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping VM integration test in short mode")
	}

	// Check orb is available
	if _, err := exec.LookPath("orb"); err != nil {
		t.Skip("orb not installed, skipping VM test")
	}

	// Build a test binary
	binaryPath := buildTestBinary(t)

	config := testrunner.DefaultConfig()
	config.BinaryPath = binaryPath
	config.BinaryName = "testapp"
	config.Args = []string{"install"}
	config.StdinInput = "test.localhost\ny\n" // domain + confirm
	config.EnvVars = map[string]string{
		"ENV": "test",
	}
	config.Debug = true

	runner := testrunner.NewTestRunner(config)

	err := runner.Run()
	if err != nil {
		t.Fatalf("Install failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Verify containers are running
	out, err := runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if err != nil {
		t.Fatalf("Failed to check containers: %v", err)
	}

	if !strings.Contains(out, "testapp") {
		t.Errorf("App container not running. Docker ps output: %s", out)
	}

	if !strings.Contains(out, "matcha-proxy") {
		t.Errorf("Proxy container not running. Docker ps output: %s", out)
	}

	// Check health endpoint
	if !runner.CheckHealth("http://localhost/", 10) {
		t.Error("Health check failed after 10 attempts")
	}
}

// TestUpdateInVM tests the update process in a VM.
func TestUpdateInVM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping VM integration test in short mode")
	}

	if _, err := exec.LookPath("orb"); err != nil {
		t.Skip("orb not installed, skipping VM test")
	}

	// First install
	binaryPath := buildTestBinary(t)

	config := testrunner.DefaultConfig()
	config.BinaryPath = binaryPath
	config.BinaryName = "testapp"
	config.VMName = "matcha-update-test"
	config.EnvVars = map[string]string{"ENV": "test"}
	config.Debug = true

	// Clean up VM at end of test
	t.Cleanup(func() {
		if os.Getenv("KEEP_VM") != "1" {
			exec.Command("orb", "delete", "matcha-update-test", "-f").Run()
		}
	})

	// Install first (keep VM for update step)
	config.Args = []string{"install"}
	config.StdinInput = "test.localhost\ny\n"
	config.SkipCleanup = true

	runner := testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Now update (reuse the VM, keep it for verification)
	config.Args = []string{"update"}
	config.StdinInput = ""
	config.ReuseVM = true

	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify still running
	out, _ := runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if !strings.Contains(out, "testapp") {
		t.Errorf("App container not running after update")
	}
}

// buildTestBinary builds a test binary that uses matcha
func buildTestBinary(t *testing.T) string {
	t.Helper()

	// Create temp directory for test binary
	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")
	binaryPath := filepath.Join(tmpDir, "testapp")

	// Write a simple test app that uses matcha
	mainCode := `package main

import (
	"os"
	"github.com/karloscodes/matcha"
)

func main() {
	m := matcha.New(matcha.Config{
		Name:       "testapp",
		AppImage:   "nginx:alpine", // Use nginx for testing
		HealthPath: "/",            // nginx serves / but not /up
		AppPort:    80,             // nginx listens on 80
	})

	if len(os.Args) < 2 {
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "install":
		err = m.Install()
	case "update":
		err = m.Update()
	case "status":
		err = m.Status()
	}

	if err != nil {
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(mainFile, []byte(mainCode), 0644); err != nil {
		t.Fatalf("Failed to write test main.go: %v", err)
	}

	// Initialize go.mod
	modFile := filepath.Join(tmpDir, "go.mod")
	modContent := `module testapp

go 1.21

require github.com/karloscodes/matcha v0.0.0

replace github.com/karloscodes/matcha => ` + findProjectRoot(t) + `
`
	if err := os.WriteFile(modFile, []byte(modContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v\n%s", err, out)
	}

	// Build for Linux (VMs run Linux)
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build test binary: %v\n%s", err, out)
	}

	return binaryPath
}

func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root")
		}
		dir = parent
	}
}
