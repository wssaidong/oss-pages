package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/deployer"
	"github.com/oss-pages/oss-pages/internal/server/storage"
)

// projectNameRe validates project names: lowercase alphanumeric, hyphens, 1-64 chars
var projectNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

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
	AcquireDeployLock(ctx context.Context, projectName, projectURL string) (string, error)
	ReleaseDeployLock(ctx context.Context, meta *storage.ProjectMeta) error
}

// DeployResponse is the API response for deploy endpoint
type DeployResponse struct {
	Success    bool   `json:"success"`
	Project    string `json:"project,omitempty"`
	URL        string `json:"url,omitempty"`
	Files      int    `json:"files,omitempty"`
	DeployedAt string `json:"deployed_at,omitempty"`
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

// validateProjectName checks if the project name is valid
func validateProjectName(name string) bool {
	if len(name) == 1 {
		return name[0] >= 'a' && name[0] <= 'z' || name[0] >= '0' && name[0] <= '9'
	}
	return projectNameRe.MatchString(name)
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
	if !validateProjectName(projectName) {
		c.JSON(http.StatusBadRequest, DeployResponse{
			Success: false,
			Error:   "invalid project name: must be 1-64 chars, lowercase alphanumeric and hyphens",
			Code:    "INVALID_PROJECT_NAME",
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

	url := fmt.Sprintf("%s/%s/", strings.TrimSuffix(h.cdnBaseURL, "/"), projectName)

	// Acquire deploy lock
	deployID, err := h.metaStore.AcquireDeployLock(c.Request.Context(), projectName, url)
	if err != nil {
		if errors.Is(err, storage.ErrDeployInProgress) {
			c.JSON(http.StatusConflict, DeployResponse{
				Success: false,
				Error:   "another deployment is in progress for this project",
				Code:    "DEPLOYMENT_IN_PROGRESS",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, DeployResponse{
			Success: false,
			Error:   fmt.Sprintf("acquire deploy lock: %s", err.Error()),
			Code:    "DEPLOY_FAILED",
		})
		return
	}
	_ = deployID

	// Deploy files
	result, err := h.deployer.Deploy(c.Request.Context(), projectName, file, header.Size)
	if err != nil {
		// Release lock on failure
		h.metaStore.ReleaseDeployLock(c.Request.Context(), &storage.ProjectMeta{
			Name: projectName,
			URL:  url,
		})

		code := "DEPLOY_FAILED"
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid zip") {
			code = "INVALID_ZIP"
			status = http.StatusBadRequest
		}
		if strings.Contains(err.Error(), "path traversal") {
			code = "INVALID_ZIP"
			status = http.StatusBadRequest
		}
		if strings.Contains(err.Error(), "upload") {
			code = "UPLOAD_FAILED"
			status = http.StatusBadGateway
		}
		if strings.Contains(err.Error(), "decompressed size") {
			code = "ZIP_TOO_LARGE"
			status = http.StatusBadRequest
		}
		c.JSON(status, DeployResponse{
			Success: false,
			Error:   err.Error(),
			Code:    code,
		})
		return
	}

	// Release lock and update metadata
	meta := &storage.ProjectMeta{
		Name:       projectName,
		URL:        url,
		FileCount:  result.FileCount,
		DeployedAt: result.DeployedAt,
	}
	if err := h.metaStore.ReleaseDeployLock(c.Request.Context(), meta); err != nil {
		c.JSON(http.StatusServiceUnavailable, DeployResponse{
			Success: false,
			Error:   "files uploaded but metadata update failed, please retry",
			Code:    "META_UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, DeployResponse{
		Success:    true,
		Project:    projectName,
		URL:        url,
		Files:      result.FileCount,
		DeployedAt: result.DeployedAt.Format(time.RFC3339),
	})
}
