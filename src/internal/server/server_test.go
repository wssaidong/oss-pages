package server

import (
	"testing"

	"github.com/oss-pages/oss-pages/internal/config"
)

func TestInitS3Backend(t *testing.T) {
	cfg := &config.ServerConfig{
		S3: struct {
			Endpoint   string `mapstructure:"endpoint"`
			Bucket     string `mapstructure:"bucket"`
			Region     string `mapstructure:"region"`
			AccessKey  string `mapstructure:"access_key"`
			SecretKey  string `mapstructure:"secret_key"`
			PathPrefix string `mapstructure:"path_prefix"`
		}{
			Endpoint: "https://s3.example.com",
			Bucket:   "test-bucket",
		},
	}

	backend := initS3Backend(cfg)
	if backend == nil {
		t.Error("expected non-nil backend")
	}
}
