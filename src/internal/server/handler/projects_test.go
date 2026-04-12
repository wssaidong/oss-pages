package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/storage"
)

// mockProjectsMetaStore implements MetaStore for projects handler testing
type mockProjectsMetaStore struct {
	projects map[string]*storage.ProjectMeta
}

func (m *mockProjectsMetaStore) GetProjects(ctx context.Context) ([]*storage.ProjectMeta, error) {
	var result []*storage.ProjectMeta
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockProjectsMetaStore) GetProject(ctx context.Context, name string) (*storage.ProjectMeta, error) {
	p, ok := m.projects[name]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", name)
	}
	return p, nil
}

func (m *mockProjectsMetaStore) UpsertProject(ctx context.Context, meta *storage.ProjectMeta) error {
	m.projects[meta.Name] = meta
	return nil
}

func (m *mockProjectsMetaStore) DeleteProject(ctx context.Context, name string) error {
	delete(m.projects, name)
	return nil
}

func (m *mockProjectsMetaStore) AcquireDeployLock(ctx context.Context, projectName, projectURL string) (string, error) {
	return "test-deploy-id", nil
}

func (m *mockProjectsMetaStore) ReleaseDeployLock(ctx context.Context, meta *storage.ProjectMeta) error {
	m.projects[meta.Name] = meta
	return nil
}

func (m *mockProjectsMetaStore) GetVersion(ctx context.Context, projectName, versionID string) (*storage.VersionMeta, error) {
	p, ok := m.projects[projectName]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectName)
	}
	for _, v := range p.Versions {
		if v.ID == versionID {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("version '%s' not found", versionID)
}

func (m *mockProjectsMetaStore) AppendVersion(ctx context.Context, projectName string, version storage.VersionMeta, maxVersions int) error {
	p, ok := m.projects[projectName]
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}
	p.Versions = append(p.Versions, version)
	if len(p.Versions) > maxVersions {
		p.Versions = p.Versions[len(p.Versions)-maxVersions:]
	}
	return nil
}

func (m *mockProjectsMetaStore) DeleteVersion(ctx context.Context, projectName, versionID string) error {
	p, ok := m.projects[projectName]
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}
	filtered := make([]storage.VersionMeta, 0, len(p.Versions))
	for _, v := range p.Versions {
		if v.ID != versionID {
			filtered = append(filtered, v)
		}
	}
	p.Versions = filtered
	return nil
}

func (m *mockProjectsMetaStore) UpdateCurrentVersion(ctx context.Context, projectName, versionID string) error {
	p, ok := m.projects[projectName]
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}
	p.CurrentVersion = versionID
	return nil
}

// mockFileStore implements FileStore for projects handler testing
type mockFileStore struct {
	deletedProjects   []string
	deletedVersions   []string
	cleanedRoots      []string
	copiedVersions   []string
	listVersionFiles map[string][]string
}

func (m *mockFileStore) DeleteProject(ctx context.Context, projectName string) error {
	m.deletedProjects = append(m.deletedProjects, projectName)
	return nil
}

func (m *mockFileStore) CleanRootFiles(ctx context.Context, projectName string) error {
	m.cleanedRoots = append(m.cleanedRoots, projectName)
	return nil
}

func (m *mockFileStore) CopyVersionToRoot(ctx context.Context, projectName, versionID string) (int, error) {
	m.copiedVersions = append(m.copiedVersions, versionID)
	return 10, nil
}

func (m *mockFileStore) ListVersionFiles(ctx context.Context, projectName, versionID string) ([]string, error) {
	return m.listVersionFiles[versionID], nil
}

func (m *mockFileStore) DeleteVersionFiles(ctx context.Context, projectName, versionID string) error {
	m.deletedVersions = append(m.deletedVersions, versionID)
	return nil
}

func TestProjectsHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockProjectsMetaStore{
		projects: map[string]*storage.ProjectMeta{
			"my-app": {
				Name:       "my-app",
				URL:        "https://cdn.example.com/my-app/",
				FileCount:  10,
				DeployedAt: time.Now(),
			},
		},
	}
	h := NewProjectsHandler(store, &mockFileStore{}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/projects", nil)

	h.HandleListProjects(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp ProjectsListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(resp.Projects))
	}
}

func TestProjectsHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockProjectsMetaStore{
		projects: map[string]*storage.ProjectMeta{
			"my-app": {
				Name:       "my-app",
				URL:        "https://cdn.example.com/my-app/",
				FileCount:  10,
				DeployedAt: time.Now(),
			},
		},
	}
	h := NewProjectsHandler(store, &mockFileStore{}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/projects/my-app", nil)
	c.Params = gin.Params{{Key: "name", Value: "my-app"}}

	h.HandleGetProject(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProjectsHandler_Get_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockProjectsMetaStore{projects: make(map[string]*storage.ProjectMeta)}
	h := NewProjectsHandler(store, &mockFileStore{}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/projects/nonexistent", nil)
	c.Params = gin.Params{{Key: "name", Value: "nonexistent"}}

	h.HandleGetProject(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProjectsHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &mockProjectsMetaStore{
		projects: map[string]*storage.ProjectMeta{
			"my-app": {Name: "my-app"},
		},
	}
	fs := &mockFileStore{}
	h := NewProjectsHandler(store, fs, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/projects/my-app", nil)
	c.Params = gin.Params{{Key: "name", Value: "my-app"}}

	h.HandleDeleteProject(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if len(store.projects) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(store.projects))
	}
}
