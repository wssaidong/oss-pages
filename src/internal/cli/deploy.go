package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/oss-pages/oss-pages/internal/config"
)

// RunDeploy executes build + zip + upload
func RunDeploy(ctx context.Context, serverURL, configPath string) error {
	// 1. Parse wrangler.toml
	wranglerPath := "wrangler.toml"
	if configPath != "" {
		wranglerPath = configPath
	}
	cfg, err := config.LoadWranglerTOML(wranglerPath)
	if err != nil {
		return fmt.Errorf("load wrangler.toml: %w", err)
	}

	// 2. Run build command
	if cfg.Pages.BuildCommand != "" {
		cmd := exec.Command("sh", "-c", cfg.Pages.BuildCommand)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build command failed: %w", err)
		}
	}

	// 3. Zip output directory
	zipData, err := buildZip(cfg.Pages.OutputDirectory)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}

	// 4. Resolve server URL
	url := serverURL
	if url == "" {
		url = cfg.Deploy.ServerURL
	}
	if url == "" {
		return fmt.Errorf("server URL not configured (use --server or set deploy.server_url in wrangler.toml)")
	}

	// 5. Upload
	if err := upload(ctx, url, cfg.Name, zipData); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("Deployed %s to %s/%s/\n", cfg.Name, url, cfg.Name)
	return nil
}

func buildZip(dir string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// Use forward slashes for zip compatibility
		rel = filepath.ToSlash(rel)

		f, err := w.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	w.Close()
	return buf, nil
}

func upload(ctx context.Context, serverURL, projectName string, zipData *bytes.Buffer) error {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "dist.zip")
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, bytes.NewReader(zipData.Bytes())); err != nil {
		return err
	}
	writer.WriteField("project", projectName)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/deploy", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
