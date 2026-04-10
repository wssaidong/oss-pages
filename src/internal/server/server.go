package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oss-pages/oss-pages/internal/config"
	"github.com/oss-pages/oss-pages/internal/server/deployer"
	"github.com/oss-pages/oss-pages/internal/server/handler"
	"github.com/oss-pages/oss-pages/internal/server/storage"
	"github.com/oss-pages/oss-pages/pkg/s3client"
)

// Run starts the server with the given config and blocks until shutdown.
func Run(cfg *config.ServerConfig) error {
	backend, err := newS3Backend(cfg)
	if err != nil {
		return fmt.Errorf("init backend: %w", err)
	}
	s3Client := s3client.NewClient(backend)

	fileStore := storage.NewStorage(s3Client, cfg.S3.Bucket, cfg.S3.PathPrefix)
	metaStore := storage.NewMetaStore(s3Client, cfg.S3.Bucket, cfg.S3.PathPrefix)

	d := deployer.NewDeployer(fileStore)

	deployHandler := handler.NewDeployHandler(d, metaStore, cfg.CDNBaseURL)
	projectsHandler := handler.NewProjectsHandler(metaStore, fileStore)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.POST("/deploy", deployHandler.HandleDeploy)
	r.GET("/projects", projectsHandler.HandleListProjects)
	r.GET("/projects/:name", projectsHandler.HandleGetProject)
	r.DELETE("/projects/:name", projectsHandler.HandleDeleteProject)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("server listening on %s (backend: %s)", addr, cfg.S3.Backend)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// newS3Backend creates an S3API backend based on config.backend.
// Options: memory (default), file, s3.
func newS3Backend(cfg *config.ServerConfig) (s3client.S3API, error) {
	backend := cfg.S3.Backend
	if backend == "" {
		backend = "memory"
	}

	switch backend {
	case "memory":
		log.Println("Using memory backend (data will be lost on restart)")
		return &placeholderS3{objects: make(map[string][]byte)}, nil

	case "file":
		if cfg.S3.RootDir == "" {
			return nil, fmt.Errorf("s3.root_dir is required for file backend")
		}
		log.Printf("Using file backend: %s", cfg.S3.RootDir)
		return storage.NewFileBackend(cfg.S3.RootDir)

	case "s3":
		log.Println("Using S3 backend: " + cfg.S3.Endpoint)
		return s3client.NewRealS3Client(s3client.S3Config{
			Endpoint:   cfg.S3.Endpoint,
			Bucket:     cfg.S3.Bucket,
			Region:     cfg.S3.Region,
			AccessKey:  cfg.S3.AccessKey,
			SecretKey:  cfg.S3.SecretKey,
			PathPrefix: cfg.S3.PathPrefix,
		}), nil

	default:
		return nil, fmt.Errorf("unknown s3.backend: %q (supported: memory, file, s3)", backend)
	}
}

// placeholderS3 is a development-only in-memory S3 backend.
type placeholderS3 struct {
	objects map[string][]byte
	mu      sync.Mutex
}

func (p *placeholderS3) PutObject(_ context.Context, key string, body io.Reader) error {
	data, _ := io.ReadAll(body)
	p.mu.Lock()
	p.objects[key] = data
	p.mu.Unlock()
	return nil
}

func (p *placeholderS3) GetObject(_ context.Context, key string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.objects[key], nil
}

func (p *placeholderS3) ListObjects(_ context.Context, prefix string) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var keys []string
	for k := range p.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (p *placeholderS3) DeleteObjects(_ context.Context, keys []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range keys {
		delete(p.objects, k)
	}
	return nil
}
