package handler

import (
	"context"
	"fmt"
	"net/http"

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

// FileStore interface for file operations
type FileStore interface {
	DeleteProject(ctx context.Context, projectName string) error
}

// ProjectsHandler handles project CRUD operations
type ProjectsHandler struct {
	store     MetaStore
	fileStore FileStore
}

// NewProjectsHandler creates a new ProjectsHandler
func NewProjectsHandler(store MetaStore, fileStore FileStore) *ProjectsHandler {
	return &ProjectsHandler{store: store, fileStore: fileStore}
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
