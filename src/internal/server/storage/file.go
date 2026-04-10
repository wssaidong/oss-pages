package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileBackend implements S3API using local filesystem.
// Useful for local development and testing.
type FileBackend struct {
	rootDir string
	mu      sync.RWMutex
	// in-memory index for fast ListObjects
	index map[string][]byte
}

// NewFileBackend creates a FileBackend that stores files under rootDir.
// It scans existing files to rebuild the in-memory index on startup.
func NewFileBackend(rootDir string) (*FileBackend, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, err
	}
	fb := &FileBackend{rootDir: rootDir, index: make(map[string][]byte)}
	if err := fb.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("rebuild index: %w", err)
	}
	return fb, nil
}

// rebuildIndex walks rootDir and loads all files into the in-memory index.
func (f *FileBackend) rebuildIndex() error {
	return filepath.Walk(f.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(f.rootDir, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f.index[key] = data
		return nil
	})
}

// keyToPath converts a logical key (e.g. "my-app/index.html") to an absolute file path.
func (f *FileBackend) keyToPath(key string) string {
	return filepath.Join(f.rootDir, filepath.FromSlash(key))
}

func (f *FileBackend) PutObject(_ context.Context, key string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	path := f.keyToPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	f.mu.Lock()
	f.index[key] = data
	f.mu.Unlock()
	return nil
}

func (f *FileBackend) GetObject(_ context.Context, key string) ([]byte, error) {
	f.mu.RLock()
	data, ok := f.index[key]
	f.mu.RUnlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (f *FileBackend) ListObjects(_ context.Context, prefix string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var keys []string
	for k := range f.index {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (f *FileBackend) DeleteObjects(_ context.Context, keys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, key := range keys {
		path := f.keyToPath(key)
		os.Remove(path)
		delete(f.index, key)
	}
	return nil
}
