package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/deployer"
	"github.com/oss-pages/oss-pages/internal/server/handler"
	"github.com/oss-pages/oss-pages/internal/server/storage"
	"github.com/oss-pages/oss-pages/pkg/s3client"
)

// mockS3 implements s3client.S3API for E2E testing
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

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func setupRouter(s3Mock *mockS3) *gin.Engine {
	gin.SetMode(gin.TestMode)

	s3Client := s3client.NewClient(s3Mock)
	fileStore := storage.NewStorage(s3Client, "test-bucket", "", "_versions/")
	metaStore := storage.NewMetaStore(s3Client, "test-bucket", "")

	d := deployer.NewDeployer(fileStore)
	deployHandler := handler.NewDeployHandler(d, metaStore, fileStore, "https://cdn.example.com", 10)
	projectsHandler := handler.NewProjectsHandler(metaStore, fileStore, "https://cdn.example.com")

	r := gin.New()
	r.POST("/deploy", deployHandler.HandleDeploy)
	r.GET("/projects", projectsHandler.HandleListProjects)
	r.GET("/projects/:name", projectsHandler.HandleGetProject)
	r.DELETE("/projects/:name", projectsHandler.HandleDeleteProject)

	return r
}

func TestE2E_DeployAndList(t *testing.T) {
	mock := newMockS3()
	r := setupRouter(mock)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// 1. Deploy a project
	zipData := createTestZip(t, map[string]string{
		"index.html": "<h1>Hello</h1>",
		"app.js":     "console.log(1)",
	})

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "dist.zip")
	part.Write(zipData)
	writer.WriteField("project", "test-app")
	writer.Close()

	resp, err := http.Post(srv.URL+"/deploy", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatalf("deploy request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("deploy failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var deployResp handler.DeployResponse
	json.NewDecoder(resp.Body).Decode(&deployResp)
	if !deployResp.Success {
		t.Errorf("expected success=true, got response: %+v", deployResp)
	}
	if deployResp.Files != 2 {
		t.Errorf("expected 2 files, got %d", deployResp.Files)
	}

	// 2. Verify files were uploaded to mock S3
	if _, ok := mock.objects["test-app/index.html"]; !ok {
		t.Error("index.html not uploaded to S3")
	}
	if _, ok := mock.objects["test-app/app.js"]; !ok {
		t.Error("app.js not uploaded to S3")
	}

	// 3. List projects
	resp, err = http.Get(srv.URL + "/projects")
	if err != nil {
		t.Fatalf("list projects failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list projects failed: status %d", resp.StatusCode)
	}

	var listResp handler.ProjectsListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	if len(listResp.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(listResp.Projects))
	}
	if listResp.Projects[0].Name != "test-app" {
		t.Errorf("expected project name 'test-app', got '%s'", listResp.Projects[0].Name)
	}

	// 4. Get single project
	resp, err = http.Get(srv.URL + "/projects/test-app")
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get project failed: status %d", resp.StatusCode)
	}

	var getResp handler.ProjectResponse
	json.NewDecoder(resp.Body).Decode(&getResp)
	if !getResp.Success {
		t.Error("expected success=true for get project")
	}
	if getResp.Project.Name != "test-app" {
		t.Errorf("expected project name 'test-app', got '%s'", getResp.Project.Name)
	}

	// 5. Delete project
	req, _ := http.NewRequest("DELETE", srv.URL+"/projects/test-app", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete project failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete failed: status %d", resp.StatusCode)
	}

	// 6. Verify both S3 files and metadata deleted
	var deleteResp handler.DeleteResponse
	json.NewDecoder(resp.Body).Decode(&deleteResp)
	if !deleteResp.Success {
		t.Errorf("expected success=true for delete, got response: %+v", deleteResp)
	}
	if deleteResp.Deleted != "test-app" {
		t.Errorf("expected deleted='test-app', got '%s'", deleteResp.Deleted)
	}

	// 7. Verify project removed from metadata
	resp, err = http.Get(srv.URL + "/projects/test-app")
	if err != nil {
		t.Fatalf("get deleted project failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for deleted project, got %d", resp.StatusCode)
	}
}

func TestE2E_DeployPathTraversal(t *testing.T) {
	mock := newMockS3()
	r := setupRouter(mock)

	srv := httptest.NewServer(r)
	defer srv.Close()

	zipData := createTestZip(t, map[string]string{
		"../etc/passwd": "root:x:0:0",
	})

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "dist.zip")
	part.Write(zipData)
	writer.WriteField("project", "malicious-app")
	writer.Close()

	resp, _ := http.Post(srv.URL+"/deploy", writer.FormDataContentType(), body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", resp.StatusCode)
	}

	// Verify no files uploaded
	keys, _ := mock.ListObjects(context.Background(), "malicious-app/")
	if len(keys) != 0 {
		t.Errorf("expected 0 files for malicious deploy, got %d", len(keys))
	}
}

func TestE2E_MultipleDeploys(t *testing.T) {
	mock := newMockS3()
	r := setupRouter(mock)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Deploy two projects
	for _, name := range []string{"app-a", "app-b"} {
		zipData := createTestZip(t, map[string]string{
			"index.html": fmt.Sprintf("<h1>%s</h1>", name),
		})

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "dist.zip")
		part.Write(zipData)
		writer.WriteField("project", name)
		writer.Close()

		resp, _ := http.Post(srv.URL+"/deploy", writer.FormDataContentType(), body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("deploy %s failed: %d", name, resp.StatusCode)
		}
	}

	// List should show 2 projects
	resp, _ := http.Get(srv.URL + "/projects")
	defer resp.Body.Close()

	var listResp handler.ProjectsListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	if len(listResp.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(listResp.Projects))
	}

	// Verify each project's files in S3
	for _, name := range []string{"app-a", "app-b"} {
		data, _ := mock.GetObject(context.Background(), name+"/index.html")
		expected := fmt.Sprintf("<h1>%s</h1>", name)
		if string(data) != expected {
			t.Errorf("expected '%s' for %s/index.html, got '%s'", expected, name, string(data))
		}
	}
}
