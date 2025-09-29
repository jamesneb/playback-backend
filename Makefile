# Playback Backend Development Makefile

.PHONY: help setup-local start-local stop-local clean-local logs test build docs docs-swagger deploy build-env release migrate

# Default environment
ENV ?= local

# Docker Compose file based on environment
COMPOSE_FILE = environments/$(ENV)/docker-compose.yml

help: ## Show this help message
	@echo 'Usage: make [target] [ENV=environment]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Environments: local, dev, staging, prod'

setup-local: ## Set up local development environment
	@echo "Setting up local development environment..."
	@mkdir -p environments/local/clickhouse
	@mkdir -p bin
	@mkdir -p config/environments
	@mkdir -p infrastructure/terraform
	@docker network create telemetry_network 2>/dev/null || true
	@echo "✅ Local environment setup complete"

start-local: setup-local ## Start local development stack
	@echo "Starting local development stack..."
	@docker-compose -f $(COMPOSE_FILE) up -d
	@echo "⏳ Waiting for services to be ready..."
	@sleep 10
	@echo "✅ Local stack is running!"
	@echo ""
	@echo "🎯 Services available at:"
	@echo "   Playback Backend:  http://localhost:8080"
	@echo "   Order Service:     http://localhost:8081"
	@echo "   ClickHouse HTTP:   http://localhost:8123"
	@echo "   Redis:             localhost:6379"
	@echo "   LocalStack:        http://localhost:4566"
	@echo ""
	@echo "📊 Admin interfaces:"
	@echo "   ClickHouse Web:    http://localhost:8123/play"
	@echo "   LocalStack Web:    http://localhost:4566/_localstack/health"

stop-local: ## Stop local development stack
	@echo "Stopping local development stack..."
	@docker-compose -f $(COMPOSE_FILE) down
	@echo "✅ Local stack stopped"

restart-local: stop-local start-local ## Restart local development stack

clean-local: ## Clean local development environment (removes data)
	@echo "⚠️  This will remove all local data. Are you sure? [y/N]" && read ans && [ $${ans:-N} = y ]
	@docker-compose -f $(COMPOSE_FILE) down -v
	@docker system prune -f
	@echo "✅ Local environment cleaned"

logs: ## Show logs for all services (ENV=local by default)
	@docker-compose -f $(COMPOSE_FILE) logs -f

logs-service: ## Show logs for specific service: make logs-service SERVICE=clickhouse
	@docker-compose -f $(COMPOSE_FILE) logs -f $(SERVICE)

shell-clickhouse: ## Open ClickHouse client shell
	@docker-compose -f $(COMPOSE_FILE) exec clickhouse clickhouse-client -u admin --password admin123

shell-redis: ## Open Redis client shell
	@docker-compose -f $(COMPOSE_FILE) exec redis redis-cli -a redis123

health: ## Check health of all services
	@echo "🔍 Checking service health..."
	@curl -s http://localhost:8080/health | jq '.' || echo "❌ Playback Backend not ready"
	@curl -s http://localhost:8081/health | jq '.' || echo "❌ Order Service not ready"
	@curl -s "http://localhost:8123/ping" && echo "✅ ClickHouse ready" || echo "❌ ClickHouse not ready"
	@redis-cli -h localhost -p 6379 -a redis123 ping && echo "✅ Redis ready" || echo "❌ Redis not ready"
	@curl -s http://localhost:4566/_localstack/health | jq '.' || echo "❌ LocalStack not ready"

docs-swagger: ## Generate Swagger documentation with environment-specific values
	@echo "📚 Generating Swagger docs for $(ENV) environment..."
	@if [ -f "config/environments/$(ENV).yaml" ]; then \
		VERSION=$$(yq '.app.version' config/environments/$(ENV).yaml) && \
		HOST=$$(yq '.server.host' config/environments/$(ENV).yaml):$$(yq '.server.port' config/environments/$(ENV).yaml) && \
		swag init -g cmd/server/main.go --parseDependency --parseInternal --templateVars "Version=$$VERSION,Host=$$HOST"; \
	else \
		swag init -g cmd/server/main.go --parseDependency --parseInternal; \
	fi
	@echo "✅ Swagger docs generated"

test-load: ## Run comprehensive load test against playback backend
	@echo "🚀 Running comprehensive load test..."
	@cd test/load && go run main.go

test-load-quick: ## Run quick load test (30s duration, 50 RPS)
	@echo "🚀 Running quick load test..."
	@cd test/load && LOAD_TEST_DURATION=30s LOAD_TEST_TARGET_RPS=50 go run main.go

test-load-stress: ## Run stress test (10m duration, 200 RPS)
	@echo "🚀 Running stress load test..."
	@cd test/load && LOAD_TEST_DURATION=10m LOAD_TEST_TARGET_RPS=200 LOAD_TEST_MAX_CONCURRENCY=100 go run main.go

test-chaos: ## Run chaos engineering tests
	@echo "🌪️  Running chaos engineering tests..."
	@cd test/chaos && go run main.go

test-chaos-quick: ## Run quick chaos tests (1m intervals, 2m max time)
	@echo "🌪️  Running quick chaos tests..."
	@cd test/chaos && CHAOS_EXPERIMENT_INTERVAL=1m CHAOS_MAX_EXPERIMENT_TIME=2m go run main.go

test-resilience: test-load test-chaos ## Run full resilience test suite (load + chaos)
	@echo "🛡️  Full resilience testing completed!"

test-integration: ## Run AWS integration tests (requires AWS setup)
	@echo "🔗 Running AWS integration tests..."
	@cd test/integration && source .env 2>/dev/null || echo "⚠️  .env file not found, using environment variables"
	@cd test/integration && go test -v -timeout 10m

test-integration-setup: ## Set up AWS resources for integration tests
	@echo "🚀 Setting up AWS resources for integration tests..."
	@test/integration/setup_aws_resources.sh

test-integration-cleanup: ## Clean up AWS resources after integration tests
	@echo "🧹 Cleaning up AWS integration test resources..."
	@test/integration/cleanup_aws_resources.sh

test-full: test test-load test-chaos test-integration ## Run all tests (unit, load, chaos, integration)
	@echo "🎯 All testing suites completed successfully!"

init-terraform: ## Initialize Terraform for environment (ENV=dev|staging|prod)
	@echo "Initializing Terraform for $(ENV)..."
	@cd infrastructure/terraform/environments/$(ENV) && terraform init
	@echo "✅ Terraform initialized for $(ENV)"

plan-terraform: ## Plan Terraform changes for environment
	@echo "Planning Terraform changes for $(ENV)..."
	@cd infrastructure/terraform/environments/$(ENV) && terraform plan -var-file="../../../environments/$(ENV)/terraform.tfvars"

apply-terraform: ## Apply Terraform changes for environment
	@echo "⚠️  This will apply Terraform changes to $(ENV). Are you sure? [y/N]" && read ans && [ $${ans:-N} = y ]
	@cd infrastructure/terraform/environments/$(ENV) && terraform apply -var-file="../../../environments/$(ENV)/terraform.tfvars"

destroy-terraform: ## Destroy Terraform infrastructure for environment
	@echo "⚠️  This will DESTROY all infrastructure in $(ENV). Are you sure? [y/N]" && read ans && [ $${ans:-N} = y ]
	@cd infrastructure/terraform/environments/$(ENV) && terraform destroy -var-file="../../../environments/$(ENV)/terraform.tfvars"

build: ## Build the application
	@echo "Building playback-backend..."
	@mkdir -p bin
	@go build -o bin/playback-backend ./cmd/server
	@echo "✅ Build complete"

build-all: lint test build docs docker-build ## Complete build pipeline (lint, test, build, docs, docker)
	@echo "🎉 Complete build pipeline finished successfully!"

docker-build: ## Build Docker image for current environment
	@echo "Building Docker image..."
	@docker build -t playback-backend:latest -f deployments/Dockerfile .
	@echo "✅ Docker image built"

docker-build-env: ## Build Docker image for specific environment (ENV=dev|staging|prod)
	@echo "Building Docker image for $(ENV) environment..."
	@docker build -t playback-backend:$(ENV) -f deployments/Dockerfile --build-arg ENV=$(ENV) .
	@echo "✅ Docker image built for $(ENV)"

docs: ## Generate all documentation (API docs and Swagger)
	@echo "Generating all documentation..."
	@mkdir -p docs/generated
	@go doc -all ./... > docs/generated/api-docs.txt 2>/dev/null || echo "No docs generated"
	@$(MAKE) docs-swagger
	@echo "✅ All documentation generated"

test: ## Run all unit tests with coverage
	@echo "Running comprehensive test suite..."
	@./test.sh
	@echo "✅ Tests complete"

test-verbose: ## Run tests with verbose output
	@go test -v ./...

test-coverage: ## Run tests with coverage report
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: ## Run tests with race detection
	@go test -race ./...

test-bench: ## Run benchmarks
	@./test.sh --bench

test-package: ## Run tests for specific package
	@read -p "Enter package path: " pkg; ./test.sh --package $$pkg

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run
	@echo "✅ Linting complete"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

mod-tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "✅ Modules tidied"

# Deployment shortcut (use ENV=dev|staging|prod)
deploy: ## Deploy to specified environment (ENV=dev|staging|prod)
	@if [ -z "$(ENV)" ] || [ "$(ENV)" = "local" ]; then echo "Error: ENV must be dev, staging, or prod"; exit 1; fi
	@$(MAKE) apply-terraform ENV=$(ENV)

# Utility commands
create-env: ## Create new environment files (ENV=name required)
	@if [ -z "$(ENV)" ]; then echo "Error: ENV is required. Usage: make create-env ENV=myenv"; exit 1; fi
	@mkdir -p environments/$(ENV)
	@mkdir -p infrastructure/terraform/environments/$(ENV)
	@cp environments/local/.env.local environments/$(ENV)/.env.$(ENV)
	@echo "✅ Environment $(ENV) created. Don't forget to update the configuration files!"

backup-local: ## Backup local data
	@echo "Creating backup of local data..."
	@mkdir -p backups/$(shell date +%Y%m%d_%H%M%S)
	@docker-compose -f $(COMPOSE_FILE) exec -T clickhouse clickhouse-client -u admin --password admin123 --query "BACKUP DATABASE telemetry TO File('/var/lib/clickhouse/backups/backup.zip')"
	@echo "✅ Backup created"

# Database migration targets
migrate: ## Run database migrations for specified environment (ENV=local|dev|prod)
	@echo "Running database migrations for $(ENV) environment..."
	@if [ ! -f "db/scripts/migrate.go" ]; then echo "⚠️  Migration script not found"; exit 1; fi
	@ENV=$(ENV) go run db/scripts/migrate.go

verify-migrations: ## Verify migrations are idempotent and correct
	@echo "Verifying database migrations..."
	@./db/scripts/verify.sh

# Development helpers
generate-data: ## Generate test data
	@echo "Generating test data..."
	@echo "⚠️  Test data generation not implemented yet"

clean-build: ## Clean build artifacts
	@rm -rf bin/
	@go clean
	@echo "✅ Build artifacts cleaned"

install-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	@go mod download
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	@which jq > /dev/null || (echo "Please install jq: https://stedolan.github.io/jq/" && exit 1)
	@echo "✅ Dependencies installed"

# Environment-specific build targets
build-env: ## Build for specific environment (ENV=local|dev|staging|prod)
	@$(MAKE) build-all ENV=$(ENV)
	@if [ "$(ENV)" = "local" ]; then \
		$(MAKE) start-local; \
	else \
		$(MAKE) docker-build-env ENV=$(ENV); \
	fi

# CI/CD Pipeline targets
ci: install-deps lint test ## CI pipeline: install deps, lint, test
	@echo "✅ CI pipeline completed"

cd: build-all ## CD pipeline: complete build including docs and docker
	@echo "✅ CD pipeline completed"

pre-commit: fmt lint test ## Pre-commit checks
	@echo "✅ Pre-commit checks passed"

# Release target
release: ## Release to specified environment (ENV=local|dev|staging|prod)
	@$(MAKE) build-env ENV=$(ENV)
	@if [ "$(ENV)" != "local" ]; then \
		$(MAKE) deploy ENV=$(ENV); \
	fi
	@echo "✅ Released to $(ENV) environment"