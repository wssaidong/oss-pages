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
		Endpoint   string `mapstructure:"endpoint"`
		Bucket     string `mapstructure:"bucket"`
		Region     string `mapstructure:"region"`
		AccessKey  string `mapstructure:"access_key"`
		SecretKey  string `mapstructure:"secret_key"`
		PathPrefix string `mapstructure:"path_prefix"`
	} `mapstructure:"s3"`
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
	return &cfg, nil
}
