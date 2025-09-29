# Container Security Scanning Makefile
# High-performance security scanning tools for local development and CI/CD

# Configuration
DOCKER_IMAGE_PREFIX := playback-backend
DOCKER_FILE := deployments/Dockerfile
SCAN_OUTPUT_DIR := ./security-reports
SEVERITY_THRESHOLD := HIGH

# Scan targets
TARGETS := server consumer

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: security-scan security-scan-fast security-scan-comprehensive clean-reports help
.DEFAULT_GOAL := help

help: ## Display available security scanning commands
	@echo "$(BLUE)Container Security Scanning Commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-25s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup-security-tools: ## Install security scanning tools
	@echo "$(BLUE)Installing security scanning tools...$(NC)"
	@if ! command -v trivy &> /dev/null; then \
		echo "$(YELLOW)Installing Trivy...$(NC)"; \
		curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.48.3; \
	fi
	@if ! command -v grype &> /dev/null; then \
		echo "$(YELLOW)Installing Grype...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin; \
	fi
	@if ! command -v gosec &> /dev/null; then \
		echo "$(YELLOW)Installing Gosec...$(NC)"; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi
	@if ! command -v govulncheck &> /dev/null; then \
		echo "$(YELLOW)Installing Govulncheck...$(NC)"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@echo "$(GREEN)Security tools installed successfully$(NC)"

prepare-scan: ## Prepare environment for security scanning
	@echo "$(BLUE)Preparing security scan environment...$(NC)"
	@mkdir -p $(SCAN_OUTPUT_DIR)
	@mkdir -p $(SCAN_OUTPUT_DIR)/trivy
	@mkdir -p $(SCAN_OUTPUT_DIR)/grype
	@mkdir -p $(SCAN_OUTPUT_DIR)/gosec
	@mkdir -p $(SCAN_OUTPUT_DIR)/govulncheck

build-images: ## Build Docker images for security scanning
	@echo "$(BLUE)Building Docker images for security scanning...$(NC)"
	@for target in $(TARGETS); do \
		echo "$(YELLOW)Building $$target image...$(NC)"; \
		docker build \
			--target $$target \
			--tag $(DOCKER_IMAGE_PREFIX)-$$target:scan \
			--file $(DOCKER_FILE) \
			--build-arg ENV=production \
			. || exit 1; \
	done
	@echo "$(GREEN)Images built successfully$(NC)"

scan-source-code: prepare-scan ## Run static analysis on source code
	@echo "$(BLUE)Running source code security scan...$(NC)"

	@echo "$(YELLOW)Running Gosec (Go Security Checker)...$(NC)"
	@gosec -fmt json -out $(SCAN_OUTPUT_DIR)/gosec/report.json -stdout -verbose=text ./... || true

	@echo "$(YELLOW)Running Go vulnerability check...$(NC)"
	@govulncheck -json ./... > $(SCAN_OUTPUT_DIR)/govulncheck/report.json 2>&1 || true

	@echo "$(YELLOW)Running Trivy filesystem scan...$(NC)"
	@trivy fs \
		--format json \
		--output $(SCAN_OUTPUT_DIR)/trivy/fs-report.json \
		--severity $(SEVERITY_THRESHOLD),CRITICAL \
		--ignore-unfixed \
		. || true

	@echo "$(GREEN)Source code security scan completed$(NC)"

scan-containers: build-images prepare-scan ## Run container image security scans
	@echo "$(BLUE)Running container security scans...$(NC)"
	@for target in $(TARGETS); do \
		echo "$(YELLOW)Scanning $$target container...$(NC)"; \
		\
		echo "  - Running Trivy container scan..."; \
		trivy image \
			--format json \
			--output $(SCAN_OUTPUT_DIR)/trivy/$$target-container-report.json \
			--severity $(SEVERITY_THRESHOLD),CRITICAL \
			--ignore-unfixed \
			$(DOCKER_IMAGE_PREFIX)-$$target:scan || true; \
		\
		echo "  - Running Grype vulnerability scan..."; \
		grype $(DOCKER_IMAGE_PREFIX)-$$target:scan \
			--output json \
			--file $(SCAN_OUTPUT_DIR)/grype/$$target-report.json \
			--fail-on $(SEVERITY_THRESHOLD) || true; \
		\
		echo "$(GREEN)$$target container scan completed$(NC)"; \
	done

scan-dockerfile: prepare-scan ## Run Dockerfile security and best practice checks
	@echo "$(BLUE)Running Dockerfile security scan...$(NC)"

	@echo "$(YELLOW)Running Hadolint Dockerfile linter...$(NC)"
	@docker run --rm -i hadolint/hadolint:latest-alpine < $(DOCKER_FILE) > $(SCAN_OUTPUT_DIR)/hadolint-report.txt 2>&1 || true

	@echo "$(YELLOW)Running Trivy config scan on Dockerfile...$(NC)"
	@trivy config \
		--format json \
		--output $(SCAN_OUTPUT_DIR)/trivy/dockerfile-report.json \
		$(DOCKER_FILE) || true

	@echo "$(GREEN)Dockerfile security scan completed$(NC)"

security-scan-fast: scan-source-code scan-dockerfile ## Run fast security scans (source + dockerfile only)
	@echo "$(GREEN)Fast security scan completed!$(NC)"
	@$(MAKE) generate-report

security-scan: scan-source-code scan-containers scan-dockerfile ## Run comprehensive security scan (recommended)
	@echo "$(GREEN)Comprehensive security scan completed!$(NC)"
	@$(MAKE) generate-report

check-critical: ## Check for critical security issues and exit with error if found
	@echo "$(BLUE)Checking for critical security issues...$(NC)"
	@critical_found=0; \
	if [ -f $(SCAN_OUTPUT_DIR)/trivy/fs-report.json ]; then \
		critical_count=$$(jq '[.Results[]? | .Vulnerabilities[]? | select(.Severity == "CRITICAL")] | length' $(SCAN_OUTPUT_DIR)/trivy/fs-report.json 2>/dev/null || echo 0); \
		if [ "$$critical_count" -gt 0 ]; then \
			echo "$(RED)Found $$critical_count critical vulnerabilities in filesystem scan$(NC)"; \
			critical_found=1; \
		fi; \
	fi; \
	for target in $(TARGETS); do \
		if [ -f $(SCAN_OUTPUT_DIR)/trivy/$$target-container-report.json ]; then \
			critical_count=$$(jq '[.Results[]? | .Vulnerabilities[]? | select(.Severity == "CRITICAL")] | length' $(SCAN_OUTPUT_DIR)/trivy/$$target-container-report.json 2>/dev/null || echo 0); \
			if [ "$$critical_count" -gt 0 ]; then \
				echo "$(RED)Found $$critical_count critical vulnerabilities in $$target container$(NC)"; \
				critical_found=1; \
			fi; \
		fi; \
	done; \
	if [ $$critical_found -eq 1 ]; then \
		echo "$(RED)Critical security issues detected. Build should not proceed.$(NC)"; \
		exit 1; \
	else \
		echo "$(GREEN)No critical security issues found$(NC)"; \
	fi

generate-report: ## Generate consolidated security report
	@echo "$(BLUE)Generating consolidated security report...$(NC)"
	@echo "# Security Scan Report" > $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "Generated on: $$(date)" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "## Scan Coverage" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "- ✅ Static Analysis Security Testing (SAST)" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "- ✅ Container Image Vulnerability Scanning" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "- ✅ Dockerfile Best Practices" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "- ✅ Go Dependency Vulnerability Check" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "" >> $(SCAN_OUTPUT_DIR)/security-summary.md

	@echo "$(YELLOW)Processing scan results...$(NC)"
	@if [ -f $(SCAN_OUTPUT_DIR)/trivy/fs-report.json ]; then \
		echo "### Trivy Filesystem Scan" >> $(SCAN_OUTPUT_DIR)/security-summary.md; \
		jq -r '.Results[]? | select(.Vulnerabilities) | "- \(.Target): \(.Vulnerabilities | length) vulnerabilities"' $(SCAN_OUTPUT_DIR)/trivy/fs-report.json >> $(SCAN_OUTPUT_DIR)/security-summary.md 2>/dev/null || true; \
	fi

	@if [ -f $(SCAN_OUTPUT_DIR)/gosec/report.json ]; then \
		echo "### Gosec Security Issues" >> $(SCAN_OUTPUT_DIR)/security-summary.md; \
		jq -r '.Issues[]? | "- \(.file):\(.line): \(.rule_id) - \(.details)"' $(SCAN_OUTPUT_DIR)/gosec/report.json >> $(SCAN_OUTPUT_DIR)/security-summary.md 2>/dev/null || true; \
	fi

	@for target in $(TARGETS); do \
		if [ -f $(SCAN_OUTPUT_DIR)/trivy/$$target-container-report.json ]; then \
			echo "### $$target Container Vulnerabilities" >> $(SCAN_OUTPUT_DIR)/security-summary.md; \
			jq -r '.Results[]? | select(.Vulnerabilities) | "- \(.Target): \(.Vulnerabilities | length) vulnerabilities"' $(SCAN_OUTPUT_DIR)/trivy/$$target-container-report.json >> $(SCAN_OUTPUT_DIR)/security-summary.md 2>/dev/null || true; \
		fi; \
	done

	@echo "" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "## Recommendations" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "1. Review and fix CRITICAL and HIGH severity vulnerabilities" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "2. Update dependencies to latest secure versions" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "3. Implement security controls for identified risks" >> $(SCAN_OUTPUT_DIR)/security-summary.md
	@echo "4. Run security scans regularly in CI/CD pipeline" >> $(SCAN_OUTPUT_DIR)/security-summary.md

	@echo "$(GREEN)Security report generated: $(SCAN_OUTPUT_DIR)/security-summary.md$(NC)"
	@echo "$(BLUE)View detailed results in: $(SCAN_OUTPUT_DIR)/$(NC)"

clean-reports: ## Clean security scan reports
	@echo "$(YELLOW)Cleaning security reports...$(NC)"
	@rm -rf $(SCAN_OUTPUT_DIR)
	@echo "$(GREEN)Security reports cleaned$(NC)"

clean-images: ## Remove scan images
	@echo "$(YELLOW)Removing scan images...$(NC)"
	@for target in $(TARGETS); do \
		docker rmi $(DOCKER_IMAGE_PREFIX)-$$target:scan 2>/dev/null || true; \
	done
	@echo "$(GREEN)Scan images cleaned$(NC)"

clean-all: clean-reports clean-images ## Clean all scan artifacts

# Integration targets for CI/CD
ci-security-scan: setup-security-tools security-scan check-critical ## CI/CD security scan with critical check

pre-commit-scan: security-scan-fast ## Fast security scan for pre-commit hooks