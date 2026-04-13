package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Server struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	} `mapstructure:"server"`
	S3 struct {
		Endpoint      string `mapstructure:"endpoint"`
		Bucket        string `mapstructure:"bucket"`
		Region        string `mapstructure:"region"`
		AccessKey     string `mapstructure:"access_key"`
		SecretKey     string `mapstructure:"secret_key"`
		PathPrefix    string `mapstructure:"path_prefix"`
		VersionPrefix string `mapstructure:"version_prefix"` // separate prefix for version snapshots (default: "_versions/")
		Backend       string `mapstructure:"backend"`         // memory | file | s3 (default: memory)
		RootDir       string `mapstructure:"root_dir"`        // for file backend
	} `mapstructure:"s3"`
	CDNBaseURL  string `mapstructure:"cdn_base_url"`  // CDN base URL for deployed projects
	MaxVersions int    `mapstructure:"max_versions"` // max versions per project (default: 10)
}

// LoadServerConfig loads config.yaml with env override support
func LoadServerConfig(path string) (*ServerConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ServerConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cfg.S3.Backend == "" {
		cfg.S3.Backend = "memory"
	}
	if cfg.S3.VersionPrefix == "" {
		cfg.S3.VersionPrefix = "_versions/"
	}
	if cfg.MaxVersions <= 0 {
		cfg.MaxVersions = 10
	}
	return &cfg, nil
}
