package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/deployer"
	"github.com/oss-pages/oss-pages/internal/server/storage"
)

// Deployer interface for dependency injection
type Deployer interface {
	Deploy(ctx context.Context, projectName string, zipReader io.Reader, size int64) (*deployer.DeployResult, error)
}

// MetaStore interface for handler use
type MetaStore interface {
	GetProjects(ctx context.Context) ([]*storage.ProjectMeta, error)
	GetProject(ctx context.Context, name string) (*storage.ProjectMeta, error)
	UpsertProject(ctx context.Context, meta *storage.ProjectMeta) error
	DeleteProject(ctx context.Context, name string) error
}

// DeployResponse is the API response for deploy endpoint
type DeployResponse struct {
	Success    bool   `json:"success"`
	Project    string `json:"project"`
	URL        string `json:"url"`
	Files      int    `json:"files"`
	DeployedAt string `json:"deployed_at"`
	Error      string `json:"error,omitempty"`
	Code       string `json:"code,omitempty"`
}

// DeployHandler handles POST /deploy
type DeployHandler struct {
	deployer   Deployer
	metaStore  MetaStore
	cdnBaseURL string
}

// NewDeployHandler creates a new DeployHandler
func NewDeployHandler(d Deployer, ms MetaStore, cdnBaseURL string) *DeployHandler {
	return &DeployHandler{deployer: d, metaStore: ms, cdnBaseURL: cdnBaseURL}
}

// HandleDeploy handles the deploy request
func (h *DeployHandler) HandleDeploy(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(100 << 20); err != nil {
		c.JSON(http.StatusBadRequest, DeployResponse{
			Success: false,
			Error:   "request too large",
			Code:    "REQUEST_TOO_LARGE",
		})
		return
	}

	projectName := c.PostForm("project")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, DeployResponse{
			Success: false,
			Error:   "missing project name",
			Code:    "MISSING_PROJECT",
		})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, DeployResponse{
			Success: false,
			Error:   "missing file",
			Code:    "MISSING_FILE",
		})
		return
	}
	defer file.Close()

	result, err := h.deployer.Deploy(c.Request.Context(), projectName, file, header.Size)
	if err != nil {
		code := "DEPLOY_FAILED"
		if strings.Contains(err.Error(), "invalid zip") {
			code = "INVALID_ZIP"
		}
		if strings.Contains(err.Error(), "path traversal") {
			code = "INVALID_ZIP"
		}
		c.JSON(http.StatusBadRequest, DeployResponse{
			Success: false,
			Error:   err.Error(),
			Code:    code,
		})
		return
	}

	url := fmt.Sprintf("%s/%s/", strings.TrimSuffix(h.cdnBaseURL, "/"), projectName)
	h.metaStore.UpsertProject(c.Request.Context(), &storage.ProjectMeta{
		Name:       projectName,
		URL:        url,
		FileCount:  result.FileCount,
		DeployedAt: result.DeployedAt,
	})

	c.JSON(http.StatusOK, DeployResponse{
		Success:    true,
		Project:    projectName,
		URL:        url,
		Files:      result.FileCount,
		DeployedAt: result.DeployedAt.Format(time.RFC3339),
	})
}
