.PHONY: help build run test clean migrate-up migrate-down migrate-create docker-up docker-down

# Variables
BINARY_NAME=local-mdm
MAIN_PATH=./cmd/server
MIGRATION_DIR=./migrations
DB_URL?=postgres://postgres:postgres@localhost:5432/localmdm?sslmode=disable

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

run: ## Run the application
	@echo "Running $(BINARY_NAME)..."
	@go run $(MAIN_PATH)

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -p 4 -coverprofile=coverage.out ./...

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -v -race -short ./...

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	@go test -v -race -run Integration ./...

test-coverage: test ## Run tests with coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out
	@echo "Coverage report generated: coverage.html"

test-coverage-summary: test ## Show coverage summary
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'

test-coverage: test ## Run tests with coverage report
	@go tool cover -html=coverage.out

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run

migrate-up: ## Run database migrations up
	@echo "Running migrations up..."
	@migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" up

migrate-down: ## Run database migrations down
	@echo "Running migrations down..."
	@migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=migration_name)
	@echo "Creating migration: $(NAME)"
	@migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)

migrate-force: ## Force migration version (usage: make migrate-force VERSION=1)
	@echo "Forcing migration version to $(VERSION)"
	@migrate -path $(MIGRATION_DIR) -database "$(DB_URL)" force $(VERSION)

docker-up: ## Start infrastructure containers (postgres, keycloak, nanomdm, adminer)
	@echo "Starting infrastructure..."
	@docker compose up -d postgres keycloak nanomdm adminer

docker-down: ## Stop all Docker containers
	@echo "Stopping Docker containers..."
	@docker compose down

docker-logs: ## View Docker logs
	@docker compose logs -f

# === Docker-based development ===

dev: ## Start full dev stack (hot reload)
	@echo "Starting dev stack with hot reload..."
	@docker compose up -d postgres keycloak nanomdm adminer
	@docker compose --profile dev up -d localmdm-dev
	@echo ""
	@echo "Dev stack running:"
	@echo "  Local MDM:  http://localhost:8080 (hot reload)"
	@echo "  NanoMDM:    http://localhost:9000"
	@echo "  Keycloak:   http://localhost:8180"
	@echo "  Adminer:    http://localhost:8081"
	@echo "  Metrics:    http://localhost:9090"

dev-test: ## Run tests in dev container (fast, uses cached modules)
	@echo "Running tests in Docker..."
	@docker compose --profile test run --rm test-runner

dev-shell: ## Open a shell in the dev container
	@docker compose --profile dev exec localmdm-dev sh

# === Production-like verification ===

prod-build: ## Build production container
	@echo "Building production container..."
	@docker compose build localmdm

prod-up: ## Start full production-like stack
	@echo "Starting production-like stack..."
	@docker compose up -d postgres keycloak nanomdm localmdm adminer
	@echo ""
	@echo "Production stack running:"
	@echo "  Local MDM:  http://localhost:8080"
	@echo "  NanoMDM:    http://localhost:9000"
	@echo "  Keycloak:   http://localhost:8180"

prod-test: prod-build ## Build prod container + run full E2E tests
	@echo "Running full test suite against production build..."
	@docker compose up -d postgres keycloak nanomdm
	@sleep 5
	@docker compose --profile test run --rm test-runner
	@echo "✓ All tests passed against production build"

prod-down: ## Stop production stack
	@docker compose down

load-test: ## Run k6 load tests against local stack
	@echo "Running k6 load tests..."
	@./tests/load/run_and_record.sh tests/load/steady_state.js
	@./tests/load/run_and_record.sh tests/load/admin_dashboard.js
	@./tests/load/run_and_record.sh tests/load/enrollment_burst.js
	@echo "Results appended to tests/load/results_history.csv"

browser-test: ## Run Playwright browser tests against local stack
	@cd tests/browser && npm install --silent 2>/dev/null && node run-playbook.js

seed: ## Seed database with development data
	@echo "Seeding database..."
	@PGPASSWORD=postgres psql -h localhost -U postgres -d localmdm -f migrations/seed_data.sql
	@echo "✓ Seed data loaded"

css: ## Compile Tailwind CSS (requires ./tailwindcss binary)
	@./tailwindcss --input web/static/css/input.css --output web/static/css/output.css --minify
	@echo "✓ CSS compiled"

css-watch: ## Watch and recompile Tailwind CSS on changes
	@./tailwindcss --input web/static/css/input.css --output web/static/css/output.css --watch

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.DEFAULT_GOAL := help
