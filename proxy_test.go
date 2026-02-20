package matcha

import "testing"

func TestProxyDeployArgs(t *testing.T) {
	m := New(Config{
		Name:     "testapp",
		AppImage: "test:latest",
		AppPort:  8080,
	})

	args := m.proxyDeployArgs("app.example.com")

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

	for _, arg := range args {
		if arg == "--tls" {
			t.Error("should not include --tls for localhost")
		}
		if arg == "--host" {
			t.Error("should not include --host for localhost")
		}
	}

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
