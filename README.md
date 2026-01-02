# Web CI/CD Platform

A Kubernetes-native CI/CD platform with automatic Docker builds using Tekton Pipelines.

## 🚀 Features

- **Git Integration**: Automatic branch detection for GitHub, GitLab, and Gitea
- **Dockerfile Validation**: Pre-build validation to ensure Dockerfile exists
- **Kubernetes-Native**: Pipeline execution using Tekton Pipelines
- **Real-time Monitoring**: Live build logs and status via WebSocket/SSE
- **One-Click Operations**: Deploy and rollback with single button click
- **Multi-Registry Support**: Harbor, ECR, GCR, and more
- **Security**: Rootless container builds with Kaniko
- **Scalable**: Horizontal scaling and resource management

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────────┐
│   React Frontend │────│   Go Backend     │────│  Tekton Pipelines   │
│   (Web UI)      │    │   (REST API)     │    │  (Build Engine)     │
└─────────────────┘    └──────────────────┘    └─────────────────────┘
                                │                          │
                                │                          │
                       ┌──────────────────┐    ┌─────────────────────┐
                       │   PostgreSQL     │    │   Container Registry │
                       │   (Metadata)     │    │   (Artifacts)       │
                       └──────────────────┘    └─────────────────────┘
```

### Components

- **Frontend**: React with Ant Design for modern UI/UX
- **Backend**: Go API server with Gin framework
- **Database**: PostgreSQL for application and release metadata
- **Pipeline Engine**: Tekton Pipelines for Kubernetes-native CI/CD
- **Build Engine**: Kaniko for secure, rootless container builds
- **Storage**: Container registry for image artifacts

## 📋 Requirements

- **Kubernetes**: 1.24+ with admin access
- **Tekton Pipelines**: Latest version installed
- **Container Registry**: Harbor, ECR, GCR, or DockerHub
- **Database**: PostgreSQL 13+ (can be deployed in-cluster)
- **Resources**: Minimum 4 CPU cores, 8GB RAM

## 🛠️ Installation

### Option 1: Quick Start with Docker Compose (Development)

```bash
# Clone the repository
git clone <repository-url>
cd cicd-platform

# Start all services
make dev

# Access the application
open http://localhost:3000
```

### Option 2: Kubernetes Deployment (Production)

```bash
# 1. Install Tekton Pipelines
make setup-tekton

# 2. Create secrets for Git and Registry
make setup-git-secret
make setup-registry-secret

# 3. Deploy the platform
make deploy-k8s

# 4. Access via port-forward or ingress
kubectl port-forward -n cicd-system svc/cicd-frontend 8080:80
```

## 🚦 Getting Started

### 1. Create Your First Application

1. Access the web interface at `http://localhost:3000` (dev) or your ingress URL
2. Click "Create App" and fill in:
   - **Application Name**: `my-app`
   - **Git Repository**: `https://github.com/username/my-repo.git`
   - **Registry Repository**: `harbor.company.com/team/my-app`
   - **Target Namespace**: `production`
   - **Deployment Name**: `my-app`

### 2. Prepare Your Repository

Ensure your repository has a `Dockerfile` in the root directory:

```dockerfile
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
```

### 3. Deploy Your Application

1. Go to your application details page
2. Select a branch from the dropdown
3. Click "Validate Dockerfile" to confirm it exists
4. Click "Deploy" to trigger the pipeline
5. Monitor real-time logs and progress

### 4. Monitor and Manage

- **View Releases**: See all deployment history with status
- **Real-time Logs**: Watch build progress with live streaming
- **Rollback**: One-click rollback to previous successful release

## 📚 API Reference

### Applications

```bash
# Create application
POST /api/apps
{
  "name": "my-app",
  "git_url": "https://github.com/user/repo.git",
  "registry_repo": "harbor.company.com/team/my-app",
  "target_namespace": "production",
  "target_deploy_name": "my-app"
}

# List applications
GET /api/apps

# Get application details
GET /api/apps/{id}

# Get branches
GET /api/apps/{id}/branches

# Validate Dockerfile
POST /api/apps/{id}/validate?branch=main
```

### Releases

```bash
# Create release
POST /api/apps/{id}/releases
{
  "branch": "main",
  "commit_sha": "abc123"
}

# Get release details
GET /api/releases/{id}

# Stream logs (WebSocket)
WS /api/releases/{id}/stream

# Rollback
POST /api/releases/{id}/rollback
```

## 🔧 Configuration

### Environment Variables

#### Backend
```bash
DATABASE_HOST=postgresql
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=password
DATABASE_DBNAME=cicd
K8S_IN_CLUSTER=true
K8S_PIPELINE_NAMESPACE=cicd-runners
```

#### Frontend
```bash
REACT_APP_API_URL=http://localhost:8080/api
```

### Kubernetes Secrets

```bash
# Git credentials
kubectl create secret generic cicd-git-secret \
  --namespace=cicd-runners \
  --from-literal=username=your-username \
  --from-literal=password=your-token

# Registry credentials
kubectl create secret docker-registry cicd-registry-secret \
  --namespace=cicd-runners \
  --docker-server=harbor.company.com \
  --docker-username=your-username \
  --docker-password=your-password
```

## 🔍 Troubleshooting

### Common Issues

#### 1. Pipeline Fails with "Dockerfile not found"
- Ensure Dockerfile exists in repository root
- Check `dockerfile_path` configuration in application settings
- Verify Git credentials and repository access

#### 2. WebSocket Connection Failed
- Check network connectivity between frontend and backend
- Verify ingress configuration for WebSocket support
- Ensure proper CORS settings

#### 3. Image Push Failed
- Verify registry credentials in secret
- Check registry permissions for the specified repository
- Ensure registry URL is correct and accessible from cluster

#### 4. Deployment Update Failed
- Verify RBAC permissions for cicd-backend service account
- Check target namespace exists and is accessible
- Ensure deployment name matches configuration

### Debugging Commands

```bash
# Check backend logs
make logs-backend

# Check pipeline execution
kubectl get pipelineruns -n cicd-runners

# Check task details
kubectl describe taskrun -n cicd-runners <taskrun-name>

# Check pod logs
kubectl logs -n cicd-runners <pod-name>
```

## 🧪 Development

### Running Tests

```bash
# Backend tests
make test-backend

# Frontend tests
make test-frontend

# Linting
make lint-backend
make lint-frontend
```

### Local Development

```bash
# Start database only
docker-compose up postgres

# Run backend locally
cd backend
go run cmd/main.go

# Run frontend locally
cd frontend
npm start
```

## 🔒 Security Considerations

- **Rootless Builds**: Uses Kaniko for secure container builds
- **RBAC**: Minimal required permissions for all components
- **Secrets Management**: Kubernetes secrets for credentials
- **Network Security**: Pod-to-pod communication only
- **Image Security**: Can integrate with image scanning tools

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📄 License

This project is licensed under the MIT License. See LICENSE file for details.