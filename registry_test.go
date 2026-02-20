package matcha

import (
	"os"
	"path/filepath"
	"testing"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	return &Registry{BaseDir: t.TempDir()}
}

func testApp() AppEntry {
	return AppEntry{
		Name:       "myapp",
		Image:      "myorg/myapp:latest",
		Domain:     "myapp.example.com",
		Port:       3000,
		HealthPath: "/up",
		Volumes:    []string{"/data:/app/data"},
		Backups:    true,
	}
}

func TestRegistrySaveAndLoad(t *testing.T) {
	reg := testRegistry(t)
	app := testApp()

	err := reg.Save(app)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := reg.Load("myapp")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != app.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, app.Name)
	}
	if loaded.Image != app.Image {
		t.Errorf("Image = %q, want %q", loaded.Image, app.Image)
	}
	if loaded.Domain != app.Domain {
		t.Errorf("Domain = %q, want %q", loaded.Domain, app.Domain)
	}
	if loaded.Port != app.Port {
		t.Errorf("Port = %d, want %d", loaded.Port, app.Port)
	}
	if loaded.HealthPath != app.HealthPath {
		t.Errorf("HealthPath = %q, want %q", loaded.HealthPath, app.HealthPath)
	}
	if len(loaded.Volumes) != 1 || loaded.Volumes[0] != app.Volumes[0] {
		t.Errorf("Volumes = %v, want %v", loaded.Volumes, app.Volumes)
	}
	if loaded.Backups != app.Backups {
		t.Errorf("Backups = %v, want %v", loaded.Backups, app.Backups)
	}
}

func TestRegistryList(t *testing.T) {
	reg := testRegistry(t)

	app1 := AppEntry{Name: "beta", Image: "img:1", Domain: "beta.example.com", Port: 3000}
	app2 := AppEntry{Name: "alpha", Image: "img:2", Domain: "alpha.example.com", Port: 4000}

	reg.Save(app1)
	reg.Save(app2)

	apps, err := reg.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(apps) != 2 {
		t.Fatalf("got %d apps, want 2", len(apps))
	}
	if apps[0].Name != "alpha" {
		t.Errorf("first app = %q, want %q", apps[0].Name, "alpha")
	}
	if apps[1].Name != "beta" {
		t.Errorf("second app = %q, want %q", apps[1].Name, "beta")
	}
}

func TestRegistryRemove(t *testing.T) {
	reg := testRegistry(t)
	app := testApp()

	reg.Save(app)

	err := reg.Remove("myapp")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, err = reg.Load("myapp")
	if err == nil {
		t.Fatal("expected error loading removed app, got nil")
	}
}

func TestRegistryLoadNotFound(t *testing.T) {
	reg := testRegistry(t)

	_, err := reg.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent app, got nil")
	}
}

func TestRegistryAppDir(t *testing.T) {
	reg := &Registry{BaseDir: "/etc/matcha"}

	got := reg.AppDir("myapp")
	want := "/etc/matcha/apps/myapp"

	if got != want {
		t.Errorf("AppDir = %q, want %q", got, want)
	}
}

func TestRegistryEnvFile(t *testing.T) {
	reg := testRegistry(t)
	app := testApp()

	reg.Save(app)

	envPath := reg.EnvPath("myapp")
	if _, err := os.Stat(envPath); err != nil {
		t.Errorf(".env file not created at %s: %v", envPath, err)
	}

	want := filepath.Join(reg.BaseDir, "apps", "myapp", ".env")
	if envPath != want {
		t.Errorf("EnvPath = %q, want %q", envPath, want)
	}
}
