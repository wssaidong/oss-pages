package storage

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/oss-pages/oss-pages/pkg/s3client"
)

type mockS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMockS3() *mockS3 {
	return &mockS3{objects: make(map[string][]byte)}
}

func (m *mockS3) PutObject(ctx context.Context, key string, body io.Reader) error {
	data, _ := io.ReadAll(body)
	m.mu.Lock()
	m.objects[key] = data
	m.mu.Unlock()
	return nil
}

func (m *mockS3) GetObject(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.objects[key], nil
}

func (m *mockS3) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *mockS3) DeleteObjects(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.objects, k)
	}
	return nil
}

func TestStorage_UploadProjectFiles(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewStorage(client, "my-bucket", "")

	ctx := context.Background()
	files := map[string][]byte{
		"index.html":    []byte("<h1>Hello</h1>"),
		"static/app.js": []byte("console.log('hi')"),
	}

	count, err := store.UploadProjectFiles(ctx, "my-app", files)
	if err != nil {
		t.Fatalf("UploadProjectFiles failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files, got %d", count)
	}

	data, _ := client.GetObject(ctx, "my-app/index.html")
	if string(data) != "<h1>Hello</h1>" {
		t.Errorf("unexpected content for index.html")
	}
}

func TestStorage_DeleteProject(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewStorage(client, "my-bucket", "")

	ctx := context.Background()
	store.UploadProjectFiles(ctx, "my-app", map[string][]byte{
		"index.html": []byte("test"),
	})

	err := store.DeleteProject(ctx, "my-app")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	keys, _ := client.ListObjects(ctx, "my-app/")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after delete, got %d", len(keys))
	}
}

func TestStorage_WithPathPrefix(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewStorage(client, "my-bucket", "cdn/")

	ctx := context.Background()
	store.UploadProjectFiles(ctx, "my-app", map[string][]byte{
		"index.html": []byte("test"),
	})

	data, _ := client.GetObject(ctx, "cdn/my-app/index.html")
	if string(data) != "test" {
		t.Errorf("expected content at cdn/my-app/index.html")
	}
}
