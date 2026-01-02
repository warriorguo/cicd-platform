# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes-native CI/CD platform that provides automatic Docker builds using Tekton Pipelines. The platform consists of:

- **Frontend**: React application with Ant Design components
- **Backend**: Go REST API server using Gin framework  
- **Database**: PostgreSQL for metadata storage
- **Pipeline Engine**: Tekton Pipelines for Kubernetes-native builds
- **Build System**: Kaniko for rootless container builds

## Development Commands

### Local Development

#### Docker-based Development
```bash
# Start full development environment with Docker Compose
make dev

# Stop development environment
make dev-down

# Clean development environment (removes volumes)
make dev-clean
```

#### Non-Docker Local Development
```bash
# Start only PostgreSQL database and wait for it to be ready
make dev-local

# Then in separate terminals:
make dev-local-backend    # Start Go backend on :8080
make dev-local-frontend   # Start React frontend on :3000

# Stop/clean database
make dev-local-stop      # Stop database
make dev-local-clean     # Clean database (removes data)

# Helper commands
make install-deps        # Install Go and npm dependencies
make go-deps            # Download Go dependencies only
make npm-install        # Install npm dependencies only
```

### Testing
```bash
# Run all backend tests
make test-backend
# or: cd backend && go test ./...

# Run frontend tests
make test-frontend
# or: cd frontend && npm test

# Run frontend tests with coverage
cd frontend && npm run test:coverage
```

### Linting
```bash
# Lint backend code
make lint-backend
# This runs: cd backend && go vet ./... && golangci-lint run

# Lint frontend code  
make lint-frontend
# This runs: cd frontend && npm run lint
```

### Building
```bash
# Build container images
make build

# Build and push to registry
make build-push REGISTRY=your-registry.com TAG=v1.0.0
```

## Architecture

### Backend Structure
- `cmd/main.go`: Application entry point
- `internal/api/`: REST API handlers, routes, and WebSocket endpoints
- `internal/db/`: Database connection and migrations
- `internal/git/`: Git client for repository operations
- `internal/k8s/`: Kubernetes client for Tekton pipeline management
- `internal/models/`: Data models and GORM entities
- `pkg/config/`: Configuration management using Viper

### Frontend Structure
- `src/pages/`: Main application pages (AppsList, CreateApp, AppDetails, ReleaseDetails)
- `src/components/`: Reusable React components
- `src/services/api.js`: Axios-based API client
- React Router for navigation, Ant Design for UI components

### Key Integration Points
- Backend serves REST API on port 8080
- Frontend proxies API requests to backend via `proxy: "http://localhost:8080"` in package.json
- WebSocket endpoints for real-time build log streaming
- Tekton Pipelines execute in `cicd-runners` namespace
- Database models use GORM with PostgreSQL driver

## Configuration

The application uses a layered configuration system:
1. `config.yaml`: Default configuration values
2. Environment variables override config file values
3. Backend config structure defined in `pkg/config/config.go`

Key environment variables:
- `DATABASE_*`: Database connection parameters
- `K8S_IN_CLUSTER`: Whether running inside Kubernetes
- `K8S_PIPELINE_NAMESPACE`: Namespace for Tekton pipelines

## Deployment

### Kubernetes Deployment
```bash
# Deploy to Kubernetes (requires cluster access)
make deploy-k8s

# Setup Tekton Pipelines first
make setup-tekton

# Create required secrets
make setup-git-secret
make setup-registry-secret
```

### Development Environment
Uses docker-compose.yml with PostgreSQL database and both frontend/backend services.

## Testing Strategy

- Backend: Go standard testing with testify assertions
- Frontend: React Testing Library with Jest
- Integration tests in `backend/test/` and system tests in `test/`
- Frontend coverage threshold set to 70% for branches, functions, lines, statements

## Key Files for Understanding
- `backend/internal/api/handlers.go`: Core API logic
- `frontend/src/services/api.js`: Frontend-backend communication
- `tekton/pipeline.yaml`: Build pipeline definition
- `deployments/`: Kubernetes manifests for production deployment