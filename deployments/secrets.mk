# Secrets Management Makefile
# High-performance secrets management for the Playback Backend

# Configuration
SECRETS_CLI := ./cmd/secrets/secrets
SECRETS_CONFIG := ./config/secrets.yaml
ENV ?= local

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: secrets-help secrets-build secrets-init secrets-validate secrets-list secrets-rotate secrets-backup

secrets-help: ## Display secrets management commands
	@echo "$(GREEN)Secrets Management Commands:$(NC)"
	@echo "  $(GREEN)secrets-build$(NC)      Build secrets CLI tool"
	@echo "  $(GREEN)secrets-init$(NC)       Initialize secrets for current environment"
	@echo "  $(GREEN)secrets-validate$(NC)   Validate all required secrets are present"
	@echo "  $(GREEN)secrets-list$(NC)       List all secret keys"
	@echo "  $(GREEN)secrets-get$(NC)        Get a specific secret value"
	@echo "  $(GREEN)secrets-set$(NC)        Set a specific secret value"
	@echo "  $(GREEN)secrets-rotate$(NC)     Rotate a specific secret"
	@echo "  $(GREEN)secrets-backup$(NC)     Backup secrets configuration"
	@echo "  $(GREEN)secrets-restore$(NC)    Restore secrets from backup"
	@echo "  $(GREEN)secrets-clean$(NC)      Clean local secrets cache"
	@echo ""
	@echo "$(YELLOW)Environment Variables:$(NC)"
	@echo "  ENV=local|staging|production  Set target environment (default: local)"
	@echo "  SECRETS_PROVIDER=aws|file|vault  Override secrets provider"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make secrets-init ENV=local"
	@echo "  make secrets-validate ENV=production"
	@echo "  make secrets-get KEY=CLICKHOUSE_PASSWORD"
	@echo "  make secrets-set KEY=JWT_SECRET VALUE=newsecret"

secrets-build: ## Build the secrets CLI tool
	@echo "$(YELLOW)Building secrets CLI tool...$(NC)"
	@cd cmd/secrets && go build -o secrets main.go
	@echo "$(GREEN)Secrets CLI built successfully$(NC)"

secrets-init: secrets-build ## Initialize secrets for current environment
	@echo "$(YELLOW)Initializing secrets for $(ENV) environment...$(NC)"
	@$(SECRETS_CLI) init --env $(ENV) --config $(SECRETS_CONFIG)
	@echo "$(GREEN)Secrets initialized successfully$(NC)"

secrets-validate: secrets-build ## Validate all required secrets are present
	@echo "$(YELLOW)Validating secrets for $(ENV) environment...$(NC)"
	@$(SECRETS_CLI) validate --env $(ENV) --config $(SECRETS_CONFIG)

secrets-list: secrets-build ## List all secret keys
	@echo "$(YELLOW)Listing secrets for $(ENV) environment...$(NC)"
	@$(SECRETS_CLI) list --env $(ENV) --config $(SECRETS_CONFIG)

secrets-get: secrets-build ## Get a specific secret value (usage: make secrets-get KEY=SECRET_NAME)
	@if [ -z "$(KEY)" ]; then \
		echo "$(RED)Error: KEY parameter required$(NC)"; \
		echo "Usage: make secrets-get KEY=SECRET_NAME"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Getting secret: $(KEY)$(NC)"
	@$(SECRETS_CLI) get $(KEY) --env $(ENV) --config $(SECRETS_CONFIG)

secrets-set: secrets-build ## Set a specific secret value (usage: make secrets-set KEY=SECRET_NAME VALUE=secret_value)
	@if [ -z "$(KEY)" ] || [ -z "$(VALUE)" ]; then \
		echo "$(RED)Error: Both KEY and VALUE parameters required$(NC)"; \
		echo "Usage: make secrets-set KEY=SECRET_NAME VALUE=secret_value"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Setting secret: $(KEY)$(NC)"
	@$(SECRETS_CLI) set $(KEY) $(VALUE) --env $(ENV) --config $(SECRETS_CONFIG)
	@echo "$(GREEN)Secret set successfully$(NC)"

secrets-generate: secrets-build ## Generate a secure secret (usage: make secrets-generate KEY=SECRET_NAME)
	@if [ -z "$(KEY)" ]; then \
		echo "$(RED)Error: KEY parameter required$(NC)"; \
		echo "Usage: make secrets-generate KEY=SECRET_NAME"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Generating secure secret: $(KEY)$(NC)"
	@$(SECRETS_CLI) set $(KEY) --generate --env $(ENV) --config $(SECRETS_CONFIG)

secrets-rotate: secrets-build ## Rotate a specific secret (usage: make secrets-rotate KEY=SECRET_NAME)
	@if [ -z "$(KEY)" ]; then \
		echo "$(RED)Error: KEY parameter required$(NC)"; \
		echo "Usage: make secrets-rotate KEY=SECRET_NAME"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Rotating secret: $(KEY)$(NC)"
	@$(SECRETS_CLI) rotate $(KEY) --env $(ENV) --config $(SECRETS_CONFIG)
	@echo "$(GREEN)Secret rotated successfully$(NC)"

secrets-dev-setup: ## Quick development setup
	@echo "$(YELLOW)Setting up development secrets...$(NC)"
	@$(MAKE) secrets-build
	@ENV=local $(MAKE) secrets-init
	@ENV=local $(MAKE) secrets-validate
	@echo "$(GREEN)Development secrets setup complete$(NC)"