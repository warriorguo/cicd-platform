# CI/CD Platform API Reference

Complete API reference with request/response schemas.

## Data Models

### App Model (Source Build)
```json
{
  "id": 1,
  "name": "my-app",
  "git_url": "https://github.com/user/repo.git",
  "default_branch": "main",
  "build_type": "dockerfile",
  "dockerfile_path": "Dockerfile",
  "context_path": ".",
  "registry_repo": "harbor.example.com/library/my-app",
  "target_namespace": "default",
  "target_deploy_name": "my-app",
  "replicas": 1,
  "git_secret_ref": "",
  "registry_secret_ref": "",
  "service_name": "my-app",
  "service_port": 8080,
  "service_account": "",
  "ingress_enabled": true,
  "ingress_host": "",
  "env_vars": [],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### App Model (External Image)
```json
{
  "id": 2,
  "name": "my-nginx",
  "git_url": "",
  "default_branch": "",
  "build_type": "external-image",
  "dockerfile_path": "",
  "context_path": "",
  "registry_repo": "nginx",
  "target_namespace": "default",
  "target_deploy_name": "my-nginx",
  "replicas": 1,
  "service_name": "my-nginx",
  "service_port": 80,
  "ingress_enabled": true,
  "env_vars": [],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```
For external-image apps, `git_url` is empty and `registry_repo` stores the image repository (without tag).

### Release Model
```json
{
  "id": 1,
  "app_id": 1,
  "branch": "main",
  "commit_sha": "abc123456789",
  "image_tag": "harbor.example.com/library/my-app:abc1234",
  "image_digest": "sha256:...",
  "status": "success",
  "k8s_ref": "my-app-release-1",
  "started_at": "2024-01-15T10:35:00Z",
  "finished_at": "2024-01-15T10:40:00Z",
  "ingress_created": true,
  "ingress_host": "my-app.example.com",
  "ingress_path": "/",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:40:00Z"
}
```

### Release Status Values
| Status | Description |
|--------|-------------|
| `pending` | Release created, build not started |
| `running` | Build is in progress |
| `success` | Build completed successfully, ready to deploy |
| `failed` | Build failed |
| `deploying` | Deployment in progress |
| `deployed` | Successfully deployed to Kubernetes |

### BuildLog Model
```json
{
  "id": 1,
  "release_id": 1,
  "stage_name": "build",
  "task_run_name": "my-app-release-1-build",
  "pod_name": "my-app-release-1-build-pod",
  "container_name": "step-build",
  "status": "completed",
  "logs_content": "Step 1/10: FROM node:18...",
  "error_message": "",
  "started_at": "2024-01-15T10:35:00Z",
  "finished_at": "2024-01-15T10:38:00Z"
}
```

### BuildLog Status Values
| Status | Description |
|--------|-------------|
| `pending` | Stage not started |
| `running` | Stage is in progress |
| `completed` | Stage completed successfully |
| `failed` | Stage failed |

### EnvVar Model
```json
{
  "name": "DATABASE_URL",
  "value": "postgres://user:pass@host:5432/db",
  "is_secret": false
}
```

---

## Complete Endpoint Reference

### GET /api/apps
List all applications.

**Response:**
```json
[
  {
    "id": 1,
    "name": "my-app",
    "git_url": "https://github.com/user/repo.git",
    ...
  }
]
```

---

### POST /api/apps
Create a new application.

**Request (Source Build):**
```json
{
  "name": "my-app",
  "git_url": "https://github.com/user/repo.git",
  "default_branch": "main",
  "build_type": "dockerfile",
  "dockerfile_path": "Dockerfile",
  "service_port": 8080
}
```

**Request (External Image):**
```json
{
  "name": "my-nginx",
  "build_type": "external-image",
  "external_image": "nginx:latest",
  "service_port": 80
}
```
- For external-image: `external_image` is required, `git_url` must NOT be provided
- For source builds: `git_url` is required

**Response:** `201 Created`
```json
{
  "id": 1,
  "name": "my-app",
  ...
}
```

---

### GET /api/apps/{id}
Get application by ID.

**Response:**
```json
{
  "id": 1,
  "name": "my-app",
  ...
}
```

---

### DELETE /api/apps/{id}
Delete application and all associated resources.

**Preconditions:**
- App must have no running pods (scale to 0 first)

**Response:** `200 OK`
```json
{
  "message": "Application deleted successfully"
}
```

**Error:** `400 Bad Request`
```json
{
  "error": "Cannot delete application with running pods. Please scale down the deployment first."
}
```

---

### GET /api/apps/{id}/branches
Get available Git branches.

**Headers:**
- `X-Git-Token` (optional): Git access token for private repos

**Response:**
```json
{
  "branches": ["main", "develop", "feature/new-feature"]
}
```

---

### POST /api/apps/{id}/releases
Create a new release.

**For source build apps** (triggers a build):

**Headers:**
- `X-Git-Token` (optional): Git access token for private repos

**Request:**
```json
{
  "branch": "main",
  "commit_sha": "abc1234567890"
}
```

**Response:** `201 Created`
```json
{
  "id": 1,
  "app_id": 1,
  "branch": "main",
  "commit_sha": "abc1234567890",
  "status": "pending",
  ...
}
```

**For external-image apps** (no build, instant success):

**Request:**
```json
{
  "image_tag": "nginx:1.25"
}
```
- `image_tag` (optional): Full image address. Defaults to `{app.registry_repo}:latest`

**Response:** `201 Created`
```json
{
  "id": 2,
  "app_id": 2,
  "image_tag": "nginx:1.25",
  "status": "success",
  ...
}
```

**Errors:**
- `400`: Invalid commit_sha (must be >= 7 chars, source apps only)
- `400`: Dockerfile not found (source apps only)
- `409`: Another release is already running

---

### GET /api/apps/{id}/releases
List releases for an app.

**Query Parameters:**
- `page` (default: 1)
- `limit` (default: 10)

**Response:**
```json
{
  "releases": [...],
  "total": 25,
  "page": 1,
  "limit": 10
}
```

---

### GET /api/releases/{id}
Get release details.

**Response:**
```json
{
  "id": 1,
  "app_id": 1,
  "status": "success",
  ...
}
```

---

### GET /api/releases/{id}/build-status
Get build progress and status.

**Response:**
```json
{
  "release_id": 1,
  "overall_status": "running",
  "current_stage": "build",
  "running_stages": ["build"],
  "progress_percentage": 50,
  "completed_stages": 1,
  "total_stages": 2,
  "started_at": "2024-01-15T10:35:00Z",
  "finished_at": null
}
```

---

### GET /api/releases/{id}/build-stages
Get all build stages.

**Response:**
```json
{
  "release_id": 1,
  "stages": [
    {
      "stage_name": "git-clone",
      "status": "completed",
      "started_at": "2024-01-15T10:35:00Z",
      "finished_at": "2024-01-15T10:35:30Z",
      "containers": 1
    },
    {
      "stage_name": "build",
      "status": "running",
      "started_at": "2024-01-15T10:35:30Z",
      "finished_at": null,
      "containers": 1
    }
  ]
}
```

---

### GET /api/releases/{id}/build-logs
Get build logs.

**Query Parameters:**
- `stage` (optional): Filter by stage name
- `container` (optional): Filter by container name
- `status` (optional): Filter by status

**Response:**
```json
{
  "release_id": 1,
  "logs": [
    {
      "id": 1,
      "stage_name": "git-clone",
      "container_name": "step-clone",
      "status": "completed",
      "logs_content": "Cloning into '/workspace/source'...",
      ...
    }
  ],
  "count": 2
}
```

---

### GET /api/releases/{id}/build-logs/{stage}
Get logs for a specific build stage.

**Response:**
```json
{
  "release_id": 1,
  "stage_name": "build",
  "logs": [...]
}
```

---

### POST /api/releases/{id}/deploy
Deploy a successful release.

**Preconditions:**
- Release status must be `success`

**Request:**
```json
{
  "env_vars": [
    {"name": "NODE_ENV", "value": "production"},
    {"name": "API_KEY", "value": "secret", "is_secret": true}
  ],
  "maxUnavailable": 25,
  "cpu_request": "50m",
  "cpu_limit": "200m",
  "memory_request": "64Mi",
  "memory_limit": "256Mi"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| env_vars | array | No | [] | Environment variables for the deployment |
| maxUnavailable | int | No | 25 | Max unavailable percentage during rollout |
| cpu_request | string | No | "50m" | CPU request for pod containers |
| cpu_limit | string | No | "200m" | CPU limit for pod containers |
| memory_request | string | No | "64Mi" | Memory request for pod containers |
| memory_limit | string | No | "256Mi" | Memory limit for pod containers |

**Response:** `200 OK`
```json
{
  "message": "Deployment initiated",
  "release": {
    "id": 1,
    "status": "deploying",
    ...
  }
}
```

**Errors:**
- `400`: Can only deploy successful builds
- `409`: Another deployment is already running

---

### POST /api/releases/{id}/rollback
Rollback to a previous release.

**Preconditions:**
- Target release status must be `success`

**Response:** `200 OK`
```json
{
  "message": "Rollback initiated",
  "release": {...}
}
```

---

### GET /api/apps/{id}/deployments
Get current deployment status.

**Response:**
```json
{
  "deployments": [
    {
      "release_id": 1,
      "image_tag": "harbor.example.com/library/my-app:abc1234",
      "commit_sha": "abc1234567890",
      "deployed_at": "2024-01-15T10:45:00Z",
      "status": "running",
      "replicas": {
        "desired": 2,
        "available": 2,
        "ready": 2
      },
      "namespace": "default",
      "deployment_name": "my-app",
      "pods": [
        {
          "name": "my-app-7d4f5c6b7-abc12",
          "status": "Running",
          "ready": true,
          "restarts": 0,
          "age": "2h30m"
        }
      ]
    }
  ]
}
```

---

### POST /api/apps/{id}/scale
Scale a deployment.

**Request:**
```json
{
  "replicas": 3
}
```

**Constraints:**
- `replicas`: 0-100

**Response:** `200 OK`
```json
{
  "message": "Deployment scaled successfully",
  "app_id": 1,
  "replicas": 3,
  "release_id": 1,
  "namespace": "default",
  "deployment": "my-app"
}
```

---

### POST /api/apps/{id}/reload
Reload (restart) all pods using the same image as the currently deployed release.

**Preconditions:**
- App must have an existing deployed release

**Request:**
```json
{
  "env_vars": [
    {"name": "NODE_ENV", "value": "production"},
    {"name": "API_KEY", "value": "secret", "is_secret": true}
  ],
  "maxUnavailable": 25,
  "cpu_request": "100m",
  "cpu_limit": "500m",
  "memory_request": "128Mi",
  "memory_limit": "512Mi"
}
```
All fields are optional. If `env_vars` or resource settings are provided, they are saved to the app and used for the new deployment.

**Response:** `200 OK`
```json
{
  "message": "Reload initiated",
  "release": {
    "id": 42,
    "app_id": 1,
    "status": "deploying",
    "image_tag": "harbor.example.com/library/my-app:abc1234",
    ...
  }
}
```

**Errors:**
- `400`: No deployed release found to reload
- `409`: Another deployment is already running

---

### DELETE /api/apps/{id}/deployments/{releaseId}
Delete a specific deployment.

**Response:** `200 OK`
```json
{
  "message": "Deployment deleted successfully"
}
```

---

### GET /api/apps/{id}/pods/{podName}/logs
Get runtime logs from a pod.

**Query Parameters:**
- `container` (optional): Container name
- `download` (optional): Set to "true" for file download

**Response:**
```json
{
  "pod_name": "my-app-7d4f5c6b7-abc12",
  "container_name": "my-app",
  "namespace": "default",
  "logs": "2024-01-15 10:45:00 Server started on port 8080\n..."
}
```

---

### GET /api/apps/{id}/pods/{podName}/describe
Get detailed pod information.

**Response:**
```json
{
  "pod_name": "my-app-7d4f5c6b7-abc12",
  "namespace": "default",
  "describe": {
    "name": "my-app-7d4f5c6b7-abc12",
    "namespace": "default",
    "status": "Running",
    "node": "worker-1",
    "containers": [...],
    "events": [
      {
        "type": "Normal",
        "reason": "Scheduled",
        "message": "Successfully assigned..."
      }
    ]
  }
}
```

---

### GET /api/apps/{id}/ingress
Get Ingress configuration.

**Response:**
```json
{
  "enabled": true,
  "created": true,
  "path": "/",
  "host": "my-app.example.com",
  "url": "http://my-app.example.com/",
  "annotations": {
    "kubernetes.io/ingress.class": "nginx"
  }
}
```

---

### POST /api/apps/{id}/validate
Validate Dockerfile exists in repository.

**Query Parameters:**
- `branch` (optional): Branch to check

**Headers:**
- `X-Git-Token` (optional): Git access token

**Response:**
```json
{
  "valid": true,
  "path": "Dockerfile"
}
```

---

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "ok"
}
```
