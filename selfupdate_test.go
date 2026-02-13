package matcha

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"v1 older", "1.0.0", "1.0.1", -1},
		{"v1 newer", "1.0.2", "1.0.1", 1},
		{"major difference", "2.0.0", "1.9.9", 1},
		{"with v prefix", "v1.0.0", "v1.0.1", -1},
		{"mixed v prefix", "v1.0.0", "1.0.1", -1},
		{"different lengths", "1.0", "1.0.0", 0},
		{"different lengths v1 older", "1.0", "1.0.1", -1},
		{"with dirty suffix", "1.4.37-dirty", "1.4.37", 0},
		{"dirty older than next", "1.4.37-dirty", "1.4.38", -1},
		{"real world upgrade", "1.4.37", "1.4.38", -1},
		{"no upgrade needed", "1.4.38", "1.4.38", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)

			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestGetManagerAssetName(t *testing.T) {
	m := New(Config{Name: "fusionaly", AppImage: "test:latest"})

	got := m.getManagerAssetName()

	// Should contain the name and linux
	if got != "fusionaly-linux-arm64" && got != "fusionaly-linux-amd64" {
		t.Errorf("getManagerAssetName() = %q, want fusionaly-linux-{arch}", got)
	}
}

func TestSelfUpdateSkipsWhenNotConfigured(t *testing.T) {
	m := New(Config{Name: "test", AppImage: "test:latest"})
	// ManagerRepo not set

	updated, err := m.SelfUpdate()

	if err != nil {
		t.Errorf("SelfUpdate() error = %v, want nil", err)
	}
	if updated {
		t.Error("SelfUpdate() = true, want false when not configured")
	}
}

func TestSelfUpdateSkipsDevVersion(t *testing.T) {
	m := New(Config{
		Name:           "test",
		AppImage:       "test:latest",
		ManagerRepo:    "example/repo",
		ManagerVersion: "dev",
	})

	updated, err := m.SelfUpdate()

	if err != nil {
		t.Errorf("SelfUpdate() error = %v, want nil", err)
	}
	if updated {
		t.Error("SelfUpdate() = true, want false for dev version")
	}
}
