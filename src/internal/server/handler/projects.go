package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/server/storage"
)

// ProjectsListResponse is the API response for listing projects
type ProjectsListResponse struct {
	Projects []*storage.ProjectMeta `json:"projects"`
}

// ProjectResponse is the API response for a single project
type ProjectResponse struct {
	Success bool                `json:"success"`
	Project *storage.ProjectMeta `json:"project,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// DeleteResponse is the API response for deleting a project
type DeleteResponse struct {
	Success bool   `json:"success"`
	Deleted string `json:"deleted,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RollbackResponse is the API response for rollback
type RollbackResponse struct {
	Success      bool   `json:"success"`
	Project      string `json:"project,omitempty"`
	Version      string `json:"version,omitempty"`
	RolledBackAt string `json:"rolled_back_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

// VersionsResponse is the API response for listing versions
type VersionsResponse struct {
	Versions []VersionResponseItem `json:"versions"`
}

// VersionResponseItem represents a single version in the list
type VersionResponseItem struct {
	ID         string `json:"id"`
	DeployedAt string `json:"deployed_at"`
	FileCount  int    `json:"file_count"`
	PreviewURL string `json:"preview_url"`
	Current    bool   `json:"current"`
}

// DeleteVersionResponse is the API response for deleting a version
type DeleteVersionResponse struct {
	Success        bool   `json:"success"`
	DeletedVersion string `json:"deleted_version,omitempty"`
	Error          string `json:"error,omitempty"`
}

// FileStore interface for file operations
type FileStore interface {
	DeleteProject(ctx context.Context, projectName string) error
	CleanRootFiles(ctx context.Context, projectName string) error
	CopyVersionToRoot(ctx context.Context, projectName, versionID string) (int, error)
	ListVersionFiles(ctx context.Context, projectName, versionID string) ([]string, error)
	DeleteVersionFiles(ctx context.Context, projectName, versionID string) error
}

// ProjectsHandler handles project CRUD operations
type ProjectsHandler struct {
	store      MetaStore
	fileStore  FileStore
	cdnBaseURL string
}

// NewProjectsHandler creates a new ProjectsHandler
func NewProjectsHandler(store MetaStore, fileStore FileStore, cdnBaseURL string) *ProjectsHandler {
	return &ProjectsHandler{store: store, fileStore: fileStore, cdnBaseURL: cdnBaseURL}
}

// HandleListProjects handles GET /projects
func (h *ProjectsHandler) HandleListProjects(c *gin.Context) {
	projects, err := h.store.GetProjects(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if projects == nil {
		projects = []*storage.ProjectMeta{}
	}
	c.JSON(http.StatusOK, ProjectsListResponse{Projects: projects})
}

// HandleGetProject handles GET /projects/:name
func (h *ProjectsHandler) HandleGetProject(c *gin.Context) {
	name := c.Param("name")
	project, err := h.store.GetProject(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, ProjectResponse{
			Success: false,
			Error:   fmt.Sprintf("project '%s' not found", name),
		})
		return
	}
	c.JSON(http.StatusOK, ProjectResponse{
		Success: true,
		Project: project,
	})
}

// HandleDeleteProject handles DELETE /projects/:name
func (h *ProjectsHandler) HandleDeleteProject(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Check project exists
	if _, err := h.store.GetProject(ctx, name); err != nil {
		c.JSON(http.StatusNotFound, DeleteResponse{
			Success: false,
			Error:   fmt.Sprintf("project '%s' not found", name),
		})
		return
	}

	// Step 1: delete S3 files first
	if err := h.fileStore.DeleteProject(ctx, name); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteResponse{
			Success: false,
			Error:   fmt.Sprintf("delete files: %s", err.Error()),
		})
		return
	}

	// Step 2: delete metadata
	if err := h.store.DeleteProject(ctx, name); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DeleteResponse{
		Success: true,
		Deleted: name,
	})
}

// HandleRollback handles POST /projects/:name/rollback
func (h *ProjectsHandler) HandleRollback(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	var req struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, RollbackResponse{
			Success: false,
			Error:   "missing version in request body",
		})
		return
	}

	// Validate version exists
	versionMeta, err := h.store.GetVersion(ctx, name, req.Version)
	if err != nil {
		c.JSON(http.StatusNotFound, RollbackResponse{
			Success: false,
			Error:   fmt.Sprintf("version '%s' not found", req.Version),
		})
		return
	}

	// Acquire deploy lock
	url := fmt.Sprintf("%s/%s/", h.cdnBaseURL, name)
	deployID, err := h.store.AcquireDeployLock(ctx, name, url)
	if err != nil {
		c.JSON(http.StatusConflict, RollbackResponse{
			Success: false,
			Error:   "another deployment is in progress for this project",
		})
		return
	}
	defer h.store.ReleaseDeployLock(ctx, &storage.ProjectMeta{Name: name, URL: url})
	_ = deployID

	// Clean root files
	if err := h.fileStore.CleanRootFiles(ctx, name); err != nil {
		c.JSON(http.StatusInternalServerError, RollbackResponse{
			Success: false,
			Error:   fmt.Sprintf("clean root files: %s", err.Error()),
		})
		return
	}

	// Copy version files to root
	if _, err := h.fileStore.CopyVersionToRoot(ctx, name, req.Version); err != nil {
		c.JSON(http.StatusInternalServerError, RollbackResponse{
			Success: false,
			Error:   fmt.Sprintf("copy version to root: %s", err.Error()),
		})
		return
	}

	// Update current version in metadata
	if err := h.store.UpdateCurrentVersion(ctx, name, req.Version); err != nil {
		c.JSON(http.StatusInternalServerError, RollbackResponse{
			Success: false,
			Error:   fmt.Sprintf("update current version: %s", err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, RollbackResponse{
		Success:      true,
		Project:      name,
		Version:       req.Version,
		RolledBackAt: versionMeta.DeployedAt.Format(time.RFC3339),
	})
}

// HandleListVersions handles GET /projects/:name/versions
func (h *ProjectsHandler) HandleListVersions(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	project, err := h.store.GetProject(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("project '%s' not found", name)})
		return
	}

	versions := make([]VersionResponseItem, len(project.Versions))
	for i, v := range project.Versions {
		versions[i] = VersionResponseItem{
			ID:         v.ID,
			DeployedAt: v.DeployedAt.Format(time.RFC3339),
			FileCount:  v.FileCount,
			PreviewURL: v.PreviewURL,
			Current:    v.ID == project.CurrentVersion,
		}
	}

	c.JSON(http.StatusOK, VersionsResponse{Versions: versions})
}

// HandleDeleteVersion handles DELETE /projects/:name/versions/:id
func (h *ProjectsHandler) HandleDeleteVersion(c *gin.Context) {
	name := c.Param("name")
	versionID := c.Param("id")
	ctx := c.Request.Context()

	// Check version exists
	if _, err := h.store.GetVersion(ctx, name, versionID); err != nil {
		c.JSON(http.StatusNotFound, DeleteVersionResponse{
			Success: false,
			Error:   fmt.Sprintf("version '%s' not found", versionID),
		})
		return
	}

	// Delete version files from storage
	if err := h.fileStore.DeleteVersionFiles(ctx, name, versionID); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteVersionResponse{
			Success: false,
			Error:   fmt.Sprintf("delete version files: %s", err.Error()),
		})
		return
	}

	// Delete version metadata
	if err := h.store.DeleteVersion(ctx, name, versionID); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteVersionResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DeleteVersionResponse{
		Success:        true,
		DeletedVersion: versionID,
	})
}
