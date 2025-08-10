.PHONY: help build run test clean docs dev

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@egrep '^(.+)\s*:.*##\s*(.+)' $(MAKEFILE_LIST) | column -t -c 2 -s ':#'

build: ## Build the application
	@echo "Building application..."
	@go build -o bin/server cmd/server/main.go

run: build ## Build and run the application
	@echo "Running application..."
	@./bin/server

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf docs/

docs: ## Generate API documentation
	@echo "Generating API documentation..."
	@$(HOME)/go/bin/swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

dev: docs ## Generate docs and run in development mode
	@echo "Starting development server..."
	@go run cmd/server/main.go

install-deps: ## Install development dependencies
	@echo "Installing dependencies..."
	@go mod tidy
	@go install github.com/swaggo/swag/cmd/swag@latest

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t goapitemplate .

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	@docker run -p 8080:8080 goapitemplate

setup-db: ## Set up database tables
	@echo "Setting up database tables..."
	@go run scripts/migrate.go