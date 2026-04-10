package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oss-pages/oss-pages/pkg/s3client"
)

// ProjectMeta holds project metadata
type ProjectMeta struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	FileCount  int       `json:"file_count"`
	DeployedAt time.Time `json:"deployed_at"`
	Deploying  bool      `json:"deploying"`
	DeployID   string    `json:"deploy_id"`
}

// ProjectsMeta is the root metadata file structure
type ProjectsMeta struct {
	Projects []*ProjectMeta `json:"projects"`
}

const metaFile = "_projects.json"

// maxMetaRetries is the maximum number of retries for metadata updates
const maxMetaRetries = 3

// ErrDeployInProgress indicates a concurrent deployment is already running
var ErrDeployInProgress = fmt.Errorf("deployment in progress")

// MetaStore manages project metadata in S3
type MetaStore struct {
	client     *s3client.Client
	bucket     string
	pathPrefix string
	mu         sync.Mutex // protects metadata read-modify-write cycles
}

// NewMetaStore creates a new MetaStore
func NewMetaStore(client *s3client.Client, bucket, pathPrefix string) *MetaStore {
	return &MetaStore{
		client:     client,
		bucket:     bucket,
		pathPrefix: pathPrefix,
	}
}

// GetProjects returns all project metadata
func (m *MetaStore) GetProjects(ctx context.Context) ([]*ProjectMeta, error) {
	data, err := m.client.GetObject(ctx, m.metaKey())
	if err != nil || data == nil {
		return []*ProjectMeta{}, nil
	}
	var meta ProjectsMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return []*ProjectMeta{}, nil
	}
	return meta.Projects, nil
}

// GetProject returns metadata for a specific project
func (m *MetaStore) GetProject(ctx context.Context, name string) (*ProjectMeta, error) {
	projects, err := m.GetProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project '%s' not found", name)
}

// AcquireDeployLock sets deploying=true for the project, returns deploy_id.
// Returns ErrDeployInProgress if the project is already being deployed.
func (m *MetaStore) AcquireDeployLock(ctx context.Context, projectName, projectURL string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, _ := m.getProjectsLocked(ctx)
	deployID := uuid.New().String()

	found := false
	for _, p := range projects {
		if p.Name == projectName {
			if p.Deploying {
				return "", ErrDeployInProgress
			}
			p.Deploying = true
			p.DeployID = deployID
			found = true
			break
		}
	}
	if !found {
		projects = append(projects, &ProjectMeta{
			Name:      projectName,
			URL:       projectURL,
			Deploying: true,
			DeployID:  deployID,
		})
	}

	if err := m.saveProjectsLocked(ctx, projects); err != nil {
		return "", fmt.Errorf("acquire deploy lock: %w", err)
	}
	return deployID, nil
}

// ReleaseDeployLock sets deploying=false and updates deployment result.
func (m *MetaStore) ReleaseDeployLock(ctx context.Context, meta *ProjectMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta.Deploying = false
	meta.DeployID = ""

	projects, _ := m.getProjectsLocked(ctx)
	found := false
	for i, p := range projects {
		if p.Name == meta.Name {
			projects[i] = meta
			found = true
			break
		}
	}
	if !found {
		projects = append(projects, meta)
	}

	return m.saveProjectsRetry(ctx, projects)
}

// UpsertProject creates or updates project metadata
func (m *MetaStore) UpsertProject(ctx context.Context, meta *ProjectMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, _ := m.getProjectsLocked(ctx)

	found := false
	for i, p := range projects {
		if p.Name == meta.Name {
			projects[i] = meta
			found = true
			break
		}
	}
	if !found {
		projects = append(projects, meta)
	}

	return m.saveProjectsRetry(ctx, projects)
}

// DeleteProject removes project metadata
func (m *MetaStore) DeleteProject(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, _ := m.getProjectsLocked(ctx)

	filtered := make([]*ProjectMeta, 0, len(projects))
	for _, p := range projects {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}

	return m.saveProjectsRetry(ctx, filtered)
}

// getProjectsLocked reads projects without acquiring the mutex (caller must hold it)
func (m *MetaStore) getProjectsLocked(ctx context.Context) ([]*ProjectMeta, error) {
	data, err := m.client.GetObject(ctx, m.metaKey())
	if err != nil || data == nil {
		return []*ProjectMeta{}, nil
	}
	var meta ProjectsMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return []*ProjectMeta{}, nil
	}
	return meta.Projects, nil
}

// saveProjectsLocked writes projects to S3 (caller must hold mutex)
func (m *MetaStore) saveProjectsLocked(ctx context.Context, projects []*ProjectMeta) error {
	data, err := json.Marshal(&ProjectsMeta{Projects: projects})
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return m.client.PutObject(ctx, m.metaKey(), data)
}

// saveProjectsRetry writes projects with retry on failure
func (m *MetaStore) saveProjectsRetry(ctx context.Context, projects []*ProjectMeta) error {
	var lastErr error
	for i := 0; i < maxMetaRetries; i++ {
		if err := m.saveProjectsLocked(ctx, projects); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("metadata update failed after %d retries: %w", maxMetaRetries, lastErr)
}

func (m *MetaStore) metaKey() string {
	return m.pathPrefix + metaFile
}
