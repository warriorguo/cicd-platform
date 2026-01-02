# Deployment Guide

This document provides detailed instructions for deploying the CI/CD platform in different environments.

## Prerequisites

### Kubernetes Cluster
- Kubernetes 1.24+
- kubectl configured with cluster admin access
- At least 4 CPU cores and 8GB RAM available
- StorageClass for persistent volumes (for PostgreSQL)

### Required Tools
- kubectl
- docker (for building images)
- make (for running deployment commands)

## Installation Steps

### 1. Install Tekton Pipelines

```bash
# Install latest Tekton Pipelines
kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# Wait for installation to complete
kubectl wait --for=condition=Ready pods --all -n tekton-pipelines --timeout=300s

# Verify installation
kubectl get pods -n tekton-pipelines
```

### 2. Create Namespaces

```bash
kubectl apply -f deployments/namespace.yaml
```

### 3. Set up RBAC

```bash
kubectl apply -f tekton/rbac.yaml
```

### 4. Create Secrets

#### Git Credentials
For private repositories, create a secret with Git credentials:

```bash
# Interactive setup
make setup-git-secret

# Or manual setup
kubectl create secret generic cicd-git-secret \
  --namespace=cicd-runners \
  --from-literal=username=your-git-username \
  --from-literal=password=your-git-token \
  --type=kubernetes.io/basic-auth
```

#### Registry Credentials
For private container registries:

```bash
# Interactive setup
make setup-registry-secret

# Or manual setup
kubectl create secret docker-registry cicd-registry-secret \
  --namespace=cicd-runners \
  --docker-server=your-registry.com \
  --docker-username=your-username \
  --docker-password=your-password
```

### 5. Deploy Database

```bash
kubectl apply -f deployments/postgresql.yaml

# Wait for database to be ready
kubectl wait --for=condition=Ready pod -l app=postgresql -n cicd-system --timeout=300s
```

### 6. Deploy Application Components

```bash
# Deploy backend
kubectl apply -f deployments/backend.yaml

# Deploy frontend
kubectl apply -f deployments/frontend.yaml

# Deploy ingress (optional)
kubectl apply -f deployments/ingress.yaml
```

### 7. Install Tekton Tasks and Pipelines

```bash
kubectl apply -f tekton/pipeline.yaml
```

### 8. Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n cicd-system
kubectl get pods -n cicd-runners

# Check services
kubectl get svc -n cicd-system
```

## Access the Application

### Option 1: Port Forward (Development)
```bash
kubectl port-forward -n cicd-system svc/cicd-frontend 8080:80
```
Access at: http://localhost:8080

### Option 2: NodePort (Local Cluster)
```bash
# Get NodePort
kubectl get svc cicd-frontend-nodeport -n cicd-system

# Access via node IP and port
curl http://<node-ip>:30080
```

### Option 3: Ingress (Production)
Configure your DNS to point to the ingress controller and access via:
http://cicd.local (or your configured domain)

## Environment-Specific Configurations

### Development Environment

For development, you can use Docker Compose instead of Kubernetes:

```bash
# Start all services locally
make dev

# Access the application
open http://localhost:3000
```

### Staging Environment

For staging, update the ingress configuration:

```yaml
# deployments/ingress.yaml
spec:
  rules:
  - host: cicd-staging.company.com
    http:
      paths: ...
```

### Production Environment

For production, consider these additional configurations:

#### 1. Resource Limits
Update resource limits in deployment files:

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

#### 2. High Availability
Scale deployments for high availability:

```bash
kubectl scale deployment cicd-backend --replicas=3 -n cicd-system
kubectl scale deployment cicd-frontend --replicas=3 -n cicd-system
```

#### 3. External Database
For production, use an external PostgreSQL database:

```yaml
# Update backend deployment
env:
- name: DATABASE_HOST
  value: "production-db.company.com"
```

#### 4. TLS/SSL
Configure TLS for ingress:

```yaml
# deployments/ingress.yaml
spec:
  tls:
  - hosts:
    - cicd.company.com
    secretName: cicd-tls-secret
```

#### 5. Monitoring and Logging
Add monitoring labels and configure log aggregation:

```yaml
metadata:
  labels:
    app: cicd-platform
    component: backend
    version: v1.0.0
```

## Backup and Recovery

### Database Backup
```bash
# Create database backup
kubectl exec -n cicd-system deployment/postgresql -- pg_dump -U postgres cicd > backup.sql

# Restore database
kubectl exec -i -n cicd-system deployment/postgresql -- psql -U postgres cicd < backup.sql
```

### Configuration Backup
```bash
# Backup all configurations
kubectl get all,secrets,configmaps -n cicd-system -o yaml > cicd-backup.yaml
kubectl get all,secrets,configmaps -n cicd-runners -o yaml > cicd-runners-backup.yaml
```

## Troubleshooting

### Common Issues

#### 1. Backend Not Starting
```bash
# Check logs
kubectl logs -f -n cicd-system deployment/cicd-backend

# Check database connectivity
kubectl exec -n cicd-system deployment/cicd-backend -- nslookup postgresql
```

#### 2. Pipeline Failures
```bash
# List pipeline runs
kubectl get pipelineruns -n cicd-runners

# Check specific pipeline run
kubectl describe pipelinerun <name> -n cicd-runners

# Check task run logs
kubectl logs <taskrun-pod> -n cicd-runners
```

#### 3. Image Pull Errors
```bash
# Check registry secret
kubectl get secret cicd-registry-secret -n cicd-runners -o yaml

# Test registry connectivity
kubectl run test-pod --image=alpine --rm -it -- wget -O- https://your-registry.com/v2/
```

#### 4. Permission Errors
```bash
# Check service account permissions
kubectl auth can-i create pipelineruns --as=system:serviceaccount:cicd-runners:cicd-pipeline -n cicd-runners

# Check RBAC
kubectl describe clusterrolebinding cicd-pipeline-binding
```

### Health Checks

```bash
# Backend health
kubectl exec -n cicd-system deployment/cicd-backend -- curl localhost:8080/health

# Database health
kubectl exec -n cicd-system deployment/postgresql -- pg_isready -U postgres

# Frontend health
kubectl exec -n cicd-system deployment/cicd-frontend -- curl localhost:80/
```

## Scaling

### Horizontal Scaling
```bash
# Scale backend
kubectl scale deployment cicd-backend --replicas=3 -n cicd-system

# Scale frontend
kubectl scale deployment cicd-frontend --replicas=5 -n cicd-system
```

### Resource Adjustment
```bash
# Update resource limits
kubectl patch deployment cicd-backend -n cicd-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"backend","resources":{"limits":{"cpu":"1000m","memory":"1Gi"}}}]}}}}'
```

### Database Scaling
For database scaling, consider:
- Read replicas for read operations
- Connection pooling (pgbouncer)
- External managed database services

## Security Hardening

### Network Policies
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cicd-network-policy
  namespace: cicd-system
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: cicd-system
```

### Pod Security Standards
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: cicd-system
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

### Image Security
- Use specific image tags (not :latest)
- Scan images for vulnerabilities
- Use minimal base images
- Configure image pull policies