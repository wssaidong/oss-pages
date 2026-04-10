package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"projects":[{"name":"my-app","url":"https://cdn.com/my-app/","deployed_at":"2024-01-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()

	err := ListProjects(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
}

func TestViewProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/my-app" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"my-app","url":"https://cdn.com/my-app/","files":10,"deployed_at":"2024-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	err := ViewProject(context.Background(), srv.URL, "my-app")
	if err != nil {
		t.Fatalf("ViewProject failed: %v", err)
	}
}

func TestDeleteProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/projects/my-app" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"deleted":"my-app"}`))
	}))
	defer srv.Close()

	err := DeleteProject(context.Background(), srv.URL, "my-app")
	if err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
}

func TestViewProject_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	err := ViewProject(context.Background(), srv.URL, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}
