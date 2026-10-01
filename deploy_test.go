package matcha

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker puts a `docker` script first on PATH. It records every call and
// answers just enough for a deploy: the proxy is running and every command succeeds,
// except `pull` when failPull is set.
func fakeDocker(t *testing.T, failPull bool) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$*" >> "` + logPath + `"
case "$*" in
  pull*) [ "$FAIL_PULL" = 1 ] && exit 1 ;;
  "ps -q --filter name=^matcha-proxy$") echo proxy-id ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if failPull {
		t.Setenv("FAIL_PULL", "1")
	}
	pullRetryDelay = 0
	return logPath
}

func newDeployable(t *testing.T) *Matcha {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	app := AppConfig{Image: "ghcr.io/example/aja:latest", Domain: "aja.example.com", Port: 8080, HealthPath: "/up"}
	if err := SaveAppTo(cfgPath, "aja", app); err != nil {
		t.Fatal(err)
	}
	m := NewFromApp("aja", app, Config{ConfigPath: cfgPath})
	m.domain = app.Domain
	return m
}

func calls(t *testing.T, logPath string) []string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func indexOf(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

func TestDeploy(t *testing.T) {
	t.Run("pulls the configured image before starting the new container", func(t *testing.T) {
		log := fakeDocker(t, false)
		m := newDeployable(t)

		err := m.deploy()

		if err != nil {
			t.Fatalf("deploy: %v", err)
		}
		got := calls(t, log)
		pull, run := indexOf(got, "pull ghcr.io/example/aja:latest"), indexOf(got, "run -d --name aja")
		if pull < 0 || run < 0 || pull > run {
			t.Errorf("want pull before run, got calls:\n%s", strings.Join(got, "\n"))
		}
	})

	t.Run("does not pull when the caller already has the image", func(t *testing.T) {
		log := fakeDocker(t, false)
		m := newDeployable(t)
		m.config.SkipPull = true

		err := m.deploy()

		if err != nil {
			t.Fatalf("deploy: %v", err)
		}
		got := calls(t, log)
		if indexOf(got, "pull") >= 0 || indexOf(got, "run -d --name aja") < 0 {
			t.Errorf("want a run and no pull, got calls:\n%s", strings.Join(got, "\n"))
		}
	})

	t.Run("deploys the local image when the pull fails", func(t *testing.T) {
		log := fakeDocker(t, true)
		m := newDeployable(t)

		err := m.deploy()

		if err != nil {
			t.Fatalf("deploy: %v", err)
		}
		if indexOf(calls(t, log), "run -d --name aja") < 0 {
			t.Error("want the container started from the local image")
		}
	})
}
