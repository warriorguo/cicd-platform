---
name: cicd-manager
description: Manage CI/CD workflows including building Docker images, deploying releases, deploying external/pre-built images, and monitoring build/deploy/runtime logs. Supports creating apps, triggering builds, deploying to Kubernetes, scaling deployments, reloading (restarting) pods, and viewing pod logs.
license: Apache-2.0
compatibility: Requires network access to the CI/CD platform API
metadata:
  author: cicd-platform
  version: "1.0"
---

# CI/CD Manager Skill

This skill manages the complete CI/CD workflow for a Kubernetes-native CI/CD platform using Tekton Pipelines.

## Activation

When this skill is activated, first ask the user for the CI/CD platform domain/URL:

**Ask the user:** "What is the CI/CD platform API URL? (e.g., http://localhost:8080 or https://cicd.example.com)"

Store this as `CICD_API_URL` and use it as the base URL for all API calls.

## API Reference

All endpoints are relative to `{CICD_API_URL}/api`.

### Apps Management

#### List All Apps
```
GET /api/apps
```
Returns array of all applications.

#### Get App Details
```
GET /api/apps/{app_id}
```
Returns detailed information about a specific app.

#### Create App (Build from Source)
```
POST /api/apps
Content-Type: application/json

{
  "name": "my-app",
  "git_url": "https://github.com/user/repo.git",
  "default_branch": "main",
  "build_type": "dockerfile",
  "dockerfile_path": "Dockerfile",
  "service_port": 8080
}
```
- `name` (required): Application name
- `git_url` (required for source builds): Git repository URL
- `default_branch` (optional): Default branch, defaults to "main"
- `build_type` (optional): "dockerfile", "docker-compose", or "external-image"
- `dockerfile_path` (optional): Path to Dockerfile, defaults to "Dockerfile"
- `service_port` (optional): Service port, defaults to 8080

#### Create App (External Image)
```
POST /api/apps
Content-Type: application/json

{
  "name": "my-nginx",
  "build_type": "external-image",
  "external_image": "nginx:latest",
  "service_port": 80
}
```
- `name` (required): Application name
- `build_type` (required): Must be "external-image"
- `external_image` (required): Full image address (e.g., `nginx:latest`, `gcr.io/project/app:v1`)
- `service_port` (optional): Service port, defaults to 8080
- `git_url` must NOT be provided for external-image apps

#### Delete App
```
DELETE /api/apps/{app_id}
```
Deletes app and all associated resources. App must have no running pods.

---

### Building Images (Releases)

#### Trigger a Build (Source Apps)
```
POST /api/apps/{app_id}/releases
Content-Type: application/json
X-Git-Token: <optional-git-token>

{
  "branch": "main",
  "commit_sha": "abc1234567890"
}
```
- `branch` (optional): Branch to build from
- `commit_sha` (required): Full or short commit SHA (min 7 chars)

Returns the created release object with status `pending`.

#### Create Release (External Image Apps)
```
POST /api/apps/{app_id}/releases
Content-Type: application/json

{
  "image_tag": "nginx:1.25"
}
```
- `image_tag` (optional): Full image address to deploy. Defaults to `{app.registry_repo}:latest`

Returns the created release object with status `success` (no build needed). Deploy it immediately with `POST /api/releases/{id}/deploy`.

#### List Releases for App
```
GET /api/apps/{app_id}/releases?page=1&limit=10
```
Returns paginated list of releases.

#### Get Release Details
```
GET /api/releases/{release_id}
```
Returns release information including status.

**Release Status Flow:** `pending` -> `running` -> `success` -> `deploying` -> `deployed`

---

### Build Logs

#### Get Build Status
```
GET /api/releases/{release_id}/build-status
```
Returns:
```json
{
  "release_id": 1,
  "overall_status": "running",
  "current_stage": "build",
  "running_stages": ["build"],
  "progress_percentage": 50,
  "completed_stages": 1,
  "total_stages": 2
}
```

#### Get Build Stages
```
GET /api/releases/{release_id}/build-stages
```
Returns all build stages with their status.

#### Get Build Logs
```
GET /api/releases/{release_id}/build-logs
GET /api/releases/{release_id}/build-logs?stage=git-clone
GET /api/releases/{release_id}/build-logs?container=step-clone
```
Optional query params: `stage`, `container`, `status`

#### Get Logs for Specific Stage
```
GET /api/releases/{release_id}/build-logs/{stage_name}
```
Returns detailed logs for a specific build stage.

---

### Deploying Releases

#### Deploy a Release
```
POST /api/releases/{release_id}/deploy
Content-Type: application/json

{
  "env_vars": [
    {"name": "DATABASE_URL", "value": "postgres://..."},
    {"name": "API_KEY", "value": "secret", "is_secret": true}
  ],
  "maxUnavailable": 25
}
```
- `env_vars` (optional): Environment variables for the deployment
- `maxUnavailable` (optional): Max unavailable percentage during rollout, defaults to 25

Only releases with status `success` can be deployed.

#### Rollback to Previous Release
```
POST /api/releases/{release_id}/rollback
```
Rolls back to a previously successful release.

#### Delete Deployment
```
DELETE /api/apps/{app_id}/deployments/{release_id}
```
Deletes the Kubernetes deployment for a specific release.

---

### Deployment Management

#### Get App Deployments
```
GET /api/apps/{app_id}/deployments
```
Returns current deployment information including:
- Pod status and details
- Replica counts (desired, available, ready)
- Deployment metadata

#### Scale Deployment
```
POST /api/apps/{app_id}/scale
Content-Type: application/json

{
  "replicas": 3
}
```
Scales the deployment to the specified number of replicas (0-100).

#### Reload (Restart) Pods
```
POST /api/apps/{app_id}/reload
Content-Type: application/json

{
  "env_vars": [
    {"name": "NODE_ENV", "value": "production"}
  ],
  "maxUnavailable": 25
}
```
- `env_vars` (optional): Updated environment variables (saved to app if provided)
- `maxUnavailable` (optional): Max unavailable percentage during rollout, defaults to 25

Restarts all pods by creating a new deploy PipelineRun using the same image as the currently deployed release. Useful for picking up config changes or restarting unhealthy pods. Requires an existing deployed release.

---

### Runtime Pod Logs

#### Get Pod Logs
```
GET /api/apps/{app_id}/pods/{pod_name}/logs
GET /api/apps/{app_id}/pods/{pod_name}/logs?container=main
GET /api/apps/{app_id}/pods/{pod_name}/logs?download=true
```
- `container` (optional): Specific container name
- `download` (optional): Returns as downloadable file

Returns runtime logs from a running pod.

#### Get Pod Description
```
GET /api/apps/{app_id}/pods/{pod_name}/describe
```
Returns detailed pod information including events, useful for debugging.

#### Pod TTY (WebSocket)
```
WS /api/apps/{app_id}/pods/{pod_name}/tty?container=main
```
Opens an interactive terminal session to a pod via WebSocket.

---

### Additional Endpoints

#### Get App Branches
```
GET /api/apps/{app_id}/branches
X-Git-Token: <optional-git-token>
```
Lists available Git branches for the app's repository.

#### Validate Dockerfile
```
POST /api/apps/{app_id}/validate?branch=main
X-Git-Token: <optional-git-token>
```
Validates that the Dockerfile exists in the repository.

#### Get App Ingress
```
GET /api/apps/{app_id}/ingress
```
Returns Ingress configuration and URL for the app.

---

## Common Workflows

### 1. Build and Deploy a New Version

```bash
# 1. Trigger a build
curl -X POST {CICD_API_URL}/api/apps/{app_id}/releases \
  -H "Content-Type: application/json" \
  -d '{"branch": "main", "commit_sha": "abc1234"}'

# 2. Monitor build status
curl {CICD_API_URL}/api/releases/{release_id}/build-status

# 3. View build logs if needed
curl {CICD_API_URL}/api/releases/{release_id}/build-logs

# 4. Deploy once build succeeds
curl -X POST {CICD_API_URL}/api/releases/{release_id}/deploy \
  -H "Content-Type: application/json" \
  -d '{}'

# 5. Check deployment status
curl {CICD_API_URL}/api/apps/{app_id}/deployments
```

### 2. Debug a Failing Pod

```bash
# 1. Get deployment to find pod names
curl {CICD_API_URL}/api/apps/{app_id}/deployments

# 2. Get pod description for events
curl {CICD_API_URL}/api/apps/{app_id}/pods/{pod_name}/describe

# 3. Get runtime logs
curl {CICD_API_URL}/api/apps/{app_id}/pods/{pod_name}/logs
```

### 3. Reload (Restart) Pods

```bash
# Reload with same config
curl -X POST {CICD_API_URL}/api/apps/{app_id}/reload \
  -H "Content-Type: application/json" \
  -d '{}'

# Reload with updated env vars
curl -X POST {CICD_API_URL}/api/apps/{app_id}/reload \
  -H "Content-Type: application/json" \
  -d '{"env_vars": [{"name": "LOG_LEVEL", "value": "debug"}]}'
```

### 4. Deploy an External Image

```bash
# 1. Create app (one-time)
curl -X POST {CICD_API_URL}/api/apps \
  -H "Content-Type: application/json" \
  -d '{"name": "my-nginx", "build_type": "external-image", "external_image": "nginx:latest", "service_port": 80}'

# 2. Create release (instant success, no build)
curl -X POST {CICD_API_URL}/api/apps/{app_id}/releases \
  -H "Content-Type: application/json" \
  -d '{"image_tag": "nginx:1.25"}'

# 3. Deploy with optional env vars
curl -X POST {CICD_API_URL}/api/releases/{release_id}/deploy \
  -H "Content-Type: application/json" \
  -d '{"env_vars": [{"name": "MY_VAR", "value": "hello"}]}'

# 4. Check deployment status
curl {CICD_API_URL}/api/apps/{app_id}/deployments
```

### 5. Scale a Deployment

```bash
# Scale up
curl -X POST {CICD_API_URL}/api/apps/{app_id}/scale \
  -H "Content-Type: application/json" \
  -d '{"replicas": 5}'

# Scale down to zero (stop all pods)
curl -X POST {CICD_API_URL}/api/apps/{app_id}/scale \
  -H "Content-Type: application/json" \
  -d '{"replicas": 0}'
```

---

## Error Handling

Common HTTP status codes:
- `200`: Success
- `201`: Created (for POST creating resources)
- `400`: Bad request (invalid parameters)
- `404`: Resource not found
- `409`: Conflict (e.g., another build already running)
- `500`: Server error

Error responses follow this format:
```json
{
  "error": "Error message describing what went wrong"
}
```

---

## Tips for AI Agents

1. **Always get the API URL first** before making any API calls
2. **Poll build status** every 5-10 seconds when waiting for builds
3. **Check release status is "success"** before attempting to deploy
4. **Use pod describe** for debugging failing pods - it shows events
5. **Build logs are available per-stage** - check stages first to find failures
6. **Commit SHA must be at least 7 characters** when creating releases (source apps only)
7. **External image apps** skip the build step — releases are created with status `success` immediately, ready to deploy
8. **Check app's `build_type`** to determine if it's `"external-image"` or a source build app
