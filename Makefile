.PHONY: all build cli server test clean run push help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

# Build paths
CLI_BINARY=cli
SERVER_BINARY=server
SRC_DIR=src

all: build

# Build both cli and server
build: cli server

# Build CLI
cli:
	cd $(SRC_DIR) && $(GOBUILD) -o ../$(CLI_BINARY) ./cmd/cli

# Build server
server:
	cd $(SRC_DIR) && $(GOBUILD) -o ../$(SERVER_BINARY) ./cmd/server

# Run tests
test:
	cd $(SRC_DIR) && $(GOTEST) -race ./...

# Clean build artifacts
clean:
	rm -f $(CLI_BINARY) $(SERVER_BINARY)
	cd $(SRC_DIR) && $(GOCMD) clean

# Run server locally
run: server
	./$(SERVER_BINARY)

# Push a directory to server (usage: make push DIR=./docs)
push:
	./$(CLI_BINARY) push $(DIR) --server http://127.0.0.1:8080

# Download dependencies
deps:
	cd $(SRC_DIR) && $(GOMOD) download && $(GOMOD) tidy

# Lint
lint:
	cd $(SRC_DIR) && $(GOCMD) vet ./...

help:
	@echo "Available targets:"
	@echo "  build    - Build both cli and server"
	@echo "  cli      - Build CLI binary"
	@echo "  server   - Build server binary"
	@echo "  test     - Run tests with race detector"
	@echo "  clean    - Remove build artifacts"
	@echo "  run      - Build and run server"
	@echo "  push     - Push directory (DIR=./docs make push)"
	@echo "  deps     - Download and tidy dependencies"
	@echo "  lint     - Run go vet"
	@echo "  help     - Show this help"
