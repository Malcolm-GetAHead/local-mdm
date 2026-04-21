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

docker-up: ## Start Docker containers
	@echo "Starting Docker containers..."
	@docker-compose up -d

docker-down: ## Stop Docker containers
	@echo "Stopping Docker containers..."
	@docker-compose down

docker-logs: ## View Docker logs
	@docker-compose logs -f

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev: docker-up migrate-up run ## Start development environment

.DEFAULT_GOAL := help
