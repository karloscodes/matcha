package matcha

import (
	"testing"
)

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

		if m.config.CaddyImage != "caddy:2-alpine" {
			t.Errorf("CaddyImage = %q, want caddy:2-alpine", m.config.CaddyImage)
		}

		if m.config.HealthPath != "/_health" {
			t.Errorf("HealthPath = %q, want /_health", m.config.HealthPath)
		}

		if m.config.AppPort != 8080 {
			t.Errorf("AppPort = %d, want 8080", m.config.AppPort)
		}
	})

	t.Run("respects custom values", func(t *testing.T) {
		m := New(Config{
			Name:       "testapp",
			AppImage:   "test/image:latest",
			InstallDir: "/custom/path",
			HealthPath: "/healthz",
			AppPort:    3000,
		})

		if m.config.InstallDir != "/custom/path" {
			t.Errorf("InstallDir = %q, want /custom/path", m.config.InstallDir)
		}

		if m.config.HealthPath != "/healthz" {
			t.Errorf("HealthPath = %q, want /healthz", m.config.HealthPath)
		}

		if m.config.AppPort != 3000 {
			t.Errorf("AppPort = %d, want 3000", m.config.AppPort)
		}
	})
}

func TestEnvPrefix(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.EnvPrefix()

	if got != "FUSIONALY" {
		t.Errorf("EnvPrefix() = %q, want FUSIONALY", got)
	}
}

func TestNetworkName(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.NetworkName()

	if got != "fusionaly-network" {
		t.Errorf("NetworkName() = %q, want fusionaly-network", got)
	}
}

func TestCaddyContainerName(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.CaddyContainerName()

	if got != "fusionaly-caddy" {
		t.Errorf("CaddyContainerName() = %q, want fusionaly-caddy", got)
	}
}

func TestAppContainerName(t *testing.T) {
	t.Run("blue-green enabled", func(t *testing.T) {
		m := New(Config{Name: "fusionaly", AppImage: "test:latest", BlueGreen: true})

		if got := m.AppContainerName(1); got != "fusionaly-app-1" {
			t.Errorf("AppContainerName(1) = %q, want fusionaly-app-1", got)
		}

		if got := m.AppContainerName(2); got != "fusionaly-app-2" {
			t.Errorf("AppContainerName(2) = %q, want fusionaly-app-2", got)
		}
	})

	t.Run("blue-green disabled", func(t *testing.T) {
		m := New(Config{Name: "fusionaly", AppImage: "test:latest", BlueGreen: false})

		got := m.AppContainerName(0)

		if got != "fusionaly-app" {
			t.Errorf("AppContainerName(0) = %q, want fusionaly-app", got)
		}
	})
}

func TestGetConfig(t *testing.T) {
	cfg := Config{
		Name:      "myapp",
		AppImage:  "myapp:v1",
		BlueGreen: true,
	}
	m := New(cfg)

	got := m.GetConfig()

	if got.Name != "myapp" {
		t.Errorf("GetConfig().Name = %q, want myapp", got.Name)
	}

	if got.BlueGreen != true {
		t.Errorf("GetConfig().BlueGreen = %v, want true", got.BlueGreen)
	}
}
