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
	s3Client := s3client.NewClient(initS3Backend(cfg))

	fileStore := storage.NewStorage(s3Client, cfg.S3.Bucket, cfg.S3.PathPrefix)
	metaStore := storage.NewMetaStore(s3Client, cfg.S3.Bucket, cfg.S3.PathPrefix)

	d := deployer.NewDeployer(fileStore)

	deployHandler := handler.NewDeployHandler(d, metaStore, cfg.S3.Endpoint)
	projectsHandler := handler.NewProjectsHandler(metaStore)

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
		log.Printf("server listening on %s", addr)
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

// initS3Backend returns an S3API implementation.
// Currently returns an in-memory placeholder; production AWS SDK v2 integration is a future task.
func initS3Backend(_ *config.ServerConfig) s3client.S3API {
	log.Println("WARNING: Using placeholder S3 backend. Configure AWS credentials for production.")
	return &placeholderS3{objects: make(map[string][]byte)}
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
