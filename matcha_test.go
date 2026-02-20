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

func TestGetConfig(t *testing.T) {
	cfg := Config{
		Name:     "myapp",
		AppImage: "myapp:v1",
	}
	m := New(cfg)

	got := m.GetConfig()

	if got.Name != "myapp" {
		t.Errorf("GetConfig().Name = %q, want myapp", got.Name)
	}

	if got.AppImage != "myapp:v1" {
		t.Errorf("GetConfig().AppImage = %q, want myapp:v1", got.AppImage)
	}
}
