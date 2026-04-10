package s3client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mimeTypes maps file extensions to Content-Type values
var mimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".htm":  "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript",
	".json": "application/json",
	".xml":  "application/xml",
	".txt":  "text/plain; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".ico":  "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".wasm":  "application/wasm",
	".pdf":   "application/pdf",
	".zip":   "application/zip",
	".map":   "application/json",
}

// contentType returns the Content-Type for a file key
func contentType(key string) string {
	if ct, ok := mimeTypes[path.Ext(key)]; ok {
		return ct
	}
	return "application/octet-stream"
}

// S3Config holds S3-compatible storage configuration
type S3Config struct {
	Endpoint   string
	Bucket     string
	Region     string
	AccessKey  string
	SecretKey  string
	PathPrefix string
}

// RealS3Client implements S3API using AWS SDK v2
type RealS3Client struct {
	client     *s3.Client
	bucket     string
	pathPrefix string
}

// NewRealS3Client creates a real S3 client
func NewRealS3Client(cfg S3Config) *RealS3Client {
	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})

	return &RealS3Client{
		client:     client,
		bucket:     cfg.Bucket,
		pathPrefix: cfg.PathPrefix,
	}
}

// PutObject uploads data to S3
func (c *RealS3Client) PutObject(_ context.Context, key string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	fullKey := c.pathPrefix + key
	_, err = c.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(data),
		ACL:         types.ObjectCannedACLPublicRead,
		ContentType: aws.String(contentType(fullKey)),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", fullKey, err)
	}
	return nil
}

// GetObject downloads data from S3
func (c *RealS3Client) GetObject(_ context.Context, key string) ([]byte, error) {
	fullKey := c.pathPrefix + key
	resp, err := c.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", fullKey, err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ListObjects lists all object keys under a prefix
func (c *RealS3Client) ListObjects(_ context.Context, prefix string) ([]string, error) {
	fullPrefix := c.pathPrefix + prefix
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

// DeleteObjects batch deletes objects
func (c *RealS3Client) DeleteObjects(_ context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}

	_, err := c.client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: aws.String(c.bucket),
		Delete: &types.Delete{Objects: objects},
	})
	if err != nil {
		return fmt.Errorf("delete objects: %w", err)
	}
	return nil
}
