# Makefile for GitLab Reviewer Roulette Bot
# See `make help` for available targets
#
# Quick Reference:
#   make check-env        - Check prerequisites
#   make setup-complete   - Setup complete local Docker environment (automated)
#   make start            - Start Docker stack
#   make logs             - Follow Docker logs
#   make down             - Stop Docker stack
#   make help             - Show all available commands

.DEFAULT_GOAL := help

# ==================================================================================== #
# VARIABLES
# ==================================================================================== #

# Application
APP_NAME := gitlab-reviewer-roulette
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Directories
BIN_DIR := ./bin
CMD_DIR := ./cmd
INTERNAL_DIR := ./internal
MIGRATIONS_DIR := ./migrations

# Binaries
SERVER_BIN := $(BIN_DIR)/server
MIGRATE_BIN := $(BIN_DIR)/migrate
INIT_BIN := $(BIN_DIR)/init

# Go
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod

# Docker
DOCKER := docker
DOCKER_COMPOSE := docker compose

# Coverage
COVERAGE_FILE := coverage.txt
COVERAGE_HTML := coverage.html

# Colors (optional, can be disabled with NO_COLOR=1)
ifndef NO_COLOR
	CYAN := \033[0;36m
	GREEN := \033[0;32m
	YELLOW := \033[0;33m
	RED := \033[0;31m
	NC := \033[0m
else
	CYAN :=
	GREEN :=
	YELLOW :=
	RED :=
	NC :=
endif

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

.PHONY: help
help: ## Display this help message
	@printf "\n"
	@printf "$(CYAN)GitLab Reviewer Roulette Bot - Makefile$(NC)\n"
	@printf "\n"
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(CYAN)<target>$(NC)\n\nTargets:\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  $(CYAN)%-20s$(NC) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@printf "\n"

##@ Running Application (Docker - Primary Method)

.PHONY: start
start: ## Start Docker stack (postgres, redis, app)
	@printf "$(CYAN)Starting Docker stack...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile local up -d
	@printf "$(GREEN)✓ Docker stack started$(NC)\n"
	@printf "$(CYAN)Services:$(NC)\n"
	@printf "  • PostgreSQL: localhost:5432\n"
	@printf "  • Redis: localhost:6379\n"
	@printf "  • App: http://localhost:8080\n"

.PHONY: down
down: ## Stop Docker stack
	@printf "$(CYAN)Stopping Docker stack...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile local down
	@printf "$(GREEN)✓ Docker stack stopped$(NC)\n"

.PHONY: restart
restart: ## Restart container
	@printf "$(CYAN)Restarting containers...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile local restart
	@printf "$(GREEN)✓ App restarted$(NC)\n"

.PHONY: logs
logs: ## Follow Docker logs (all services)
	@$(DOCKER_COMPOSE) --profile local logs -f

.PHONY: logs-app
logs-app: ## Follow app container logs only
	@$(DOCKER_COMPOSE) logs -f app

##@ Binary Development (Advanced)

.PHONY: run
run: ## Run app binary on host (requires services running)
	@printf "$(YELLOW)⚠ Running binary on host. Services must be available.$(NC)\n"
	@printf "$(CYAN)Starting server...$(NC)\n"
	@$(GO) run $(CMD_DIR)/server/main.go

##@ Development Tools

.PHONY: fmt
fmt: ## Format Go code
	@printf "$(CYAN)Formatting code...$(NC)\n"
	@$(GO) fmt ./...
	@printf "$(GREEN)✓ Code formatted$(NC)\n"

.PHONY: fmt-check
fmt-check: ## Check if code is formatted (non-modifying)
	@printf "$(CYAN)Checking code formatting...$(NC)\n"
	@diff=$$(gofmt -l .); \
	if [ -n "$$diff" ]; then \
		printf "$(RED)✗ Code not formatted. Run 'make fmt'$(NC)\n"; \
		printf "Files needing formatting:\n$$diff\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)✓ Code is properly formatted$(NC)\n"

.PHONY: vet
vet: ## Run go vet
	@printf "$(CYAN)Running go vet...$(NC)\n"
	@$(GO) vet ./...
	@printf "$(GREEN)✓ Vet passed$(NC)\n"

.PHONY: lint
lint: ## Run golangci-lint
	@printf "$(CYAN)Running linters...$(NC)\n"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		printf "$(RED)✗ golangci-lint not installed$(NC)\n"; \
		printf "Install: $(CYAN)make install-tools$(NC)\n"; \
		exit 1; \
	fi
	@golangci-lint run --timeout 5m
	@printf "$(GREEN)✓ Linting passed$(NC)\n"

##@ Building

.PHONY: build
build: build-server build-migrate build-init ## Build all binaries

.PHONY: build-server
build-server: ## Build server binary
	@printf "$(CYAN)Building server...$(NC)\n"
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) $(GOFLAGS) -ldflags="-X main.version=$(VERSION)" -o $(SERVER_BIN) $(CMD_DIR)/server
	@printf "$(GREEN)✓ Server built: $(SERVER_BIN)$(NC)\n"

.PHONY: build-migrate
build-migrate: ## Build migrate binary
	@printf "$(CYAN)Building migrate tool...$(NC)\n"
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) $(GOFLAGS) -o $(MIGRATE_BIN) $(CMD_DIR)/migrate
	@printf "$(GREEN)✓ Migrate built: $(MIGRATE_BIN)$(NC)\n"

.PHONY: build-init
build-init: ## Build init binary
	@printf "$(CYAN)Building init tool...$(NC)\n"
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) $(GOFLAGS) -o $(INIT_BIN) $(CMD_DIR)/init
	@printf "$(GREEN)✓ Init built: $(INIT_BIN)$(NC)\n"

.PHONY: clean
clean: ## Remove build artifacts and test files
	@printf "$(CYAN)Cleaning build artifacts...$(NC)\n"
	@rm -rf $(BIN_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@$(GOCLEAN)
	@printf "$(GREEN)✓ Clean complete$(NC)\n"

##@ Testing

.PHONY: test
test: test-unit ## Run all tests (default: unit tests)

.PHONY: test-unit
test-unit: ## Run unit tests with coverage
	@printf "$(CYAN)Running unit tests...$(NC)\n"
	@$(GOTEST) -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(INTERNAL_DIR)/... $(CMD_DIR)/... 2>&1 | \
		grep -v "no test files" || true
	@printf "$(GREEN)✓ Unit tests passed$(NC)\n"

.PHONY: test-short
test-short: ## Run unit tests without race detector (faster)
	@printf "$(CYAN)Running unit tests (short mode)...$(NC)\n"
	@$(GOTEST) -short -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(INTERNAL_DIR)/... $(CMD_DIR)/... 2>&1 | \
		grep -v "no test files" || true
	@printf "$(GREEN)✓ Unit tests passed$(NC)\n"

.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	@printf "$(CYAN)Running integration tests...$(NC)\n"
	@if ! $(DOCKER) ps >/dev/null 2>&1; then \
		printf "$(RED)✗ Docker not running$(NC)\n"; \
		exit 1; \
	fi
	@$(GOTEST) -v -tags=integration ./test/integration/...
	@printf "$(GREEN)✓ Integration tests passed$(NC)\n"

.PHONY: test-all
test-all: test-unit test-integration ## Run all tests (unit + integration)

.PHONY: test-coverage
test-coverage: test-unit ## Generate HTML coverage report
	@printf "$(CYAN)Generating coverage report...$(NC)\n"
	@$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@printf "$(GREEN)✓ Coverage report: $(COVERAGE_HTML)$(NC)\n"

.PHONY: test-ci
test-ci: ## Run tests in CI mode (no race detector if CGO unavailable)
	@printf "$(CYAN)Running CI tests...$(NC)\n"
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(INTERNAL_DIR)/... $(CMD_DIR)/... || \
		(printf "$(YELLOW)⚠ Tests failed$(NC)\n" && exit 1)
	@printf "$(GREEN)✓ CI tests passed$(NC)\n"

##@ Quality

.PHONY: check
check: fmt vet lint lint-markdown test security vuln-check ## Run all quality checks including security

.PHONY: verify
verify: ## Verify dependencies and code
	@printf "$(CYAN)Verifying dependencies...$(NC)\n"
	@$(GOMOD) verify
	@printf "$(GREEN)✓ Dependencies verified$(NC)\n"

.PHONY: deps
deps: ## Download and tidy dependencies
	@printf "$(CYAN)Downloading dependencies...$(NC)\n"
	@$(GOMOD) download
	@$(GOMOD) tidy
	@printf "$(GREEN)✓ Dependencies updated$(NC)\n"

.PHONY: install-tools
install-tools: ## Install development tools
	@printf "$(CYAN)Installing development tools...$(NC)\n"
	@printf "  • Installing golangci-lint v2.6.1...\n"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.6.1
	@printf "  • Installing govulncheck (vulnerability checker)...\n"
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@printf "$(GREEN)✓ All development tools installed$(NC)\n"

##@ Markdown Quality

.PHONY: lint-markdown
lint-markdown: ## Lint markdown files
	@printf "$(CYAN)Linting markdown files...$(NC)\n"
	@if command -v npx >/dev/null 2>&1; then \
		npx --yes markdownlint-cli2 "**/*.md" || \
		(printf "$(YELLOW)⚠ Markdown linting found issues$(NC)\n" && exit 1); \
	elif command -v markdownlint-cli2 >/dev/null 2>&1; then \
		markdownlint-cli2 "**/*.md" || \
		(printf "$(YELLOW)⚠ Markdown linting found issues$(NC)\n" && exit 1); \
	else \
		printf "$(YELLOW)⚠ markdownlint-cli2 not found$(NC)\n"; \
		printf "Install: $(CYAN)npm install -g markdownlint-cli2$(NC)\n"; \
		printf "Or use npx (requires Node.js): $(CYAN)npx will auto-install on first run$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)✓ Markdown linting passed$(NC)\n"

.PHONY: fix-markdown
fix-markdown: ## Auto-fix markdown issues (where possible)
	@printf "$(CYAN)Fixing markdown files...$(NC)\n"
	@if command -v npx >/dev/null 2>&1; then \
		npx --yes markdownlint-cli2 "**/*.md" --fix || \
		printf "$(YELLOW)⚠ Some issues could not be auto-fixed$(NC)\n"; \
	elif command -v markdownlint-cli2 >/dev/null 2>&1; then \
		markdownlint-cli2 "**/*.md" --fix || \
		printf "$(YELLOW)⚠ Some issues could not be auto-fixed$(NC)\n"; \
	else \
		printf "$(RED)✗ markdownlint-cli2 not found$(NC)\n"; \
		printf "Install: $(CYAN)npm install -g markdownlint-cli2$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)✓ Markdown auto-fix complete$(NC)\n"

##@ Security

.PHONY: security
security: ## Run security checks with golangci-lint (respects .golangci.yml exclusions)
	@printf "$(CYAN)Running security scan...$(NC)\n"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		printf "$(RED)✗ golangci-lint not installed$(NC)\n"; \
		printf "Install: $(CYAN)make install-tools$(NC)\n"; \
		exit 1; \
	fi
	@golangci-lint run --timeout 5m --enable-only gosec
	@printf "$(GREEN)✓ Security scan complete$(NC)\n"

.PHONY: vuln-check
vuln-check: ## Check for dependency vulnerabilities
	@printf "$(CYAN)Checking for vulnerabilities...$(NC)\n"
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		printf "$(RED)✗ govulncheck not installed$(NC)\n"; \
		printf "Install: $(CYAN)make install-tools$(NC)\n"; \
		exit 1; \
	fi
	@govulncheck ./...
	@printf "$(GREEN)✓ No vulnerabilities found$(NC)\n"

##@ Pre-commit Hooks

.PHONY: pre-commit-install
pre-commit-install: ## Install pre-commit and commit-msg hooks
	@printf "$(CYAN)Installing Git hooks...$(NC)\n"
	@if [ ! -f scripts/pre-commit.sh ]; then \
		printf "$(RED)✗ scripts/pre-commit.sh not found$(NC)\n"; \
		exit 1; \
	fi
	@if [ ! -f scripts/commit-msg.sh ]; then \
		printf "$(RED)✗ scripts/commit-msg.sh not found$(NC)\n"; \
		exit 1; \
	fi
	@cp scripts/pre-commit.sh .git/hooks/pre-commit
	@cp scripts/commit-msg.sh .git/hooks/commit-msg
	@chmod +x .git/hooks/pre-commit
	@chmod +x .git/hooks/commit-msg
	@printf "$(GREEN)✓ Git hooks installed$(NC)\n"
	@printf "$(YELLOW)Note: Hooks run automatically on commit$(NC)\n"
	@printf "$(YELLOW)To skip: git commit --no-verify$(NC)\n"

.PHONY: pre-commit-uninstall
pre-commit-uninstall: ## Uninstall pre-commit hooks
	@printf "$(CYAN)Uninstalling Git hooks...$(NC)\n"
	@rm -f .git/hooks/pre-commit
	@rm -f .git/hooks/commit-msg
	@printf "$(GREEN)✓ Git hooks uninstalled$(NC)\n"

##@ Database (Smart Auto-Detection)

.PHONY: migrate
migrate: ## Run migrations (auto-detects Docker vs host)
	@if docker compose ps app 2>/dev/null | grep -q "Up"; then \
		printf "$(CYAN)⚙ Detected app container running, using Docker...$(NC)\n"; \
		$(MAKE) docker-migrate; \
	else \
		printf "$(CYAN)⚙ App container not running, using host binary...$(NC)\n"; \
		$(MAKE) migrate-up; \
	fi

.PHONY: seed
seed: ## Seed database (auto-detects Docker vs host)
	@if docker compose ps app 2>/dev/null | grep -q "Up"; then \
		printf "$(CYAN)⚙ Detected app container running, using Docker...$(NC)\n"; \
		$(MAKE) docker-seed; \
	else \
		printf "$(CYAN)⚙ App container not running, using host psql...$(NC)\n"; \
		$(MAKE) seed-host; \
	fi

##@ Database (Advanced - Manual Control)

.PHONY: db-setup
db-setup: migrate seed ## Setup database (migrate + seed)

.PHONY: migrate-up
migrate-up: ## Run migrations via host binary
	@printf "$(YELLOW)⚠ Running on HOST. Connects to: $${POSTGRES_HOST:-localhost}:5432$(NC)\n"
	@$(GO) run $(CMD_DIR)/migrate/main.go up
	@printf "$(GREEN)✓ Migrations applied$(NC)\n"

.PHONY: migrate-down
migrate-down: ## Rollback migration via host binary
	@printf "$(YELLOW)⚠ Running on HOST$(NC)\n"
	@$(GO) run $(CMD_DIR)/migrate/main.go down
	@printf "$(GREEN)✓ Migration rolled back$(NC)\n"

.PHONY: migrate-create
migrate-create: ## Create new migration (usage: make migrate-create name=add_users)
	@if [ -z "$(name)" ]; then \
		printf "$(RED)✗ Error: name parameter required$(NC)\n"; \
		printf "Usage: $(CYAN)make migrate-create name=add_users$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(CYAN)Creating migration: $(name)$(NC)\n"
	@timestamp=$$(date +%Y%m%d%H%M%S); \
	touch $(MIGRATIONS_DIR)/$${timestamp}_$(name).up.sql; \
	touch $(MIGRATIONS_DIR)/$${timestamp}_$(name).down.sql; \
	printf "$(GREEN)✓ Created:$(NC)\n"; \
	printf "  - $(MIGRATIONS_DIR)/$${timestamp}_$(name).up.sql\n"; \
	printf "  - $(MIGRATIONS_DIR)/$${timestamp}_$(name).down.sql\n"

.PHONY: seed-host
seed-host: ## Seed database via host psql
	@printf "$(YELLOW)⚠ Running on HOST via psql$(NC)\n"
	@./scripts/seed-database.sh
	@printf "$(GREEN)✓ Database seeded$(NC)\n"

.PHONY: docker-seed
docker-seed: ## Seed database via Docker
	@printf "$(YELLOW)⚠ Running in DOCKER container$(NC)\n"
	@$(DOCKER_COMPOSE) exec postgres psql -U postgres -d reviewer_roulette -c "\copy users(gitlab_id,username,email,role,team,created_at,updated_at) FROM '/tmp/seed.sql' WITH DELIMITER ',' CSV HEADER" || \
		(printf "$(YELLOW)Note: Docker seeding requires seed data in container. Using exec instead.$(NC)\n" && \
		 $(DOCKER_COMPOSE) exec -T postgres psql -U postgres -d reviewer_roulette < scripts/seed-database.sh)
	@printf "$(GREEN)✓ Database seeded$(NC)\n"

##@ Docker Services (Advanced - Granular Control)

.PHONY: docker-up-services
docker-up-services: ## Start only PostgreSQL and Redis (minimal)
	@printf "$(CYAN)Starting PostgreSQL and Redis...$(NC)\n"
	@$(DOCKER_COMPOSE) up -d postgres redis
	@printf "$(GREEN)✓ Services started$(NC)\n"
	@printf "$(CYAN)Services:$(NC)\n"
	@printf "  • PostgreSQL: localhost:5432 (postgres/postgres)\n"
	@printf "  • Redis: localhost:6379\n"

.PHONY: docker-ps
docker-ps: ## Show running Docker containers
	@$(DOCKER_COMPOSE) ps

.PHONY: docker-build
docker-build: ## Build Docker image
	@printf "$(CYAN)Building Docker image...$(NC)\n"
	@$(DOCKER) build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@printf "$(GREEN)✓ Docker image built: $(APP_NAME):$(VERSION)$(NC)\n"

.PHONY: docker-clean
docker-clean: ## Remove all containers, volumes, and images
	@printf "$(RED)⚠️  This will remove all containers, volumes, and network!$(NC)\n"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		printf "$(CYAN)Cleaning Docker environment...$(NC)\n"; \
		$(DOCKER_COMPOSE) --profile local --profile gitlab --profile observability down -v; \
		printf "$(GREEN)✓ Docker environment cleaned$(NC)\n"; \
	else \
		printf "$(YELLOW)Cancelled$(NC)\n"; \
	fi

.PHONY: reset
reset: docker-clean ## Complete reset (containers, volumes, binaries)
	@printf "$(CYAN)Removing build artifacts and .env...$(NC)\n"
	@rm -rf $(BIN_DIR)
	@rm -f .env
	@printf "$(GREEN)✓ Reset complete$(NC)\n"
	@printf "\n$(YELLOW)To start fresh:$(NC)\n"
	@printf "  • Run: $(CYAN)make setup-complete$(NC)\n\n"

##@ CI/CD

.PHONY: ci
ci: deps verify check build ## Run full CI pipeline locally

.PHONY: pre-commit
pre-commit: fmt vet test-short ## Quick checks before commit

##@ Initialization

.PHONY: init
init: build-init ## Initialize from GitLab (sync users from test-org group, on host)
	@printf "$(CYAN)Running initialization...$(NC)\n"
	@$(INIT_BIN) --config config.yaml --group-path=test-org --mrs=false
	@printf "$(GREEN)✓ Initialization complete$(NC)\n"

.PHONY: init-dry-run
init-dry-run: build-init ## Dry run initialization (no database writes)
	@printf "$(CYAN)Running initialization (dry run)...$(NC)\n"
	@$(INIT_BIN) --config config.yaml --dry-run
	@printf "$(GREEN)✓ Dry run complete$(NC)\n"

.PHONY: sync-users
sync-users: build-init ## Sync users from GitLab group (usage: make sync-users GROUP=123)
	@if [ -z "$(GROUP)" ]; then \
		printf "$(RED)✗ Error: GROUP parameter required$(NC)\n"; \
		printf "Usage: $(CYAN)make sync-users GROUP=123$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(CYAN)Syncing users from group $(GROUP)...$(NC)\n"
	@$(INIT_BIN) --config config.yaml --group $(GROUP) --mrs=false
	@printf "$(GREEN)✓ User sync complete$(NC)\n"

.PHONY: sync-mrs
sync-mrs: build-init ## Sync open MRs from project (usage: make sync-mrs PROJECT=456)
	@if [ -z "$(PROJECT)" ]; then \
		printf "$(RED)✗ Error: PROJECT parameter required$(NC)\n"; \
		printf "Usage: $(CYAN)make sync-mrs PROJECT=456$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(CYAN)Syncing MRs from project $(PROJECT)...$(NC)\n"
	@$(INIT_BIN) --config config.yaml --project $(PROJECT) --users=false
	@printf "$(GREEN)✓ MR sync complete$(NC)\n"

##@ Docker Operations (Advanced - Container Commands)

.PHONY: docker-migrate
docker-migrate: ## Run migrations in Docker container
	@printf "$(YELLOW)⚠ Running in DOCKER container$(NC)\n"
	@$(DOCKER_COMPOSE) exec app /app/migrate up
	@printf "$(GREEN)✓ Migrations applied$(NC)\n"

.PHONY: docker-migrate-down
docker-migrate-down: ## Rollback migrations in Docker container
	@printf "$(YELLOW)⚠ Running in DOCKER container$(NC)\n"
	@$(DOCKER_COMPOSE) exec app /app/migrate down
	@printf "$(GREEN)✓ Migration rolled back$(NC)\n"

.PHONY: docker-init
docker-init: ## Initialize from GitLab in Docker container
	@printf "$(YELLOW)⚠ Running in DOCKER container$(NC)\n"
	@$(DOCKER_COMPOSE) exec app /app/init --config /app/config.yaml --group-path=test-org --mrs=false
	@printf "$(GREEN)✓ Initialization complete$(NC)\n"

.PHONY: docker-shell
docker-shell: ## Open shell inside app container
	@$(DOCKER_COMPOSE) exec app /bin/sh

##@ GitLab Local Setup

.PHONY: setup-gitlab
setup-gitlab: ## Run automated GitLab configuration script
	@printf "$(CYAN)Running GitLab setup script...$(NC)\n"
	@if [ ! -f ./scripts/setup-gitlab.sh ]; then \
		printf "$(RED)✗ Setup script not found$(NC)\n"; \
		exit 1; \
	fi
	@if ! $(DOCKER) ps | grep -q gitlab; then \
		printf "$(RED)✗ GitLab container not running$(NC)\n"; \
		printf "Start GitLab first: $(CYAN)make gitlab-up$(NC)\n"; \
		exit 1; \
	fi
	@./scripts/setup-gitlab.sh --minimal
	@printf "$(GREEN)✓ GitLab configured$(NC)\n"

.PHONY: setup-complete
setup-complete: docker-build ## Setup complete local Docker environment (automated all-in-one)
	@printf "$(CYAN)╔════════════════════════════════════════════╗$(NC)\n"
	@printf "$(CYAN)║  Local Docker Environment Setup            ║$(NC)\n"
	@printf "$(CYAN)╚════════════════════════════════════════════╝$(NC)\n\n"
	@printf "$(YELLOW)Step 1/6: Starting database services (postgres, redis)...$(NC)\n"
	@$(DOCKER_COMPOSE) up -d postgres redis
	@printf "$(GREEN)✓ Database services started$(NC)\n\n"
	@printf "$(YELLOW)Step 2/6: Starting GitLab infrastructure...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile gitlab up -d
	@printf "$(GREEN)✓ GitLab starting$(NC)\n\n"
	@printf "$(YELLOW)Step 3/6: Waiting for GitLab to be ready (5-10 min)...$(NC)\n"
	@printf "Monitor: $(CYAN)docker logs -f gitlab$(NC)\n\n"
	@timeout=600; \
	elapsed=0; \
	while [ $$elapsed -lt $$timeout ]; do \
		if curl -sf http://localhost:8000/users/sign_in >/dev/null 2>&1; then \
			printf "$(GREEN) ✓ GitLab is ready!$(NC)\n\n"; \
			break; \
		fi; \
		printf "."; \
		sleep 10; \
		elapsed=$$((elapsed + 10)); \
	done; \
	if [ $$elapsed -ge $$timeout ]; then \
		printf "\n$(RED) ✗ GitLab failed to start within 10 minutes$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(YELLOW)Step 4/6: Running GitLab configuration (creates .env)...$(NC)\n"
	@./scripts/setup-gitlab.sh --minimal --skip-wait --automated
	@if [ ! -f .env ]; then \
		printf "$(RED)✗ .env file not created$(NC)\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)✓ GitLab configured, .env created$(NC)\n\n"
	@printf "$(YELLOW)Step 5/6: Running database migrations...$(NC)\n"
	@$(DOCKER_COMPOSE) run --rm app /app/migrate up
	@printf "$(GREEN)✓ Migrations applied$(NC)\n\n"
	@printf "$(YELLOW)Step 6/6: Starting app container (with .env credentials)...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile local up -d app
	@printf "$(GREEN)✓ App container started$(NC)\n\n"
	@printf "$(YELLOW)Populating GitLab with test data...$(NC)\n"
	@$(MAKE) gitlab-seed
	@printf "\n$(YELLOW)Syncing users from GitLab to database...$(NC)\n"
	@$(MAKE) docker-init
	@printf "\n$(GREEN)✓ Local Docker environment ready!$(NC)\n\n"
	@printf "$(CYAN)Services running:$(NC)\n"
	@printf "  • GitLab:   http://localhost:8000 (root/admin123)\n"
	@printf "  • App:      http://localhost:8080\n"
	@printf "  • Metrics:  http://localhost:9090/metrics\n\n"
	@printf "$(CYAN)Test data created:$(NC)\n"
	@printf "  • 12 users across 5 teams\n"
	@printf "  • 5 projects with CODEOWNERS files\n"
	@printf "  • 9 merge requests ready for testing\n"
	@printf "  • Teams: backend, frontend, platform, mobile, data\n\n"
	@printf "$(CYAN)Try it out:$(NC)\n"
	@printf "  1. Open an MR: http://localhost:8000/dashboard/merge_requests\n"
	@printf "  2. Post a comment: $(YELLOW)/roulette$(NC)\n"
	@printf "  3. Watch reviewers get assigned!\n\n"
	@printf "$(CYAN)Useful commands:$(NC)\n"
	@printf "  • $(CYAN)make logs$(NC)       	- Follow all logs\n"
	@printf "  • $(CYAN)make logs-app$(NC)   	- Follow app logs only\n"
	@printf "  • $(CYAN)make restart$(NC)    	- Restart app container\n"
	@printf "  • $(CYAN)make logs-gitlab$(NC) 	- View GitLab logs\n"
	@printf "  • $(CYAN)make down$(NC)       	- Stop everything\n\n"

# ============================================================================
# GitLab Infrastructure Management
# ============================================================================

.PHONY: gitlab-up
gitlab-up: ## Start GitLab infrastructure
	@printf "$(YELLOW)Starting GitLab infrastructure...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile gitlab up -d gitlab
	@printf "$(GREEN)✓ GitLab started$(NC)\n"
	@printf "$(YELLOW)⏳ GitLab is starting (may take 5-10 minutes)...$(NC)\n"
	@printf "Monitor: $(CYAN)make logs-gitlab$(NC)\n"
	@printf "Check: $(CYAN)http://localhost:8000$(NC)\n"

.PHONY: gitlab-down
gitlab-down: ## Stop GitLab infrastructure
	@printf "$(YELLOW)Stopping GitLab infrastructure...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile gitlab down
	@printf "$(GREEN)✓ GitLab stopped$(NC)\n"

.PHONY: gitlab-restart
gitlab-restart: ## Restart GitLab infrastructure
	@printf "$(YELLOW)Restarting GitLab infrastructure...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile gitlab restart
	@printf "$(GREEN)✓ GitLab restarted$(NC)\n"

.PHONY: logs-gitlab
logs-gitlab: ## View GitLab logs
	@$(DOCKER_COMPOSE) --profile gitlab logs -f

# ============================================================================
# GitLab Test Data Management
# ============================================================================

.PHONY: gitlab-seed
gitlab-seed: ## Populate GitLab with realistic test data
	@printf "$(CYAN)Setting up GitLab test data...$(NC)\n"
	@if [ ! -f .env ]; then \
		printf "$(RED)✗ .env file not found. Run 'make setup-complete' first.$(NC)\n"; \
		exit 1; \
	fi
	@if [ -f ./scripts/setup-gitlab.sh ]; then \
		set -a; . ./.env; set +a; \
		./scripts/setup-gitlab.sh --realistic; \
		printf "$(GREEN)✓ GitLab test data created$(NC)\n"; \
	else \
		printf "$(YELLOW)⚠ Test data script not found (optional)$(NC)\n"; \
	fi

.PHONY: gitlab-clean
gitlab-clean: ## Remove all GitLab test data
	@printf "$(YELLOW)⚠️  Automated cleanup not yet implemented in unified script$(NC)\n"
	@printf "\nTo clean GitLab test data manually:\n"
	@printf "  1. Open GitLab: $(CYAN)http://localhost:8000$(NC)\n"
	@printf "  2. Delete test-org group (Settings > General > Advanced)\n"
	@printf "  3. Delete test users (Admin Area > Users)\n"
	@printf "\nOr completely reset GitLab:\n"
	@printf "  $(CYAN)make docker-clean$(NC) (removes all containers and volumes)\n"

##@ Environment Status

.PHONY: status
status: ## Show status of all services
	@printf "\n$(CYAN)╔════════════════════════════════════════════╗$(NC)\n"
	@printf "$(CYAN)║  Service Status                            ║$(NC)\n"
	@printf "$(CYAN)╚════════════════════════════════════════════╝$(NC)\n\n"
	@printf "$(YELLOW)Docker Services:$(NC)\n"
	@if $(DOCKER) ps 2>/dev/null | grep -q reviewer-roulette; then \
		$(DOCKER_COMPOSE) ps; \
	else \
		printf "  $(YELLOW)No services running$(NC)\n"; \
	fi
	@printf "\n$(YELLOW)Application Health:$(NC)\n"
	@if curl -sf http://localhost:8080/health 2>/dev/null | grep -q "ok"; then \
		printf "  $(GREEN)✓ Server running$(NC) (http://localhost:8080)\n"; \
	else \
		printf "  $(YELLOW)✗ Server not running$(NC)\n"; \
	fi
	@printf "\n$(YELLOW)GitLab Status:$(NC)\n"
	@if curl -sf http://localhost:8000/-/readiness 2>/dev/null >/dev/null; then \
		printf "  $(GREEN)✓ GitLab ready$(NC) (http://localhost:8000)\n"; \
	elif $(DOCKER) ps 2>/dev/null | grep -q gitlab; then \
		printf "  $(YELLOW)⏳ GitLab initializing...$(NC)\n"; \
		printf "  Monitor: $(CYAN)docker logs -f gitlab$(NC)\n"; \
	else \
		printf "  $(YELLOW)✗ GitLab not running$(NC)\n"; \
	fi
	@printf "\n"

.PHONY: check-env
check-env: ## Check environment prerequisites
	@./scripts/check-prerequisites.sh

##@ Observability Stack

.PHONY: observability-up
observability-up: ## Start Prometheus and Grafana
	@printf "$(CYAN)Starting observability stack...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile observability up -d
	@printf "$(GREEN)✓ Observability stack started$(NC)\n"
	@printf "$(CYAN)Services:$(NC)\n"
	@printf "  • Prometheus: http://localhost:9091\n"
	@printf "  • Grafana: http://localhost:3000 (admin/admin)\n"

.PHONY: observability-down
observability-down: ## Stop Prometheus and Grafana
	@printf "$(CYAN)Stopping observability stack...$(NC)\n"
	@$(DOCKER_COMPOSE) --profile observability down
	@printf "$(GREEN)✓ Observability stack stopped$(NC)\n"

.PHONY: observability-logs
observability-logs: ## Follow observability stack logs
	@$(DOCKER_COMPOSE) --profile observability logs -f

.PHONY: observability-clean
observability-clean: ## Remove observability stack volumes
	@printf "$(RED)⚠️  This will remove Prometheus and Grafana data!$(NC)\n"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		printf "$(CYAN)Cleaning observability stack...$(NC)\n"; \
		$(DOCKER_COMPOSE) --profile observability down -v; \
		printf "$(GREEN)✓ Observability stack cleaned$(NC)\n"; \
	else \
		printf "$(YELLOW)Cancelled$(NC)\n"; \
	fi

.PHONY: observability-restart
observability-restart: observability-down observability-up ## Restart observability stack

##@ Utilities

.PHONY: version
version: ## Show version information
	@printf "App: $(CYAN)$(APP_NAME)$(NC)\n"
	@printf "Version: $(CYAN)$(VERSION)$(NC)\n"
	@printf "Go: $(CYAN)$(shell $(GO) version)$(NC)\n"

.PHONY: info
info: ## Show project information
	@printf "\n$(CYAN)Project Information$(NC)\n"
	@printf "Name:     $(APP_NAME)\n"
	@printf "Version:  $(VERSION)\n"
	@printf "Go:       $(shell $(GO) version | cut -d' ' -f3)\n"
	@printf "Platform: $(shell uname -s)/$(shell uname -m)\n"
	@printf "\n$(CYAN)Directories$(NC)\n"
	@printf "Binaries: $(BIN_DIR)\n"
	@printf "Source:   $(CMD_DIR), $(INTERNAL_DIR)\n"
	@printf "Tests:    ./test\n"
	@printf ""

.PHONY: all
all: clean deps build check ## Build and test everything
	@printf "$(GREEN)✓ All tasks complete$(NC)\n"
