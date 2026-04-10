package storage

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestFileBackend_PutAndGet(t *testing.T) {
	backend, err := NewFileBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileBackend failed: %v", err)
	}

	ctx := context.Background()
	data := []byte("hello world")
	err = backend.PutObject(ctx, "my-app/index.html", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	got, err := backend.GetObject(ctx, "my-app/index.html")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(got))
	}
}

func TestFileBackend_ListAndDelete(t *testing.T) {
	backend, err := NewFileBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileBackend failed: %v", err)
	}

	ctx := context.Background()
	backend.PutObject(ctx, "test/file.txt", bytes.NewReader([]byte("a")))

	keys, err := backend.ListObjects(ctx, "test/")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	backend.DeleteObjects(ctx, []string{"test/file.txt"})

	_, err = backend.GetObject(ctx, "test/file.txt")
	if !os.IsNotExist(err) {
		t.Errorf("expected NotExist, got %v", err)
	}
}

func TestFileBackend_NotFound(t *testing.T) {
	backend, _ := NewFileBackend(t.TempDir())
	_, err := backend.GetObject(context.Background(), "nonexistent/key")
	if !os.IsNotExist(err) {
		t.Errorf("expected NotExist, got %v", err)
	}
}