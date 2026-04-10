package main

import (
	"log"
	"os"

	"github.com/oss-pages/oss-pages/internal/config"
	"github.com/oss-pages/oss-pages/internal/server"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.LoadServerConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := server.Run(cfg); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
