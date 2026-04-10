package storage

import (
	"context"
	"testing"

	"github.com/oss-pages/oss-pages/pkg/s3client"
)

func TestMetaStore_UpsertAndGet(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewMetaStore(client, "my-bucket", "")

	ctx := context.Background()
	meta := &ProjectMeta{
		Name:      "my-app",
		URL:       "https://cdn.example.com/my-app/",
		FileCount: 5,
	}

	err := store.UpsertProject(ctx, meta)
	if err != nil {
		t.Fatalf("UpsertProject failed: %v", err)
	}

	got, err := store.GetProject(ctx, "my-app")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if got.Name != "my-app" {
		t.Errorf("expected name 'my-app', got '%s'", got.Name)
	}
	if got.FileCount != 5 {
		t.Errorf("expected 5 files, got %d", got.FileCount)
	}
}

func TestMetaStore_GetProjects(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewMetaStore(client, "my-bucket", "")

	ctx := context.Background()
	store.UpsertProject(ctx, &ProjectMeta{Name: "app1", URL: "https://cdn.com/app1/", FileCount: 3})
	store.UpsertProject(ctx, &ProjectMeta{Name: "app2", URL: "https://cdn.com/app2/", FileCount: 7})

	projects, err := store.GetProjects(ctx)
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestMetaStore_DeleteProject(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewMetaStore(client, "my-bucket", "")

	ctx := context.Background()
	store.UpsertProject(ctx, &ProjectMeta{Name: "my-app", URL: "https://cdn.com/my-app/", FileCount: 1})

	err := store.DeleteProject(ctx, "my-app")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	_, err = store.GetProject(ctx, "my-app")
	if err == nil {
		t.Error("expected error for deleted project")
	}
}

func TestMetaStore_GetProject_NotFound(t *testing.T) {
	mock := newMockS3()
	client := s3client.NewClient(mock)
	store := NewMetaStore(client, "my-bucket", "")

	ctx := context.Background()
	_, err := store.GetProject(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}
