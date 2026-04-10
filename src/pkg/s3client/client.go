package s3client

import (
	"bytes"
	"context"
	"io"
)

// S3API defines the interface for S3 operations (mock-friendly)
type S3API interface {
	PutObject(ctx context.Context, key string, body io.Reader) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	ListObjects(ctx context.Context, prefix string) ([]string, error)
	DeleteObjects(ctx context.Context, keys []string) error
}

// Client wraps S3 operations
type Client struct {
	s3 S3API
}

// NewClient creates a new S3 client
func NewClient(s3 S3API) *Client {
	return &Client{s3: s3}
}

// PutObject uploads data to S3
func (c *Client) PutObject(ctx context.Context, key string, data []byte) error {
	return c.s3.PutObject(ctx, key, bytes.NewReader(data))
}

// GetObject downloads data from S3
func (c *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	return c.s3.GetObject(ctx, key)
}

// ListObjects lists all object keys under a prefix
func (c *Client) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	return c.s3.ListObjects(ctx, prefix)
}

// DeleteObjects batch deletes objects
func (c *Client) DeleteObjects(ctx context.Context, keys []string) error {
	return c.s3.DeleteObjects(ctx, keys)
}
