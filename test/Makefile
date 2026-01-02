.PHONY: help test-unit test-integration test-system test-performance test-all test-coverage clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Database setup for tests
setup-test-db: ## Setup test database
	@echo "Setting up test database..."
	docker run --name cicd-test-db -d \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=cicd_test \
		-p 5433:5432 \
		postgres:15-alpine || docker start cicd-test-db
	@echo "Waiting for database to be ready..."
	@for i in {1..30}; do \
		if docker exec cicd-test-db pg_isready -U postgres > /dev/null 2>&1; then \
			echo "Database is ready!"; \
			break; \
		fi; \
		sleep 1; \
	done

teardown-test-db: ## Teardown test database
	@echo "Stopping test database..."
	docker stop cicd-test-db || true
	docker rm cicd-test-db || true

# Unit tests
test-unit: ## Run unit tests for backend and frontend
	@echo "Running backend unit tests..."
	cd .. && go test -v -race -short ./backend/internal/...
	
	@echo "Running frontend unit tests..."
	cd ../frontend && npm test -- --coverage --watchAll=false

test-unit-backend: ## Run only backend unit tests
	@echo "Running backend unit tests..."
	cd .. && go test -v -race -short ./backend/internal/...

test-unit-frontend: ## Run only frontend unit tests
	@echo "Running frontend unit tests..."
	cd ../frontend && npm test -- --coverage --watchAll=false

# Integration tests
test-integration: setup-test-db ## Run integration tests
	@echo "Running integration tests..."
	cd .. && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v -tags=integration ./backend/test/...

# System tests
test-system: setup-test-db ## Run system tests
	@echo "Running system tests..."
	cd .. && SYSTEM_TEST=1 \
		TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v ./test/...

# Performance tests
test-performance: setup-test-db ## Run performance and load tests
	@echo "Running performance tests..."
	cd .. && LOAD_TEST=1 SYSTEM_TEST=1 \
		TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v -timeout=10m ./test/... -run TestSystemLoadTest

# Coverage tests
test-coverage: setup-test-db ## Run tests with coverage
	@echo "Running tests with coverage..."
	cd .. && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v -race -coverprofile=coverage.out ./...
	cd .. && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Benchmark tests
test-benchmark: ## Run benchmark tests
	@echo "Running benchmark tests..."
	cd .. && go test -v -bench=. -benchmem ./backend/test/...

# Smoke tests (quick validation)
test-smoke: ## Run quick smoke tests
	@echo "Running smoke tests..."
	cd .. && go test -v -short -run TestSystemHealthCheck ./test/...

# Security tests
test-security: ## Run security tests
	@echo "Running security scan..."
	cd .. && gosec ./...
	cd ../frontend && npm audit --audit-level=moderate

# All tests
test-all: setup-test-db ## Run all tests
	@echo "Running all tests..."
	$(MAKE) test-unit
	$(MAKE) test-integration
	$(MAKE) test-system
	@echo "All tests completed!"

# Test with real external services (GitHub API, etc)
test-real-api: ## Run tests with real external APIs
	@echo "Running tests with real APIs..."
	cd .. && SKIP_REAL_API_TESTS="" go test -v ./backend/internal/git/...

# Stress tests
test-stress: setup-test-db ## Run stress tests
	@echo "Running stress tests..."
	cd .. && LOAD_TEST=1 SYSTEM_TEST=1 \
		TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v -timeout=20m ./test/... -count=5

# Clean up
clean: teardown-test-db ## Clean up test artifacts
	@echo "Cleaning up test artifacts..."
	cd .. && rm -f coverage.out coverage.html
	cd ../frontend && rm -rf coverage/
	@echo "Cleanup completed!"

# Continuous testing (watch mode)
test-watch: ## Run tests in watch mode (requires entr)
	@echo "Starting test watch mode..."
	find .. -name "*.go" | entr -c make test-unit-backend

# Test specific package
test-package: ## Test specific package (usage: make test-package PKG=./internal/api)
	@echo "Testing package: $(PKG)"
	cd .. && go test -v $(PKG)

# Generate test data
generate-test-data: setup-test-db ## Generate test data for manual testing
	@echo "Generating test data..."
	cd .. && go run scripts/generate_test_data.go

# Docker-based testing
test-docker: ## Run tests in Docker environment
	@echo "Running tests in Docker..."
	docker-compose -f ../docker-compose.test.yml up --build --abort-on-container-exit
	docker-compose -f ../docker-compose.test.yml down

# Test database migrations
test-migrations: setup-test-db ## Test database migrations
	@echo "Testing database migrations..."
	cd .. && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v ./backend/internal/db/...

# API contract testing
test-api-contract: ## Test API contracts
	@echo "Running API contract tests..."
	# Could integrate with tools like Pact or OpenAPI validators
	@echo "API contract testing placeholder"

# Report generation
generate-reports: ## Generate test reports
	@echo "Generating test reports..."
	cd .. && go test -v -json ./... > test-results.json
	# Convert to JUnit format if needed
	@echo "Test reports generated"

# Test environment setup
setup-test-env: ## Setup complete test environment
	$(MAKE) setup-test-db
	@echo "Installing test dependencies..."
	cd .. && go mod download
	cd ../frontend && npm ci
	@echo "Test environment ready!"

teardown-test-env: ## Teardown test environment
	$(MAKE) teardown-test-db
	$(MAKE) clean

# Quick development test
test-dev: ## Quick test for development (unit tests only)
	@echo "Running quick development tests..."
	cd .. && go test -short ./backend/internal/...

# Test with race condition detection
test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	cd .. && go test -race -short ./...

# Memory leak tests
test-memory: ## Run tests checking for memory leaks
	@echo "Running memory leak tests..."
	cd .. && go test -v -memprofile=mem.prof ./backend/test/...
	cd .. && go tool pprof mem.prof

# Database-specific tests
test-database: setup-test-db ## Test database operations specifically
	@echo "Running database-specific tests..."
	cd .. && TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5433/cicd_test?sslmode=disable \
		go test -v ./backend/internal/models/...

# Test configuration
test-config: ## Test configuration loading
	@echo "Testing configuration..."
	cd .. && go test -v ./backend/pkg/config/...

# Parallel test execution
test-parallel: setup-test-db ## Run tests in parallel
	@echo "Running tests in parallel..."
	cd .. && go test -v -parallel=4 ./...

# Test artifacts collection
collect-artifacts: ## Collect test artifacts
	@echo "Collecting test artifacts..."
	mkdir -p artifacts
	cp ../coverage.html artifacts/ 2>/dev/null || true
	cp -r ../frontend/coverage artifacts/frontend-coverage 2>/dev/null || true
	cp ../test-results.json artifacts/ 2>/dev/null || true
	@echo "Artifacts collected in ./artifacts/"

# Example usage targets
example-usage: ## Show example usage
	@echo "Example test commands:"
	@echo ""
	@echo "  make test-dev          # Quick tests for development"
	@echo "  make test-all          # Complete test suite"
	@echo "  make test-coverage     # Tests with coverage report"
	@echo "  make test-performance  # Performance and load tests"
	@echo "  make test-package PKG=./internal/api  # Test specific package"
	@echo ""