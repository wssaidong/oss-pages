package handler

import (
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

// ProjectsHandler handles project CRUD operations
type ProjectsHandler struct {
	store MetaStore
}

// NewProjectsHandler creates a new ProjectsHandler
func NewProjectsHandler(store MetaStore) *ProjectsHandler {
	return &ProjectsHandler{store: store}
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

	if err := h.store.DeleteProject(c.Request.Context(), name); err != nil {
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
