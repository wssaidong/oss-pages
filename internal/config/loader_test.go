package config

import (
	"os"
	"testing"
)

func TestLoadWranglerTOML_Success(t *testing.T) {
	content := `
name = "my-app"
compatibility_date = "2024-01-01"

[pages]
build_command = "npm run build"
output_directory = "dist"

[deploy]
server_url = "https://api.example.com"
`
	tmp, _ := os.CreateTemp("", "wrangler.toml")
	tmp.WriteString(content)
	defer os.Remove(tmp.Name())

	cfg, err := LoadWranglerTOML(tmp.Name())
	if err != nil {
		t.Fatalf("LoadWranglerTOML failed: %v", err)
	}
	if cfg.Name != "my-app" {
		t.Errorf("expected name 'my-app', got '%s'", cfg.Name)
	}
	if cfg.Pages.BuildCommand != "npm run build" {
		t.Errorf("expected build_command 'npm run build', got '%s'", cfg.Pages.BuildCommand)
	}
	if cfg.Pages.OutputDirectory != "dist" {
		t.Errorf("expected output_directory 'dist', got '%s'", cfg.Pages.OutputDirectory)
	}
	if cfg.Deploy.ServerURL != "https://api.example.com" {
		t.Errorf("expected server_url 'https://api.example.com', got '%s'", cfg.Deploy.ServerURL)
	}
}

func TestLoadWranglerTOML_NotFound(t *testing.T) {
	_, err := LoadWranglerTOML("/nonexistent/wrangler.toml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
