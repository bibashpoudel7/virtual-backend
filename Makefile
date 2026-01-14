# Makefile for backend development

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: install-air
install-air: ## Install Air for hot-reload
	@echo "Installing Air for hot-reload..."
	@go install github.com/air-verse/air@latest
	@echo "Air installed successfully!"

.PHONY: dev
dev: ## Run backend with hot-reload (requires Air)
	@echo "Starting backend with hot-reload..."
	@if command -v air >/dev/null 2>&1; then air; else ~/go/bin/air; fi

.PHONY: dev-watch
dev-watch: ## Run backend with file watching and auto-restart
	@echo "Starting backend with file watching..."
	@air -c .air.toml

.PHONY: build
build: ## Build the backend binary
	@echo "Building backend..."
	@go build -o bin/server ./cmd/server
	@echo "Build complete! Binary at bin/server"

.PHONY: run
run: build ## Build and run the backend
	@echo "Running backend..."
	@./bin/server

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

.PHONY: test-watch
test-watch: ## Run tests with file watching
	@echo "Running tests with file watching..."
	@watch -n 2 go test -v ./...

.PHONY: migrate
migrate: ## Run database migrations
	@echo "Running migrations..."
	@go run cmd/migrate/main.go

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	@echo "Rolling back last migration..."
	@go run cmd/migrate/main.go -rollback 1

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf tmp/ bin/ *.log
	@echo "Clean complete!"

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated!"

.PHONY: lint
lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete!"

.PHONY: docker-up
docker-up: ## Start PostgreSQL with Docker
	@echo "Starting PostgreSQL..."
	@docker-compose up -d postgres
	@echo "PostgreSQL started!"

.PHONY: docker-down
docker-down: ## Stop PostgreSQL Docker container
	@echo "Stopping PostgreSQL..."
	@docker-compose down
	@echo "PostgreSQL stopped!"

# Combined commands
.PHONY: dev-full
dev-full: docker-up migrate dev ## Start everything for development (DB, migrations, hot-reload)

.PHONY: restart
restart: ## Force restart the development server
	@echo "Restarting server..."
	@pkill -f "air" || true
	@sleep 1
	@make dev