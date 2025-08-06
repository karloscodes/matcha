# Matcha - Makefile
.PHONY: help dev build run test clean docker-build docker-run release deps fmt lint vet security

# Default target
help: ## Show this help message
	@echo "Matcha - Available commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development
dev: deps build-css ## Start development server with hot reload and CSS building
	@echo "Starting development server with hot reload..."
	@if ! command -v air > /dev/null; then \
		echo "Installing air for hot reload..."; \
		go install github.com/air-verse/air@latest; \
	fi
	@GO_ENV=development air

build-css: ## Build Tailwind CSS
	@echo "Building Tailwind CSS..."
	@if [ ! -d "node_modules" ]; then \
		echo "Installing npm dependencies..."; \
		npm install; \
	fi
	@npm run build-css-prod

watch-css: ## Watch and rebuild CSS on changes
	@echo "Watching CSS files for changes..."
	@npm run build-css

run: ## Run the application
	@echo "Starting Matcha..."
	@GO_ENV=development go run main.go

build: ## Build the application (Linux AMD64)
	@echo "Building Matcha for Linux AMD64..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/matcha main.go

build-all: ## Build for all platforms
	@echo "Building Matcha for all platforms..."
	@mkdir -p dist
	@echo "Building Linux AMD64 (primary)..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/matcha main.go
	@echo "Building Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/matcha-linux-arm64 main.go
	@echo "Building macOS Intel..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/matcha-darwin-amd64 main.go
	@echo "Building macOS Apple Silicon..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/matcha-darwin-arm64 main.go
	@echo "Building Windows..."
	@GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/matcha-windows-amd64.exe main.go
	@echo "Creating checksums..."
	@cd dist && sha256sum * > checksums.txt
	@echo "Build complete! Binaries in dist/ directory"

release: deps build-css-prod build-all ## Build everything for release
	@echo "Release build complete!"

build-css-prod: ## Build production CSS
	@echo "Building production CSS..."
	@npm run build-css-prod

# Dependencies
deps: ## Download and install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@if [ ! -d "node_modules" ]; then \
		echo "Installing npm dependencies..."; \
		npm install; \
	fi

# Database
db-reset: ## Reset the database (removes existing data)
	@echo "Resetting database..."
	@rm -f db/matcha.db db/test_matcha.db

db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	@GO_ENV=development go run main.go &
	@sleep 2
	@pkill -f "go run main.go" || true

# Testing
test: ## Run tests
	@echo "Running tests..."
	@GO_ENV=test go test -v ./...

ci-test: ## Run comprehensive CI tests (includes race detection and coverage)
	@echo "Running comprehensive CI tests..."
	@GO_ENV=test go test -v -race -coverprofile=coverage.out ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@GO_ENV=test go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code Quality
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@if ! command -v golangci-lint > /dev/null; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2; \
	fi
	@golangci-lint run

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

security: ## Run security scan
	@echo "Running security scan..."
	@if ! command -v gosec > /dev/null; then \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi
	@gosec ./...

ci-full: deps build-css-prod fmt vet lint ci-test ## Complete CI pipeline (deps, build, format, vet, lint, test)
	@echo "✅ Full CI pipeline completed successfully!"

ci-security: security ## Run security scan separately
	@echo "✅ Security scan completed successfully!"

ci-quick: deps build-css-prod fmt vet ci-test ## Quick CI pipeline (without lint and security)
	@echo "✅ Quick CI pipeline completed successfully!"

# Docker
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t matcha:latest .

docker-run: docker-build ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8080:8080 -e GO_ENV=production matcha:latest

# Production

# Utilities
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/ tmp/ dist/ coverage.out coverage.html build-errors.log

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/goreleaser/goreleaser@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2

# Production deployment helpers
prod-build: ## Build for production
	@echo "Building for production..."
	@GO_ENV=production go build -ldflags="-s -w -X main.version=$(shell git describe --tags --always)" -o bin/matcha-prod main.go

prod-run: prod-build ## Run production build
	@echo "Starting production server..."
	@GO_ENV=production ./bin/matcha-prod

# Version info
version: ## Show version information
	@echo "Go version: $(shell go version)"
	@echo "Git commit: $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build time: $(shell date)"

# Development workflow
quick-start: deps db-reset dev ## Quick start for new developers
