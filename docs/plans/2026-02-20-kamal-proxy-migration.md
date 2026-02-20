# Kamal Proxy Migration Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace Caddy with kamal-proxy in Matcha for zero-downtime single-app deployments.

**Architecture:** kamal-proxy runs as a Docker container on ports 80/443. App containers join the same Docker network. Deploys happen via `docker exec` into the kamal-proxy container. kamal-proxy handles health checks, request buffering, TLS, and connection draining — eliminating Matcha's blue-green slot logic entirely.

**Tech Stack:** Go stdlib, Docker CLI (exec.Command), kamal-proxy (`basecamp/kamal-proxy:latest`)

---

## Key Design Decisions

**kamal-proxy runs in Docker** (not as a host binary) — same pattern as current Caddy setup. Shared Docker network lets kamal-proxy reach app containers by name.

**Blue-green logic removed.** kamal-proxy's `deploy` command is atomic: health check new target → buffer requests → switch → drain old. One container at a time, always zero-downtime.

**Container naming simplifies.** No more slot 1/2. Just `{name}-app` always. kamal-proxy tracks the target internally.

**kamal-proxy CLI via `docker exec`.** All proxy commands run as:
```
docker exec {name}-proxy kamal-proxy deploy {name} --target {name}-app:{port} --host {domain} --tls
```

## What Changes

| File | Action | Summary |
|------|--------|---------|
| `matcha.go` | Modify | Remove `BlueGreen` from Config, remove `CaddyContainerName()`, add `ProxyContainerName()`, simplify `AppContainerName()` |
| `caddy.go` | Delete | Replaced entirely by proxy.go |
| `proxy.go` | Create | kamal-proxy deploy/remove via docker exec |
| `deploy.go` | Modify | Single deploy path (no blue-green/single split) |
| `docker.go` | Modify | Replace `deployCaddy`/`reloadCaddy` with `deployProxy`, remove `getActiveSlot`, remove `waitForHealth` |
| `config.go` | Modify | Remove `CaddyImage` from envData, add `ProxyImage`, update `saveEnv`/`readEnv` |
| `status.go` | Modify | Show proxy instead of Caddy |
| `requirements.go` | No change | Still checks ports 80/443 |
| `caddy_test.go` | Delete | Replaced by proxy_test.go |
| `proxy_test.go` | Create | Test proxy command building |
| `matcha_test.go` | Modify | Remove blue-green tests, remove Caddy references |
| `integration_test.go` | Modify | Verify proxy container instead of Caddy |

---

### Task 1: Create branch and remove Caddy references from Config

**Files:**
- Modify: `matcha.go`
- Modify: `config.go`

**Step 1: Create branch**

```bash
cd ~/code/matcha
git checkout -b kamal-proxy
```

**Step 2: Update Config struct in matcha.go**

Remove `CaddyImage` and `BlueGreen` fields. Add `ProxyImage`. Remove `CaddyContainerName()`. Add `ProxyContainerName()`. Simplify `AppContainerName()` to always return `{name}-app` (no slots).

```go
type Config struct {
	Name     string
	AppImage string

	InstallDir string
	BinaryPath string
	ProxyImage string // default: basecamp/kamal-proxy:latest
	HealthPath string // default: /up (kamal-proxy default)
	AppPort    int    // default: 8080

	CronUpdates    bool
	Backups        bool
	ManagerRepo    string
	ManagerVersion string
}
```

In `New()`:
- Default `ProxyImage` to `"basecamp/kamal-proxy:latest"`
- Default `HealthPath` to `"/up"` (kamal-proxy default instead of `/_health`)
- Remove `CaddyImage` default

Replace `CaddyContainerName()` with:
```go
func (m *Matcha) ProxyContainerName() string {
	return m.config.Name + "-proxy"
}
```

Simplify `AppContainerName()`:
```go
func (m *Matcha) AppContainerName() string {
	return m.config.Name + "-app"
}
```

**Step 3: Update envData in config.go**

Replace `CaddyImage` with `ProxyImage` in envData struct, `readEnv()`, and `saveEnv()`.

Remove Caddyfile generation from `setupConfig()`. Remove the `caddy` subdirectories from directory creation.

```go
// In setupConfig(), directories become:
for _, dir := range []string{"storage", "logs", "storage/backups"} {
```

**Step 4: Run tests to see what breaks**

```bash
cd ~/code/matcha && go test ./... -short -count=1
```

Expected: Compilation errors in tests referencing `BlueGreen`, `CaddyContainerName`, `AppContainerName(slot)`. That's correct — we fix those in Task 3.

**Step 5: Commit**

```bash
git add matcha.go config.go
git commit -m "Remove Caddy config, add kamal-proxy config"
```

---

### Task 2: Create proxy.go (replace caddy.go)

**Files:**
- Create: `proxy.go`
- Delete: `caddy.go`

**Step 1: Write proxy_test.go with the key test**

```go
package matcha

import "testing"

func TestProxyDeployArgs(t *testing.T) {
	m := New(Config{
		Name:     "testapp",
		AppImage: "test:latest",
		AppPort:  8080,
	})

	args := m.proxyDeployArgs("app.example.com")

	// Should be: docker exec testapp-proxy kamal-proxy deploy testapp
	//   --target testapp-app:8080 --host app.example.com --tls
	//   --health-check-path /up
	expected := []string{
		"exec", "testapp-proxy", "kamal-proxy", "deploy", "testapp",
		"--target", "testapp-app:8080",
		"--host", "app.example.com",
		"--tls",
		"--health-check-path", "/up",
	}

	if len(args) != len(expected) {
		t.Fatalf("got %d args, want %d: %v", len(args), len(expected), args)
	}

	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestProxyDeployArgsLocalhost(t *testing.T) {
	m := New(Config{
		Name:     "testapp",
		AppImage: "test:latest",
		AppPort:  3000,
	})

	args := m.proxyDeployArgs("localhost")

	// No --tls for localhost
	for _, arg := range args {
		if arg == "--tls" {
			t.Error("should not include --tls for localhost")
		}
	}

	// Should have correct port
	found := false
	for i, arg := range args {
		if arg == "--target" && i+1 < len(args) && args[i+1] == "testapp-app:3000" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing --target testapp-app:3000 in %v", args)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd ~/code/matcha && go test -run TestProxyDeploy -short -count=1
```

Expected: FAIL — `proxyDeployArgs` not defined.

**Step 3: Create proxy.go**

```go
package matcha

import "fmt"

// proxyDeployArgs builds the docker exec args for kamal-proxy deploy.
func (m *Matcha) proxyDeployArgs(domain string) []string {
	serviceName := m.config.Name
	proxyContainer := m.ProxyContainerName()
	target := fmt.Sprintf("%s:%d", m.AppContainerName(), m.config.AppPort)

	args := []string{
		"exec", proxyContainer, "kamal-proxy", "deploy", serviceName,
		"--target", target,
		"--host", domain,
	}

	if !isLocalhost(domain) {
		args = append(args, "--tls")
	}

	args = append(args, "--health-check-path", m.config.HealthPath)

	return args
}

// deployToProxy registers the app container with kamal-proxy.
func (m *Matcha) deployToProxy(domain string) error {
	args := m.proxyDeployArgs(domain)
	_, err := m.runDocker(args...)
	if err != nil {
		return fmt.Errorf("kamal-proxy deploy failed: %w", err)
	}
	return nil
}

// removeFromProxy removes the service from kamal-proxy.
func (m *Matcha) removeFromProxy() error {
	proxyContainer := m.ProxyContainerName()
	_, err := m.runDocker("exec", proxyContainer, "kamal-proxy", "remove", m.config.Name)
	return err
}
```

**Step 4: Delete caddy.go**

```bash
rm ~/code/matcha/caddy.go
```

**Step 5: Run tests**

```bash
cd ~/code/matcha && go test -run TestProxyDeploy -short -count=1
```

Expected: PASS for the proxy tests.

**Step 6: Commit**

```bash
git add proxy.go proxy_test.go
git rm caddy.go
git commit -m "Add kamal-proxy integration, remove Caddy"
```

---

### Task 3: Rewrite deploy.go and docker.go

**Files:**
- Modify: `deploy.go`
- Modify: `docker.go`

**Step 1: Rewrite deploy.go**

The entire deploy flow simplifies to:

```go
package matcha

import "fmt"

// deploy handles the deployment logic.
func (m *Matcha) deploy() error {
	data, err := m.readEnv()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := m.createNetwork(); err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Ensure proxy is running
	if !m.isRunning(m.ProxyContainerName()) {
		if err := m.deployProxy(); err != nil {
			return fmt.Errorf("failed to deploy proxy: %w", err)
		}
	}

	// Deploy app container
	containerName := m.AppContainerName()
	if err := m.deployApp(containerName, data); err != nil {
		return fmt.Errorf("failed to deploy app: %w", err)
	}

	// Register with kamal-proxy (handles health check + zero-downtime switch)
	if err := m.deployToProxy(data.Domain); err != nil {
		return fmt.Errorf("failed to register with proxy: %w", err)
	}

	return nil
}
```

No more `deployBlueGreen()`, no more `deploySingle()`. One path.

**Step 2: Update docker.go**

Remove: `deployCaddy()`, `reloadCaddy()`, `getActiveSlot()`, `waitForHealth()`.

Add `deployProxy()`:

```go
// deployProxy starts the kamal-proxy container.
func (m *Matcha) deployProxy() error {
	name := m.ProxyContainerName()
	m.stopAndRemove(name)

	args := []string{
		"run", "-d",
		"--name", name,
		"--network", m.NetworkName(),
		"-p", "80:80",
		"-p", "443:443",
		"-p", "443:443/udp",
		"-v", m.config.InstallDir + "/kamal-proxy-data:/home/kamal-proxy/.config/kamal-proxy",
		"--memory=128m",
		"--restart", "unless-stopped",
		m.config.ProxyImage,
	}

	_, err := m.runDocker(args...)
	return err
}
```

Update `pullImages()` to pull proxy image instead of Caddy:

```go
func (m *Matcha) pullImages() error {
	images := []string{m.config.AppImage, m.config.ProxyImage}
	// ... rest stays the same
}
```

Update `deployApp()` — remove `--pull always` flag (we pull explicitly), and the signature changes since `AppContainerName()` no longer takes a slot:

```go
func (m *Matcha) deployApp(name string, data *envData) error {
	m.stopAndRemove(name)

	prefix := m.EnvPrefix()
	args := []string{
		"run", "-d",
		"--name", name,
		"--network", m.NetworkName(),
		"-v", m.config.InstallDir + "/storage:/app/storage",
		"-v", m.config.InstallDir + "/logs:/app/logs",
		"-e", fmt.Sprintf("%s_DOMAIN=%s", prefix, data.Domain),
		"-e", fmt.Sprintf("%s_PRIVATE_KEY=%s", prefix, data.PrivateKey),
		"-e", fmt.Sprintf("%s_APP_PORT=%d", prefix, m.config.AppPort),
		"-e", fmt.Sprintf("%s_ENV=production", prefix),
		"--memory=512m",
		"--restart", "unless-stopped",
		m.config.AppImage,
	}

	_, err := m.runDocker(args...)
	return err
}
```

**Step 3: Run tests**

```bash
cd ~/code/matcha && go test ./... -short -count=1
```

Fix any remaining compilation errors.

**Step 4: Commit**

```bash
git add deploy.go docker.go
git commit -m "Simplify deploy flow with kamal-proxy"
```

---

### Task 4: Update status.go and remaining references

**Files:**
- Modify: `status.go`
- Modify: `matcha.go` (Exec, minor fixes)

**Step 1: Update status.go**

Replace Caddy checks with proxy checks. Remove blue-green slot display:

```go
func (m *Matcha) showStatus() error {
	fmt.Printf("\n%s Status\n", m.config.Name)
	fmt.Println(strings.Repeat("=", 40))

	// Check proxy
	proxyName := m.ProxyContainerName()
	if m.isRunning(proxyName) {
		fmt.Printf("  Proxy:    %s✓ running%s\n", "\033[0;32m", "\033[0m")
		m.showContainerInfo(proxyName)
	} else {
		fmt.Printf("  Proxy:    %s✗ not running%s\n", "\033[0;31m", "\033[0m")
	}

	// Check app
	name := m.AppContainerName()
	if m.isRunning(name) {
		fmt.Printf("  App:      %s✓ running%s\n", "\033[0;32m", "\033[0m")
		m.showContainerInfo(name)
	} else {
		fmt.Printf("  App:      %s✗ not running%s\n", "\033[0;31m", "\033[0m")
	}

	if data, err := m.readEnv(); err == nil {
		fmt.Println()
		fmt.Printf("  Domain:   https://%s\n", data.Domain)
	}

	fmt.Println()
	return nil
}
```

**Step 2: Simplify Exec in matcha.go**

```go
func (m *Matcha) Exec(args ...string) error {
	containerName := m.AppContainerName()
	execArgs := append([]string{"exec", containerName}, args...)
	_, err := m.runDocker(execArgs...)
	return err
}
```

**Step 3: Commit**

```bash
git add status.go matcha.go
git commit -m "Update status and exec for kamal-proxy"
```

---

### Task 5: Fix all tests

**Files:**
- Modify: `matcha_test.go`
- Delete: `caddy_test.go`
- Modify: `integration_test.go`
- Modify: `config_test.go`

**Step 1: Update matcha_test.go**

Remove `TestCaddyContainerName`, `TestAppContainerName` blue-green sub-test. Update defaults test:

```go
func TestNew(t *testing.T) {
	t.Run("applies defaults", func(t *testing.T) {
		m := New(Config{
			Name:     "testapp",
			AppImage: "test/image:latest",
		})

		if m.config.InstallDir != "/opt/testapp" {
			t.Errorf("InstallDir = %q, want /opt/testapp", m.config.InstallDir)
		}
		if m.config.BinaryPath != "/usr/local/bin/testapp" {
			t.Errorf("BinaryPath = %q, want /usr/local/bin/testapp", m.config.BinaryPath)
		}
		if m.config.ProxyImage != "basecamp/kamal-proxy:latest" {
			t.Errorf("ProxyImage = %q, want basecamp/kamal-proxy:latest", m.config.ProxyImage)
		}
		if m.config.HealthPath != "/up" {
			t.Errorf("HealthPath = %q, want /up", m.config.HealthPath)
		}
		if m.config.AppPort != 8080 {
			t.Errorf("AppPort = %d, want 8080", m.config.AppPort)
		}
	})
}

func TestProxyContainerName(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.ProxyContainerName()

	if got != "fusionaly-proxy" {
		t.Errorf("ProxyContainerName() = %q, want fusionaly-proxy", got)
	}
}

func TestAppContainerName(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.AppContainerName()

	if got != "fusionaly-app" {
		t.Errorf("AppContainerName() = %q, want fusionaly-app", got)
	}
}
```

**Step 2: Delete caddy_test.go**

```bash
rm ~/code/matcha/caddy_test.go
```

The `extractBaseDomain` and `generateAdminEmail` functions are also deleted (they were in caddy.go). Remove those tests.

**Step 3: Update config_test.go**

Remove `TestExtractBaseDomain`, `TestGenerateAdminEmail`, and `TestGenerateCaddyfileForContainer` tests since those functions no longer exist.

Keep any `.env` related tests if they exist, updating `CaddyImage` references to `ProxyImage`.

**Step 4: Update integration_test.go**

Change container name checks from `testapp-caddy` to `testapp-proxy`:

```go
if !strings.Contains(out, "testapp-proxy") {
    t.Errorf("Proxy container not running. Docker ps output: %s", out)
}
```

**Step 5: Run all tests**

```bash
cd ~/code/matcha && go test ./... -short -count=1
```

Expected: All short tests PASS.

**Step 6: Commit**

```bash
git add matcha_test.go config_test.go integration_test.go
git rm caddy_test.go
git commit -m "Update tests for kamal-proxy migration"
```

---

### Task 6: Verify and clean up

**Step 1: Full test run**

```bash
cd ~/code/matcha && go vet ./... && go test ./... -short -count=1
```

**Step 2: Review all files for stale Caddy/blue-green references**

```bash
cd ~/code/matcha && grep -ri "caddy\|blue.green\|slot" *.go
```

Should return nothing.

**Step 3: Review diff**

```bash
cd ~/code/matcha && git diff master..kamal-proxy --stat
```

**Step 4: Commit any remaining fixes**

---

## Summary of Removed Complexity

| Removed | Why |
|---------|-----|
| `BlueGreen` config flag | Always zero-downtime now |
| Slot tracking (1/2) | kamal-proxy manages targets |
| `deployBlueGreen()` | kamal-proxy atomic deploy |
| `deploySingle()` | Single deploy path |
| `waitForHealth()` | kamal-proxy health checks |
| `generateCaddyfile()` | No config files to generate |
| `reloadCaddy()` | No Caddy to reload |
| `getActiveSlot()` | No slots |
| Caddyfile template | Gone |
| `caddy/` directories | Gone |

**Net result:** ~150 lines removed, ~60 lines added. Simpler, fewer moving parts.
