package main

import (
	"os"

	"github.com/oss-pages/oss-pages/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}