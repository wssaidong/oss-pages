package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// WranglerConfig represents wrangler.toml configuration
type WranglerConfig struct {
	Name              string `toml:"name"`
	CompatibilityDate string `toml:"compatibility_date"`
	Pages             struct {
		BuildCommand    string `toml:"build_command"`
		OutputDirectory string `toml:"output_directory"`
	} `toml:"pages"`
	Deploy struct {
		ServerURL string `toml:"server_url"`
	} `toml:"deploy"`
}

// LoadWranglerTOML loads wrangler.toml from the given path
func LoadWranglerTOML(path string) (*WranglerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wrangler.toml: %w", err)
	}
	var cfg WranglerConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse wrangler.toml: %w", err)
	}
	return &cfg, nil
}
