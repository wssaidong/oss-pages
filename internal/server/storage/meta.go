package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// MetaStore manages project metadata in S3
type MetaStore struct {
	client     *s3client.Client
	bucket     string
	pathPrefix string
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

// UpsertProject creates or updates project metadata
func (m *MetaStore) UpsertProject(ctx context.Context, meta *ProjectMeta) error {
	projects, _ := m.GetProjects(ctx)

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

	data, err := json.Marshal(&ProjectsMeta{Projects: projects})
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return m.client.PutObject(ctx, m.metaKey(), data)
}

// DeleteProject removes project metadata
func (m *MetaStore) DeleteProject(ctx context.Context, name string) error {
	projects, _ := m.GetProjects(ctx)

	filtered := make([]*ProjectMeta, 0, len(projects))
	for _, p := range projects {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}

	data, err := json.Marshal(&ProjectsMeta{Projects: filtered})
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return m.client.PutObject(ctx, m.metaKey(), data)
}

func (m *MetaStore) metaKey() string {
	return m.pathPrefix + metaFile
}
