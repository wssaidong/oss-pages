package cli

import (
	"fmt"
	"os"
	"text/template"
)

var initTemplate = template.Must(template.New("wrangler").Parse(`name = "{{.Name}}"
compatibility_date = "{{.Date}}"

[pages]
build_command = "{{.BuildCommand}}"
output_directory = "{{.OutputDirectory}}"

[deploy]
server_url = "{{.ServerURL}}"
`))

// RunInit creates a wrangler.toml file in the current directory
func RunInit(name, buildCommand, outputDir, serverURL string) error {
	if _, err := os.Stat("wrangler.toml"); err == nil {
		return fmt.Errorf("wrangler.toml already exists")
	}

	f, err := os.Create("wrangler.toml")
	if err != nil {
		return err
	}
	defer f.Close()

	return initTemplate.Execute(f, struct {
		Name            string
		Date            string
		BuildCommand    string
		OutputDirectory string
		ServerURL       string
	}{
		Name:            name,
		Date:            "2024-01-01",
		BuildCommand:    buildCommand,
		OutputDirectory: outputDir,
		ServerURL:       serverURL,
	})
}
