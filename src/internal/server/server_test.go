package server

import (
	"testing"

	"github.com/oss-pages/oss-pages/internal/config"
)

func TestNewS3Backend_Memory(t *testing.T) {
	cfg := &config.ServerConfig{
		S3: struct {
			Endpoint      string `mapstructure:"endpoint"`
			Bucket        string `mapstructure:"bucket"`
			Region        string `mapstructure:"region"`
			AccessKey     string `mapstructure:"access_key"`
			SecretKey     string `mapstructure:"secret_key"`
			PathPrefix    string `mapstructure:"path_prefix"`
			VersionPrefix string `mapstructure:"version_prefix"`
			Backend       string `mapstructure:"backend"`
			RootDir       string `mapstructure:"root_dir"`
		}{
			Backend: "memory",
			Bucket:  "test-bucket",
		},
	}

	backend, err := newS3Backend(cfg)
	if err != nil {
		t.Fatalf("newS3Backend failed: %v", err)
	}
	if backend == nil {
		t.Error("expected non-nil backend")
	}
}

func TestNewS3Backend_File(t *testing.T) {
	cfg := &config.ServerConfig{
		S3: struct {
			Endpoint      string `mapstructure:"endpoint"`
			Bucket        string `mapstructure:"bucket"`
			Region        string `mapstructure:"region"`
			AccessKey     string `mapstructure:"access_key"`
			SecretKey     string `mapstructure:"secret_key"`
			PathPrefix    string `mapstructure:"path_prefix"`
			VersionPrefix string `mapstructure:"version_prefix"`
			Backend       string `mapstructure:"backend"`
			RootDir       string `mapstructure:"root_dir"`
		}{
			Backend: "file",
			RootDir: t.TempDir(),
			Bucket:  "test-bucket",
		},
	}

	backend, err := newS3Backend(cfg)
	if err != nil {
		t.Fatalf("newS3Backend(file) failed: %v", err)
	}
	if backend == nil {
		t.Error("expected non-nil file backend")
	}
}

func TestNewS3Backend_Unknown(t *testing.T) {
	cfg := &config.ServerConfig{
		S3: struct {
			Endpoint      string `mapstructure:"endpoint"`
			Bucket        string `mapstructure:"bucket"`
			Region        string `mapstructure:"region"`
			AccessKey     string `mapstructure:"access_key"`
			SecretKey     string `mapstructure:"secret_key"`
			PathPrefix    string `mapstructure:"path_prefix"`
			VersionPrefix string `mapstructure:"version_prefix"`
			Backend       string `mapstructure:"backend"`
			RootDir       string `mapstructure:"root_dir"`
		}{
			Backend: "unknown",
			Bucket:  "test-bucket",
		},
	}

	_, err := newS3Backend(cfg)
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

func TestNewS3Backend_FileRequiresRootDir(t *testing.T) {
	cfg := &config.ServerConfig{
		S3: struct {
			Endpoint      string `mapstructure:"endpoint"`
			Bucket        string `mapstructure:"bucket"`
			Region        string `mapstructure:"region"`
			AccessKey     string `mapstructure:"access_key"`
			SecretKey     string `mapstructure:"secret_key"`
			PathPrefix    string `mapstructure:"path_prefix"`
			VersionPrefix string `mapstructure:"version_prefix"`
			Backend       string `mapstructure:"backend"`
			RootDir       string `mapstructure:"root_dir"`
		}{
			Backend: "file",
			Bucket:  "test-bucket",
		},
	}

	_, err := newS3Backend(cfg)
	if err == nil {
		t.Error("expected error for file backend without root_dir")
	}
}
