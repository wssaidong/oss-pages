package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/deployer"
	"github.com/oss-pages/oss-pages/internal/server/storage"
)

// mockDeployer implements the Deployer interface for testing
type mockDeployer struct {
	result *deployer.DeployResult
	err    error
}

func (m *mockDeployer) Deploy(ctx context.Context, projectName string, zipReader io.Reader, size int64) (*deployer.DeployResult, error) {
	return m.result, m.err
}

// mockMetaStoreForDeploy implements MetaStore for testing
type mockMetaStoreForDeploy struct {
	projects map[string]*storage.ProjectMeta
}

func (m *mockMetaStoreForDeploy) GetProjects(ctx context.Context) ([]*storage.ProjectMeta, error) {
	var result []*storage.ProjectMeta
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockMetaStoreForDeploy) GetProject(ctx context.Context, name string) (*storage.ProjectMeta, error) {
	p, ok := m.projects[name]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", name)
	}
	return p, nil
}

func (m *mockMetaStoreForDeploy) UpsertProject(ctx context.Context, meta *storage.ProjectMeta) error {
	m.projects[meta.Name] = meta
	return nil
}

func (m *mockMetaStoreForDeploy) DeleteProject(ctx context.Context, name string) error {
	delete(m.projects, name)
	return nil
}

func createMultipartRequest(t *testing.T, projectName string, zipData []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "dist.zip")
	part.Write(zipData)
	writer.WriteField("project", projectName)
	writer.Close()

	req := httptest.NewRequest("POST", "/deploy", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestDeployHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{
		result: &deployer.DeployResult{FileCount: 5, DeployedAt: time.Now()},
	}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createMultipartRequest(t, "my-app", []byte("fake-zip"))

	h.HandleDeploy(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp DeployResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Files != 5 {
		t.Errorf("expected 5 files, got %d", resp.Files)
	}
}

func TestDeployHandler_MissingProject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "dist.zip")
	part.Write([]byte("not a zip"))
	writer.Close()

	c.Request = httptest.NewRequest("POST", "/deploy", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeployHandler_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("project", "my-app")
	writer.Close()

	c.Request = httptest.NewRequest("POST", "/deploy", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeployHandler_DeployFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{
		err: errors.New("upload failed"),
	}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createMultipartRequest(t, "my-app", []byte("zip-data"))

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp DeployResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != "DEPLOY_FAILED" {
		t.Errorf("expected DEPLOY_FAILED, got %s", resp.Code)
	}
}

func TestDeployHandler_InvalidZip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{
		err: errors.New("invalid zip: bad header"),
	}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createMultipartRequest(t, "my-app", []byte("bad-data"))

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp DeployResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != "INVALID_ZIP" {
		t.Errorf("expected INVALID_ZIP, got %s", resp.Code)
	}
}

func TestDeployHandler_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{
		err: errors.New("path traversal detected: ../../etc/passwd"),
	}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createMultipartRequest(t, "my-app", []byte("traversal-zip"))

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp DeployResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != "INVALID_ZIP" {
		t.Errorf("expected INVALID_ZIP, got %s", resp.Code)
	}
}

func TestDeployHandler_RequestTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockDeployer{}
	h := NewDeployHandler(mock, &mockMetaStoreForDeploy{projects: make(map[string]*storage.ProjectMeta)}, "https://cdn.example.com")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Send a request with a malformed body that will fail ParseMultipartForm
	c.Request = httptest.NewRequest("POST", "/deploy", strings.NewReader("not multipart"))
	c.Request.Header.Set("Content-Type", "multipart/form-data")

	h.HandleDeploy(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp DeployResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != "REQUEST_TOO_LARGE" {
		t.Errorf("expected REQUEST_TOO_LARGE, got %s", resp.Code)
	}
}
