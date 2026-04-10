package storage

import (
	"context"
	"sync"

	"github.com/oss-pages/oss-pages/pkg/s3client"
)

// Storage provides S3 file operations for projects
type Storage struct {
	client     *s3client.Client
	bucket     string
	pathPrefix string
}

// NewStorage creates a new Storage instance
func NewStorage(client *s3client.Client, bucket, pathPrefix string) *Storage {
	return &Storage{
		client:     client,
		bucket:     bucket,
		pathPrefix: pathPrefix,
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

// DeleteProject deletes all files for a project
func (s *Storage) DeleteProject(ctx context.Context, projectName string) error {
	prefix := s.projectPrefix(projectName)
	keys, err := s.client.ListObjects(ctx, prefix)
	if err != nil {
		return err
	}
	return s.client.DeleteObjects(ctx, keys)
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
