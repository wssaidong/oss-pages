package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDeployCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deploy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.FormValue("project") != "test-project" {
			t.Errorf("unexpected project: %s", r.FormValue("project"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"project":"test-project","url":"https://cdn.example.com/test-project/","files":2}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Create dist directory with test files
	os.MkdirAll("dist", 0755)
	os.WriteFile("dist/index.html", []byte("<h1>Hi</h1>"), 0644)
	os.WriteFile("dist/app.js", []byte("console.log(1)"), 0644)

	// Write wrangler.toml
	content := fmt.Sprintf(`name = "test-project"

[pages]
build_command = ""
output_directory = "dist"

[deploy]
server_url = "%s"
`, srv.URL)
	os.WriteFile("wrangler.toml", []byte(content), 0644)

	err := RunDeploy(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("RunDeploy failed: %v", err)
	}
}
