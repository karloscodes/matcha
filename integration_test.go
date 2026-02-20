package matcha_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karloscodes/matcha/testrunner"
)

// TestSingleAppInVM runs a single app install and verifies the new YAML config + data layout.
func TestSingleAppInVM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping VM integration test in short mode")
	}
	if _, err := exec.LookPath("orb"); err != nil {
		t.Skip("orb not installed, skipping VM test")
	}

	binaryPath := buildTestBinary(t)

	config := testrunner.DefaultConfig()
	config.BinaryPath = binaryPath
	config.BinaryName = "testapp"
	config.VMName = "matcha-single-app"
	config.Args = []string{"install"}
	config.StdinInput = "localhost\ny\n"
	config.EnvVars = map[string]string{"ENV": "test"}
	config.Debug = true
	config.SkipCleanup = true

	t.Cleanup(func() {
		if os.Getenv("KEEP_VM") != "1" {
			exec.Command("orb", "delete", config.VMName, "-f").Run()
		}
	})

	runner := testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
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

	// Verify YAML config was created
	out, err = runner.RunCommand("cat /etc/matcha/config.yml", true)
	if err != nil {
		t.Fatalf("Failed to read config.yml: %v", err)
	}
	if !strings.Contains(out, "testapp") {
		t.Errorf("config.yml missing testapp entry: %s", out)
	}
	if !strings.Contains(out, "domain: localhost") {
		t.Errorf("config.yml missing domain: %s", out)
	}
	if !strings.Contains(out, "PRIVATE_KEY") {
		t.Errorf("config.yml missing PRIVATE_KEY: %s", out)
	}

	// Verify proxy data directory
	out, err = runner.RunCommand("ls -la /var/matcha/proxy/", true)
	if err != nil {
		t.Errorf("Proxy data dir /var/matcha/proxy/ not created: %v", err)
	}

	// Verify NO old layout exists
	out, _ = runner.RunCommand("ls /etc/matcha/apps/ 2>&1", true)
	if !strings.Contains(out, "No such file") && !strings.Contains(out, "cannot access") {
		// If the directory exists, it should be empty
		out2, _ := runner.RunCommand("ls /etc/matcha/apps/testapp/app.json 2>&1", true)
		if !strings.Contains(out2, "No such file") && !strings.Contains(out2, "cannot access") {
			t.Errorf("Old app.json should not exist: %s", out2)
		}
	}

	// Check health endpoint
	if !runner.CheckHealth("http://localhost/", 10) {
		t.Error("Health check failed after 10 attempts")
	}

	// Test update (reuse VM)
	config.Args = []string{"update"}
	config.StdinInput = ""
	config.ReuseVM = true

	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Update failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Verify still running after update
	out, _ = runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if !strings.Contains(out, "testapp") {
		t.Errorf("App container not running after update")
	}
}

// TestMultiAppInVM tests adding and deploying two apps via the matcha CLI.
func TestMultiAppInVM(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping VM integration test in short mode")
	}
	if _, err := exec.LookPath("orb"); err != nil {
		t.Skip("orb not installed, skipping VM test")
	}

	matchaBin := buildMatchaCLI(t)

	config := testrunner.DefaultConfig()
	config.BinaryPath = matchaBin
	config.BinaryName = "matcha"
	config.VMName = "matcha-multi-app"
	config.EnvVars = map[string]string{"ENV": "test"}
	config.Debug = true
	config.SkipCleanup = true

	t.Cleanup(func() {
		if os.Getenv("KEEP_VM") != "1" {
			exec.Command("orb", "delete", config.VMName, "-f").Run()
		}
	})

	// Step 1: Setup shared infrastructure
	config.Args = []string{"setup"}
	runner := testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Setup failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Verify proxy is running
	out, _ := runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if !strings.Contains(out, "matcha-proxy") {
		t.Fatalf("Proxy not running after setup. Docker ps: %s", out)
	}

	// Step 2: Add first app (nginx on port 80)
	config.Args = []string{"add", "web", "--image", "nginx:alpine", "--domain", "web.localhost", "--port", "80", "--health-path", "/"}
	config.ReuseVM = true
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Add web failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Step 3: Add second app (httpd on port 80)
	config.Args = []string{"add", "api", "--image", "httpd:alpine", "--domain", "api.localhost", "--port", "80", "--health-path", "/"}
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Add api failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Step 4: Verify config.yml has both apps
	out, err := runner.RunCommand("cat /etc/matcha/config.yml", true)
	if err != nil {
		t.Fatalf("Failed to read config.yml: %v", err)
	}
	if !strings.Contains(out, "web:") {
		t.Errorf("config.yml missing web app: %s", out)
	}
	if !strings.Contains(out, "api:") {
		t.Errorf("config.yml missing api app: %s", out)
	}
	if !strings.Contains(out, "web.localhost") {
		t.Errorf("config.yml missing web domain: %s", out)
	}
	if !strings.Contains(out, "api.localhost") {
		t.Errorf("config.yml missing api domain: %s", out)
	}

	// Step 5: List apps
	config.Args = []string{"list"}
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("List failed: %v", err)
	}
	listOut := runner.Stdout()
	if !strings.Contains(listOut, "web") || !strings.Contains(listOut, "api") {
		t.Errorf("List should show both apps: %s", listOut)
	}

	// Step 6: Deploy first app
	config.Args = []string{"deploy", "web"}
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Deploy web failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Step 7: Deploy second app
	config.Args = []string{"deploy", "api"}
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Deploy api failed: %v\nStdout: %s\nStderr: %s",
			err, runner.Stdout(), runner.Stderr())
	}

	// Step 8: Verify both containers running
	out, _ = runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if !strings.Contains(out, "web") {
		t.Errorf("Web container not running. Docker ps: %s", out)
	}
	if !strings.Contains(out, "api") {
		t.Errorf("API container not running. Docker ps: %s", out)
	}

	// Step 9: Verify proxy data directory
	out, err = runner.RunCommand("ls /var/matcha/proxy/ 2>&1", true)
	if err != nil {
		t.Errorf("Proxy data dir missing: %s", out)
	}

	// Step 10: Remove one app
	config.Args = []string{"remove", "api"}
	runner = testrunner.NewTestRunner(config)
	if err := runner.Run(); err != nil {
		t.Fatalf("Remove api failed: %v", err)
	}

	// Verify api removed from config
	out, _ = runner.RunCommand("cat /etc/matcha/config.yml", true)
	if strings.Contains(out, "api:") {
		t.Errorf("api should be removed from config.yml: %s", out)
	}
	if !strings.Contains(out, "web:") {
		t.Errorf("web should still be in config.yml: %s", out)
	}

	// Verify api container stopped
	out, _ = runner.RunCommand("docker ps --format '{{.Names}}'", true)
	if strings.Contains(out, "api") {
		t.Errorf("API container should be stopped after remove")
	}
	if !strings.Contains(out, "web") {
		t.Errorf("Web container should still be running")
	}
}

// buildTestBinary builds a test binary that uses the matcha library directly.
func buildTestBinary(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")
	binaryPath := filepath.Join(tmpDir, "testapp")

	mainCode := `package main

import (
	"os"
	"github.com/karloscodes/matcha"
)

func main() {
	m := matcha.New(matcha.Config{
		Name:       "testapp",
		AppImage:   "nginx:alpine",
		HealthPath: "/",
		AppPort:    80,
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

	writeGoMod(t, tmpDir)
	return goBuild(t, tmpDir, binaryPath)
}

// buildMatchaCLI builds the matcha CLI binary for VM testing.
func buildMatchaCLI(t *testing.T) string {
	t.Helper()

	projectRoot := findProjectRoot(t)
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "matcha")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/matcha")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build matcha CLI: %v\n%s", err, out)
	}

	return binaryPath
}

func writeGoMod(t *testing.T, dir string) {
	t.Helper()

	modFile := filepath.Join(dir, "go.mod")
	modContent := `module testapp

go 1.21

require github.com/karloscodes/matcha v0.0.0

replace github.com/karloscodes/matcha => ` + findProjectRoot(t) + `
`
	if err := os.WriteFile(modFile, []byte(modContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v\n%s", err, out)
	}
}

func goBuild(t *testing.T, dir, binaryPath string) string {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, out)
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
