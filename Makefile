.PHONY: build run test clean docker-build docker-run docker-stop

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=octopus

# Docker parameters
DOCKER_IMAGE=octopus
DOCKER_TAG=latest

# Build the server
build:
	cd server && $(GOBUILD) -o ../$(BINARY_NAME) ./cmd/main.go

# Run the server locally
run: build
	./$(BINARY_NAME)

# Run tests
test:
	cd server && $(GOTEST) -v ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Download dependencies
deps:
	cd server && $(GOMOD) download
	cd server && $(GOMOD) tidy

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f docker/Dockerfile.server .

# Run with Docker Compose
docker-run:
	cd docker && docker-compose up -d

# Stop Docker Compose
docker-stop:
	cd docker && docker-compose down

# View Docker logs
docker-logs:
	cd docker && docker-compose logs -f

# Development mode - run server with auto-reload (requires air)
dev:
	cd server && air

# Format code
fmt:
	cd server && $(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	cd server && golangci-lint run

# Generate Go dependencies
generate:
	cd server && $(GOCMD) generate ./...

# Initialize the project
init:
	@echo "Creating necessary directories..."
	mkdir -p docker/config
	cp docker/config/config.yaml.example docker/config/config.yaml
	@echo "Done! Edit docker/config/config.yaml with your settings."

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the server binary"
	@echo "  run          - Build and run the server locally"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose"
	@echo "  docker-logs  - View Docker logs"
	@echo "  dev          - Run in development mode (requires air)"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"
	@echo "  init         - Initialize project configuration"
