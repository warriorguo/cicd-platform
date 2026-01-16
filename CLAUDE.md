# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Kubernetes-native CI/CD platform for automatic Docker builds using Tekton Pipelines. Users create "Apps" (linked to Git repos), trigger "Releases" (builds), and deploy built images to Kubernetes.

## Development Commands

### Local Development (Non-Docker)
```bash
make dev-local            # Start PostgreSQL and wait for ready
make dev-local-backend    # Terminal 1: Go backend on :8080
make dev-local-frontend   # Terminal 2: React frontend on :3000
```

### Docker-based Development
```bash
make dev                  # Start full environment (postgres + backend + frontend)
make dev-down             # Stop
make dev-clean            # Stop and remove volumes
```

### Testing
```bash
make test-backend                     # Run all Go tests
cd backend && go test ./internal/api  # Run specific package tests
cd backend && go test -run TestCreateApp ./internal/api  # Single test

make test-frontend                    # Run React tests
cd frontend && npm test -- --watch    # Watch mode
cd frontend && npm run test:coverage  # With coverage
```

### Linting
```bash
make lint-backend         # go vet + golangci-lint
make lint-frontend        # npm run lint
```

### Building
```bash
make build                # Build container images locally
cd backend && go build -o bin/cicd-platform cmd/main.go  # Build Go binary
```

## Architecture

### Data Flow
1. **App Creation**: User creates App with Git URL → stored in PostgreSQL
2. **Release (Build)**: User triggers build → Backend creates Tekton PipelineRun → Kaniko builds image → Pushed to Harbor
3. **Deploy**: User deploys successful release → Backend creates Deploy PipelineRun → Updates Kubernetes Deployment + creates Ingress

### Backend (`backend/`)
```
cmd/main.go              # Entry point, initializes Handler with DB + K8s client
internal/
  api/
    routes.go            # All REST endpoints defined here
    handlers.go          # Core business logic: CreateApp, CreateRelease, DeployRelease
    websocket.go         # Real-time log streaming via WebSocket/SSE
  models/app.go          # GORM models: App, Release, BuildLog (with status enums)
  k8s/
    client.go            # Kubernetes client wrapper
    pipeline.go          # Tekton PipelineRun creation for build/deploy
  git/client.go          # Git API client (GitHub/GitLab/Gitea branch listing)
  db/db.go               # Database connection + auto-migration
pkg/config/config.go     # Viper-based config with env var overrides
```

### Frontend (`frontend/src/`)
```
services/api.js          # All API calls (appsAPI object)
pages/
  AppDetails.js          # App view with releases list, deployment info
  BuildDetails.js        # Build progress with stage tracking
  DeployDetails.js       # Deployment status and pod management
  CreateApp.js           # App creation form
```

### Release Status Flow
`pending` → `running` (build) → `success` (build done) → `deploying` → `deployed`

### Key API Endpoints
- `POST /api/apps` - Create app
- `POST /api/apps/:id/releases` - Trigger build
- `POST /api/releases/:id/deploy` - Deploy release
- `GET /api/releases/:id/stream` - WebSocket for live logs
- `POST /api/apps/:id/scale` - Scale deployment replicas
- `GET /api/apps/:id/pods/:podName/logs` - Pod logs
- `GET /api/apps/:id/pods/:podName/tty` - WebSocket TTY to pod

### Tekton Pipelines (`tekton/`)
- `pipeline-build.yaml`: Git clone → Kaniko build → Push to registry
- `pipeline-deploy.yaml`: Create/update Deployment + Service + Ingress

## Configuration

Config loaded from `config.yaml` with env var overrides via Viper.

Key settings in `pkg/config/config.go`:
- `harbor.*`: Registry endpoint, credentials, project name
- `k8s.pipeline_namespace`: Where PipelineRuns execute (default: `cicd-runners`)
- `defaults.target_namespace`: Where apps deploy (default: `default`)
- `ingress.host_template`: Pattern like `*.localhost` for app Ingress hosts

## Database

PostgreSQL with GORM auto-migration. Key tables:
- `apps`: Application definitions
- `releases`: Build/deploy history with status tracking
- `build_logs`: Per-stage logs with container-level granularity

## Testing Notes

- Backend tests use `testify/assert`
- Frontend tests use React Testing Library + Jest
- Integration tests in `backend/test/` require running database