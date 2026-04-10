package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/oss-pages/oss-pages/internal/config"
)

const maxUploadRetries = 3

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
		fmt.Printf("Building project with: %s\n", cfg.Pages.BuildCommand)
		cmd := exec.Command("sh", "-c", cfg.Pages.BuildCommand)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build command failed: %w", err)
		}
		fmt.Println("Build completed.")
	}

	// 3. Zip output directory
	fmt.Printf("Packaging %s ...\n", cfg.Pages.OutputDirectory)
	zipData, err := buildZip(cfg.Pages.OutputDirectory)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}

	// 4. Resolve server URL: --server > OSS_SERVER_URL > wrangler.toml
	url := resolveServerURL(serverURL, cfg.Deploy.ServerURL)

	// 5. Upload
	fmt.Printf("Uploading to %s ...\n", url)
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

// RunPush deploys a local directory directly without building
func RunPush(ctx context.Context, serverURL, configPath, dir string) error {
	// 1. Parse wrangler.toml
	wranglerPath := "wrangler.toml"
	if configPath != "" {
		wranglerPath = configPath
	}
	cfg, err := config.LoadWranglerTOML(wranglerPath)
	if err != nil {
		return fmt.Errorf("load wrangler.toml: %w", err)
	}

	// 2. Zip the specified directory
	fmt.Printf("Packaging %s ...\n", dir)
	zipData, err := buildZip(dir)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}

	// 3. Resolve server URL: --server > OSS_SERVER_URL > wrangler.toml
	url := resolveServerURL(serverURL, cfg.Deploy.ServerURL)

	// 4. Upload
	fmt.Printf("Uploading to %s ...\n", url)
	if err := upload(ctx, url, cfg.Name, zipData); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("Pushed %s to %s/%s/\n", dir, url, cfg.Name)
	return nil
}

func upload(ctx context.Context, serverURL, projectName string, zipData *bytes.Buffer) error {
	zipBytes := zipData.Bytes()
	var lastErr error

	for attempt := 1; attempt <= maxUploadRetries; attempt++ {
		if attempt > 1 {
			delay := time.Duration(1<<(attempt-1)) * time.Second
			fmt.Printf("Retrying in %v (attempt %d/%d)...\n", delay, attempt, maxUploadRetries)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := doUpload(ctx, serverURL, projectName, zipBytes)
		if err == nil {
			return nil
		}
		lastErr = err

		// Don't retry client errors (4xx)
		if isClientError(err) {
			return err
		}
		fmt.Printf("Upload failed: %v\n", err)
	}
	return fmt.Errorf("upload failed after %d attempts: %w", maxUploadRetries, lastErr)
}

func doUpload(ctx context.Context, serverURL, projectName string, zipBytes []byte) error {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "dist.zip")
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, bytes.NewReader(zipBytes)); err != nil {
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
		return &uploadError{
			statusCode: resp.StatusCode,
			body:       string(respBody),
		}
	}
	return nil
}

type uploadError struct {
	statusCode int
	body       string
}

func (e *uploadError) Error() string {
	return fmt.Sprintf("server returned %d: %s", e.statusCode, e.body)
}

func isClientError(err error) bool {
	var ue *uploadError
	if errors.As(err, &ue) {
		return ue.statusCode >= 400 && ue.statusCode < 500
	}
	return false
}
