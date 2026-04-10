package s3client

import (
	"context"
	"io"
	"testing"
)

type mockS3 struct {
	objects map[string][]byte
}

func (m *mockS3) PutObject(ctx context.Context, key string, body io.Reader) error {
	data, _ := io.ReadAll(body)
	m.objects[key] = data
	return nil
}

func (m *mockS3) GetObject(ctx context.Context, key string) ([]byte, error) {
	return m.objects[key], nil
}

func (m *mockS3) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.objects {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *mockS3) DeleteObjects(ctx context.Context, keys []string) error {
	for _, k := range keys {
		delete(m.objects, k)
	}
	return nil
}

func TestClient_PutAndGet(t *testing.T) {
	store := &mockS3{objects: make(map[string][]byte)}
	client := NewClient(store)

	ctx := context.Background()
	err := client.PutObject(ctx, "test/key.txt", []byte("hello"))
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	data, err := client.GetObject(ctx, "test/key.txt")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(data))
	}
}

func TestClient_ListObjects(t *testing.T) {
	store := &mockS3{objects: map[string][]byte{
		"proj/file1.txt": []byte("a"),
		"proj/file2.txt": []byte("b"),
		"other/file.txt": []byte("c"),
	}}
	client := NewClient(store)

	ctx := context.Background()
	keys, err := client.ListObjects(ctx, "proj/")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestClient_DeleteObjects(t *testing.T) {
	store := &mockS3{objects: map[string][]byte{
		"proj/file1.txt": []byte("a"),
		"proj/file2.txt": []byte("b"),
	}}
	client := NewClient(store)

	ctx := context.Background()
	err := client.DeleteObjects(ctx, []string{"proj/file1.txt"})
	if err != nil {
		t.Fatalf("DeleteObjects failed: %v", err)
	}

	keys, _ := client.ListObjects(ctx, "proj/")
	if len(keys) != 1 {
		t.Errorf("expected 1 key after delete, got %d", len(keys))
	}
}
