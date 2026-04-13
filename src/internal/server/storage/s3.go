package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/oss-pages/oss-pages/pkg/s3client"
)

// Storage provides S3 file operations for projects
type Storage struct {
	client        *s3client.Client
	bucket        string
	pathPrefix    string // production files: "sites/"
	versionPrefix string // version snapshots: "_versions/"
}

// NewStorage creates a new Storage instance
func NewStorage(client *s3client.Client, bucket, pathPrefix, versionPrefix string) *Storage {
	return &Storage{
		client:        client,
		bucket:        bucket,
		pathPrefix:    pathPrefix,
		versionPrefix: versionPrefix,
	}
}

// UploadProjectFiles uploads all files for a project, returns file count
func (s *Storage) UploadProjectFiles(ctx context.Context, projectName string, files map[string][]byte) (int, error) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))
	count := 0
	var mu sync.Mutex

	concurrency := 16
	sem := make(chan struct{}, concurrency)

	for path, data := range files {
		wg.Add(1)
		go func(p string, d []byte) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key := s.projectKey(projectName, p)
			if err := s.client.PutObject(ctx, key, d); err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			count++
			mu.Unlock()
		}(path, data)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

// DeleteProject deletes all files for a project (both production and version snapshots)
func (s *Storage) DeleteProject(ctx context.Context, projectName string) error {
	// Delete production files
	prodPrefix := s.projectPrefix(projectName)
	prodKeys, err := s.client.ListObjects(ctx, prodPrefix)
	if err != nil {
		return fmt.Errorf("list production files: %w", err)
	}
	if len(prodKeys) > 0 {
		if err := s.client.DeleteObjects(ctx, prodKeys); err != nil {
			return fmt.Errorf("delete production files: %w", err)
		}
	}

	// Delete all version snapshots
	verPrefix := s.allVersionsPrefix(projectName)
	verKeys, err := s.client.ListObjects(ctx, verPrefix)
	if err != nil {
		return fmt.Errorf("list version files: %w", err)
	}
	if len(verKeys) > 0 {
		if err := s.client.DeleteObjects(ctx, verKeys); err != nil {
			return fmt.Errorf("delete version files: %w", err)
		}
	}

	return nil
}

// ListProjectFiles lists all file paths for a project
func (s *Storage) ListProjectFiles(ctx context.Context, projectName string) ([]string, error) {
	prefix := s.projectPrefix(projectName)
	return s.client.ListObjects(ctx, prefix)
}

func (s *Storage) projectPrefix(projectName string) string {
	return s.pathPrefix + projectName + "/"
}

func (s *Storage) projectKey(projectName, filePath string) string {
	return s.projectPrefix(projectName) + filePath
}

// versionPathPrefix returns the prefix for version files: {versionPrefix}/{project}/{version-id}/
func (s *Storage) versionPathPrefix(projectName, versionID string) string {
	return s.versionPrefix + projectName + "/" + versionID + "/"
}

// versionKey returns the full S3 key for a versioned file
func (s *Storage) versionKey(projectName, versionID, filePath string) string {
	return s.versionPathPrefix(projectName, versionID) + filePath
}

// allVersionsPrefix returns the prefix for all versions of a project: {versionPrefix}/{project}/
func (s *Storage) allVersionsPrefix(projectName string) string {
	return s.versionPrefix + projectName + "/"
}

// UploadVersionFiles uploads all files for a specific version
func (s *Storage) UploadVersionFiles(ctx context.Context, projectName, versionID string, files map[string][]byte) (int, error) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))
	count := 0
	var mu sync.Mutex

	concurrency := 16
	sem := make(chan struct{}, concurrency)

	for path, data := range files {
		wg.Add(1)
		go func(p string, d []byte) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key := s.versionKey(projectName, versionID, p)
			if err := s.client.PutObject(ctx, key, d); err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			count++
			mu.Unlock()
		}(path, data)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

// CleanRootFiles deletes all production files for a project.
// Since versions are stored under a separate prefix, no filtering is needed.
func (s *Storage) CleanRootFiles(ctx context.Context, projectName string) error {
	prefix := s.projectPrefix(projectName)
	keys, err := s.client.ListObjects(ctx, prefix)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return s.client.DeleteObjects(ctx, keys)
	}
	return nil
}

// ListVersionFiles lists all file paths for a specific version
func (s *Storage) ListVersionFiles(ctx context.Context, projectName, versionID string) ([]string, error) {
	prefix := s.versionPathPrefix(projectName, versionID)
	return s.client.ListObjects(ctx, prefix)
}

// CopyVersionToRoot copies all files from a version directory to the project root
func (s *Storage) CopyVersionToRoot(ctx context.Context, projectName, versionID string) (int, error) {
	versionFiles, err := s.ListVersionFiles(ctx, projectName, versionID)
	if err != nil {
		return 0, err
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(versionFiles))
	count := 0
	var mu sync.Mutex

	concurrency := 16
	sem := make(chan struct{}, concurrency)

	for _, versionKey := range versionFiles {
		wg.Add(1)
		go func(vk string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Get file from version directory
			data, err := s.client.GetObject(ctx, vk)
			if err != nil {
				errCh <- err
				return
			}

			// Extract relative path and write to root
			relPath := strings.TrimPrefix(vk, s.versionPathPrefix(projectName, versionID))
			rootKey := s.projectKey(projectName, relPath)
			if err := s.client.PutObject(ctx, rootKey, data); err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			count++
			mu.Unlock()
		}(versionKey)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

// DeleteVersionFiles deletes all files for a specific version
func (s *Storage) DeleteVersionFiles(ctx context.Context, projectName, versionID string) error {
	keys, err := s.ListVersionFiles(ctx, projectName, versionID)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return s.client.DeleteObjects(ctx, keys)
}
