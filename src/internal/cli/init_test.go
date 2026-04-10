package cli

import (
	"os"
	"strings"
	"testing"
)

func TestInit_CreatesWranglerTOML(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := RunInit("my-app", "npm run build", "dist", "https://api.example.com")
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile("wrangler.toml")
	if err != nil {
		t.Fatalf("wrangler.toml not created")
	}
	content := string(data)
	if !strings.Contains(content, `name = "my-app"`) {
		t.Error("wrangler.toml missing name")
	}
	if !strings.Contains(content, `build_command = "npm run build"`) {
		t.Error("wrangler.toml missing build_command")
	}
	if !strings.Contains(content, `output_directory = "dist"`) {
		t.Error("wrangler.toml missing output_directory")
	}
}

func TestInit_FileExists(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	os.WriteFile("wrangler.toml", []byte("existing"), 0644)

	err := RunInit("my-app", "npm run build", "dist", "")
	if err == nil {
		t.Error("expected error when wrangler.toml already exists")
	}
}
