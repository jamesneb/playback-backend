# Playback Backend Development Makefile

.PHONY: help setup-local start-local stop-local clean-local logs test build deploy-dev deploy-staging deploy-prod

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
	@mkdir -p scripts/data
	@mkdir -p config/environments
	@mkdir -p infrastructure/terraform
	@mkdir -p deployments/lambda
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

test-load: ## Run load test against order service
	@echo "🚀 Running load test..."
	@cd examples/load-test && go run main.go -url http://localhost:8081 -rps 10 -duration 30s

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
	@go build -o bin/playback-backend ./cmd/server
	@echo "✅ Build complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...
	@echo "✅ Tests complete"

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

# AWS deployment shortcuts
deploy-dev: ## Deploy to development environment
	@$(MAKE) apply-terraform ENV=dev

deploy-staging: ## Deploy to staging environment
	@$(MAKE) apply-terraform ENV=staging

deploy-prod: ## Deploy to production environment
	@$(MAKE) apply-terraform ENV=prod

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

# Development helpers
generate-data: ## Generate test data
	@echo "Generating test data..."
	@cd scripts/data && go run generate-test-data.go
	@echo "✅ Test data generated"

clean-build: ## Clean build artifacts
	@rm -rf bin/
	@go clean
	@echo "✅ Build artifacts cleaned"

install-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	@go mod download
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@which jq > /dev/null || (echo "Please install jq: https://stedolan.github.io/jq/" && exit 1)
	@echo "✅ Dependencies installed"