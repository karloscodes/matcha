package matcha

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "DATABASE_URL=sqlite:///data/app.db\nSECRET_KEY=abc123\n# comment\n\nEMPTY=\n"
	os.WriteFile(envPath, []byte(content), 0600)

	vars, err := readEnvFile(envPath)
	if err != nil {
		t.Fatalf("readEnvFile failed: %v", err)
	}

	if vars["DATABASE_URL"] != "sqlite:///data/app.db" {
		t.Errorf("expected DATABASE_URL=sqlite:///data/app.db, got %s", vars["DATABASE_URL"])
	}
	if vars["SECRET_KEY"] != "abc123" {
		t.Errorf("expected SECRET_KEY=abc123, got %s", vars["SECRET_KEY"])
	}
	if _, ok := vars["EMPTY"]; !ok {
		t.Error("expected EMPTY key to exist")
	}
}

func TestReadEnvFileNotFound(t *testing.T) {
	vars, err := readEnvFile("/nonexistent/.env")
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected empty map, got %v", vars)
	}
}
