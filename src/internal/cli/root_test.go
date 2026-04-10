package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestRootCommand_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := CreateRootCommand()
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("root --help failed: %v", err)
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("oss-cli")) {
		t.Errorf("help output missing 'oss-cli': %s", output)
	}
}

func TestInitCommand(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	rootCmd := CreateRootCommand()
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"init", "test-app", "--output-dir", tmp})

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("init command failed: %v", err)
	}
}

func TestDeployCommand_MissingConfig(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := CreateRootCommand()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"deploy", "--config", "/nonexistent/wrangler.toml"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing config")
	}
}
