.PHONY: help dev build deploy clean test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Start development environment
	docker-compose up --build

dev-down: ## Stop development environment
	docker-compose down

dev-clean: ## Clean development environment
	docker-compose down -v --remove-orphans

build: ## Build container images
	docker build -t cicd-platform/backend:latest ./backend
	docker build -t cicd-platform/frontend:latest ./frontend

build-push: build ## Build and push container images
	docker tag cicd-platform/backend:latest $(REGISTRY)/cicd-platform/backend:$(TAG)
	docker tag cicd-platform/frontend:latest $(REGISTRY)/cicd-platform/frontend:$(TAG)
	docker push $(REGISTRY)/cicd-platform/backend:$(TAG)
	docker push $(REGISTRY)/cicd-platform/frontend:$(TAG)

deploy-k8s: ## Deploy to Kubernetes
	kubectl apply -f deployments/namespace.yaml
	kubectl apply -f tekton/rbac.yaml
	kubectl apply -f tekton/pipeline.yaml
	kubectl apply -f deployments/postgresql.yaml
	kubectl apply -f deployments/backend.yaml
	kubectl apply -f deployments/frontend.yaml
	kubectl apply -f deployments/ingress.yaml

undeploy-k8s: ## Remove from Kubernetes
	kubectl delete -f deployments/ --ignore-not-found=true
	kubectl delete -f tekton/ --ignore-not-found=true

logs-backend: ## Show backend logs
	kubectl logs -f -n cicd-system deployment/cicd-backend

logs-frontend: ## Show frontend logs
	kubectl logs -f -n cicd-system deployment/cicd-frontend

test-backend: ## Run backend tests
	cd backend && go test ./...

test-frontend: ## Run frontend tests
	cd frontend && npm test

lint-backend: ## Lint backend code
	cd backend && go vet ./...
	cd backend && golangci-lint run

lint-frontend: ## Lint frontend code
	cd frontend && npm run lint

# Setup targets
setup-tekton: ## Install Tekton Pipelines
	kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

setup-git-secret: ## Create Git credentials secret
	@read -p "Git Username: " GIT_USER; \
	read -s -p "Git Token: " GIT_TOKEN; \
	echo; \
	kubectl create secret generic cicd-git-secret \
	  --namespace=cicd-runners \
	  --from-literal=username=$$GIT_USER \
	  --from-literal=password=$$GIT_TOKEN \
	  --type=kubernetes.io/basic-auth

setup-registry-secret: ## Create registry credentials secret
	@read -p "Registry Server: " REG_SERVER; \
	read -p "Registry Username: " REG_USER; \
	read -s -p "Registry Password: " REG_PASS; \
	echo; \
	kubectl create secret docker-registry cicd-registry-secret \
	  --namespace=cicd-runners \
	  --docker-server=$$REG_SERVER \
	  --docker-username=$$REG_USER \
	  --docker-password=$$REG_PASS

# Local development targets (non-docker)
dev-local-db: ## Start only PostgreSQL database for local development
	docker-compose up postgres -d

dev-local-db-wait: dev-local-db ## Start database and wait for it to be ready
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker-compose exec postgres pg_isready -U postgres > /dev/null 2>&1; do \
		echo "PostgreSQL is not ready yet, waiting..."; \
		sleep 2; \
	done
	@echo "PostgreSQL is ready!"

dev-local-backend: ## Start backend locally (requires PostgreSQL running)
	@echo "Starting backend on http://localhost:8080"
	cd backend && DATABASE_HOST=localhost go run cmd/main.go

dev-local-frontend: ## Start frontend locally
	@echo "Installing frontend dependencies and starting on http://localhost:3000"
	cd frontend && npm install && npm start

dev-local: dev-local-db-wait ## Start local development (non-docker)
	@echo "Local development environment setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "1. In terminal 1: make dev-local-backend"
	@echo "2. In terminal 2: make dev-local-frontend"
	@echo ""
	@echo "URLs:"
	@echo "- Backend API: http://localhost:8080"
	@echo "- Frontend:    http://localhost:3000"
	@echo "- Database:    postgresql://postgres:password@localhost:5432/cicd"

dev-local-stop: ## Stop local development database
	docker-compose stop postgres

dev-local-clean: ## Clean local development database
	docker-compose down postgres -v

# Go development helpers
go-deps: ## Download Go dependencies
	cd backend && go mod download

go-build: ## Build Go binary
	cd backend && go build -o bin/cicd-platform cmd/main.go

go-run: ## Run Go application with live reload (requires air)
	cd backend && air

# Frontend development helpers
npm-install: ## Install frontend dependencies
	cd frontend && npm install

npm-build: ## Build frontend for production
	cd frontend && npm run build

# Combined local development
install-deps: go-deps npm-install ## Install all dependencies

# Default values
REGISTRY ?= harbor.company.com
TAG ?= latest