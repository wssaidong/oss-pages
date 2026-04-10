package config

import (
	"os"
	"testing"
)

func TestLoadServerConfig_Success(t *testing.T) {
	content := `
server:
  port: 9000
  host: "127.0.0.1"

s3:
  endpoint: "https://s3.example.com"
  bucket: "test-bucket"
  region: "us-west-2"
  access_key: "AKID"
  secret_key: "SECRET"
  path_prefix: "cdn/"
`
	tmp, _ := os.CreateTemp("", "config.yaml")
	tmp.WriteString(content)
	defer os.Remove(tmp.Name())

	cfg, err := LoadServerConfig(tmp.Name())
	if err != nil {
		t.Fatalf("LoadServerConfig failed: %v", err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got '%s'", cfg.Server.Host)
	}
	if cfg.S3.Bucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got '%s'", cfg.S3.Bucket)
	}
	if cfg.S3.Endpoint != "https://s3.example.com" {
		t.Errorf("expected endpoint 'https://s3.example.com', got '%s'", cfg.S3.Endpoint)
	}
	if cfg.S3.Region != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got '%s'", cfg.S3.Region)
	}
	if cfg.S3.AccessKey != "AKID" {
		t.Errorf("expected access_key 'AKID', got '%s'", cfg.S3.AccessKey)
	}
	if cfg.S3.SecretKey != "SECRET" {
		t.Errorf("expected secret_key 'SECRET', got '%s'", cfg.S3.SecretKey)
	}
	if cfg.S3.PathPrefix != "cdn/" {
		t.Errorf("expected path_prefix 'cdn/', got '%s'", cfg.S3.PathPrefix)
	}
	if cfg.S3.Backend != "memory" {
		t.Errorf("expected default backend 'memory', got '%s'", cfg.S3.Backend)
	}
}

func TestLoadServerConfig_FileBackend(t *testing.T) {
	content := `
server:
  port: 9000

s3:
  backend: "file"
  root_dir: "/tmp/oss-pages-data"
  bucket: "test-bucket"
`
	tmp, _ := os.CreateTemp("", "config.yaml")
	tmp.WriteString(content)
	defer os.Remove(tmp.Name())

	cfg, err := LoadServerConfig(tmp.Name())
	if err != nil {
		t.Fatalf("LoadServerConfig failed: %v", err)
	}
	if cfg.S3.Backend != "file" {
		t.Errorf("expected backend 'file', got '%s'", cfg.S3.Backend)
	}
	if cfg.S3.RootDir != "/tmp/oss-pages-data" {
		t.Errorf("expected root_dir '/tmp/oss-pages-data', got '%s'", cfg.S3.RootDir)
	}
}

func TestLoadServerConfig_NotFound(t *testing.T) {
	_, err := LoadServerConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadServerConfig_InvalidYAML(t *testing.T) {
	content := `: invalid yaml : [`
	tmp, _ := os.CreateTemp("", "config.yaml")
	tmp.WriteString(content)
	defer os.Remove(tmp.Name())

	_, err := LoadServerConfig(tmp.Name())
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
