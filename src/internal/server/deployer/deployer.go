package deployer

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// maxDecompressedSize is the maximum total decompressed size (500MB)
const maxDecompressedSize = 500 << 20

// Storage defines the interface deployer depends on (dependency injection)
type Storage interface {
	UploadProjectFiles(ctx context.Context, projectName string, files map[string][]byte) (int, error)
	DeleteProject(ctx context.Context, projectName string) error
}

// VersioningStorage interface extension for versioning
type VersioningStorage interface {
	Storage
	UploadVersionFiles(ctx context.Context, projectName, versionID string, files map[string][]byte) (int, error)
	CleanRootFiles(ctx context.Context, projectName string) error
	CopyVersionToRoot(ctx context.Context, projectName, versionID string) (int, error)
	ListVersionFiles(ctx context.Context, projectName, versionID string) ([]string, error)
}

// generateVersionID generates a version ID in format: yyyyMMddHHmmss-shortUID
func generateVersionID() string {
	now := time.Now()
	shortUID := uuid.New().String()[:6]
	return now.Format("20060102150405") + "-" + shortUID
}

// DeployResult holds the result of a deployment
type DeployResult struct {
	FileCount  int
	UploadURL  string
	DeployedAt time.Time
}

// Deployer handles streaming zip extraction and upload
type Deployer struct {
	storage VersioningStorage
}

// NewDeployer creates a new Deployer
func NewDeployer(storage VersioningStorage) *Deployer {
	return &Deployer{storage: storage}
}

// Deploy extracts a zip stream and uploads files to storage
func (d *Deployer) Deploy(ctx context.Context, projectName string, zipReader io.Reader, size int64) (*DeployResult, error) {
	// Read all data into memory for zip.NewReader which requires io.ReaderAt
	data, err := io.ReadAll(zipReader)
	if err != nil {
		return nil, fmt.Errorf("read zip data: %w", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}

	files := make(map[string][]byte)
	var totalSize int64
	for _, f := range r.File {
		// Security: prevent path traversal
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") {
			return nil, fmt.Errorf("path traversal detected: %s", f.Name)
		}
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Zip bomb check: verify declared size before reading
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxDecompressedSize {
			return nil, fmt.Errorf("decompressed size exceeds limit (%dMB)", maxDecompressedSize>>20)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		// Use LimitReader as a second guard against lying headers
		fileData, err := io.ReadAll(io.LimitReader(rc, maxDecompressedSize-totalSize+int64(f.UncompressedSize64)))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read zip entry %s: %w", f.Name, err)
		}
		// Use forward slashes for consistency
		clean = filepath.ToSlash(clean)
		files[clean] = fileData
	}

	if len(files) == 0 {
		return &DeployResult{FileCount: 0, DeployedAt: time.Now()}, nil
	}

	count, err := d.storage.UploadProjectFiles(ctx, projectName, files)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	return &DeployResult{
		FileCount:  count,
		DeployedAt: time.Now(),
	}, nil
}

// DeployWithVersion deploys with version management (dual-write: version dir + root)
func (d *Deployer) DeployWithVersion(ctx context.Context, projectName string, zipReader io.Reader, size int64) (*DeployResult, string, error) {
	// Read all data into memory for zip.NewReader which requires io.ReaderAt
	data, err := io.ReadAll(zipReader)
	if err != nil {
		return nil, "", fmt.Errorf("read zip data: %w", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", fmt.Errorf("invalid zip: %w", err)
	}

	files := make(map[string][]byte)
	var totalSize int64
	for _, f := range r.File {
		// Security: prevent path traversal
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") {
			return nil, "", fmt.Errorf("path traversal detected: %s", f.Name)
		}
		// Skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Zip bomb check
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxDecompressedSize {
			return nil, "", fmt.Errorf("decompressed size exceeds limit (%dMB)", maxDecompressedSize>>20)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		fileData, err := io.ReadAll(io.LimitReader(rc, maxDecompressedSize-totalSize+int64(f.UncompressedSize64)))
		rc.Close()
		if err != nil {
			return nil, "", fmt.Errorf("read zip entry %s: %w", f.Name, err)
		}
		clean = filepath.ToSlash(clean)
		files[clean] = fileData
	}

	if len(files) == 0 {
		return &DeployResult{FileCount: 0, DeployedAt: time.Now()}, "", nil
	}

	// Generate version ID
	versionID := generateVersionID()

	// Upload to version directory (first write)
	versionCount, err := d.storage.UploadVersionFiles(ctx, projectName, versionID, files)
	if err != nil {
		return nil, "", fmt.Errorf("upload version files: %w", err)
	}
	_ = versionCount

	// Clean root files (remove old non-version files)
	if err := d.storage.CleanRootFiles(ctx, projectName); err != nil {
		return nil, "", fmt.Errorf("clean root files: %w", err)
	}

	// Upload to root (second write)
	rootCount, err := d.storage.UploadProjectFiles(ctx, projectName, files)
	if err != nil {
		return nil, "", fmt.Errorf("upload root files: %w", err)
	}

	return &DeployResult{
		FileCount:  rootCount, // root file count is the canonical count
		UploadURL:  "",        // URL construction moved to handler
		DeployedAt: time.Now(),
	}, versionID, nil
}
