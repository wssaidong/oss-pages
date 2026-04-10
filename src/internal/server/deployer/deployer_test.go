package deployer

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
	"time"
)

// mockStorage implements Storage interface for testing
type mockStorage struct {
	files   map[string][]byte
	deleted []string
}

func (m *mockStorage) UploadProjectFiles(ctx context.Context, projectName string, files map[string][]byte) (int, error) {
	for k, v := range files {
		m.files[projectName+"/"+k] = v
	}
	return len(files), nil
}

func (m *mockStorage) DeleteProject(ctx context.Context, projectName string) error {
	for k := range m.files {
		if len(k) > len(projectName)+1 && k[:len(projectName)+1] == projectName+"/" {
			delete(m.files, k)
			m.deleted = append(m.deleted, k)
		}
	}
	return nil
}

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestDeployer_Deploy(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	ctx := context.Background()
	zipData := createTestZip(t, map[string]string{
		"index.html":    "<h1>Hello</h1>",
		"static/app.js": "console.log('hi')",
	})

	result, err := d.Deploy(ctx, "my-app", bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if result.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", result.FileCount)
	}
	if result.DeployedAt.IsZero() {
		t.Error("expected DeployedAt to be set")
	}
}

func TestDeployer_Deploy_PathTraversal(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	ctx := context.Background()
	maliciousZip := createTestZip(t, map[string]string{
		"../etc/passwd": "root:x:0:0",
	})

	_, err := d.Deploy(ctx, "my-app", bytes.NewReader(maliciousZip), int64(len(maliciousZip)))
	if err == nil {
		t.Error("expected error for path traversal attack")
	}
}

func TestDeployer_Deploy_EmptyZip(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	ctx := context.Background()
	emptyZip := createTestZip(t, map[string]string{})

	result, err := d.Deploy(ctx, "empty-app", bytes.NewReader(emptyZip), int64(len(emptyZip)))
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if result.FileCount != 0 {
		t.Errorf("expected 0 files, got %d", result.FileCount)
	}
}

func TestDeployer_Deploy_SingleFile(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	ctx := context.Background()
	zipData := createTestZip(t, map[string]string{
		"index.html": "<h1>Single</h1>",
	})

	result, err := d.Deploy(ctx, "single-app", bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if result.FileCount != 1 {
		t.Errorf("expected 1 file, got %d", result.FileCount)
	}
}

func TestDeployer_Deploy_DeployedAt(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	before := time.Now()
	ctx := context.Background()
	zipData := createTestZip(t, map[string]string{
		"index.html": "<h1>Time</h1>",
	})

	result, err := d.Deploy(ctx, "time-app", bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	after := time.Now()

	if result.DeployedAt.Before(before) || result.DeployedAt.After(after) {
		t.Errorf("DeployedAt %v not between before %v and after %v", result.DeployedAt, before, after)
	}
}

func TestDeployer_Deploy_FilesUploadedCorrectly(t *testing.T) {
	storage := &mockStorage{files: make(map[string][]byte)}
	d := NewDeployer(storage)

	ctx := context.Background()
	zipData := createTestZip(t, map[string]string{
		"index.html":    "<h1>Hello</h1>",
		"static/app.js": "console.log('hi')",
	})

	_, err := d.Deploy(ctx, "my-app", bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if len(storage.files) != 2 {
		t.Fatalf("expected 2 files in storage, got %d", len(storage.files))
	}
	if string(storage.files["my-app/index.html"]) != "<h1>Hello</h1>" {
		t.Errorf("index.html content mismatch: got %q", storage.files["my-app/index.html"])
	}
	if string(storage.files["my-app/static/app.js"]) != "console.log('hi')" {
		t.Errorf("static/app.js content mismatch: got %q", storage.files["my-app/static/app.js"])
	}
}
