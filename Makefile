.PHONY: help test test-integration test-all lint fmt vet security build clean coverage benchmark docker

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Test targets
test: ## Run unit tests
	go test -v -race -coverprofile=coverage.out ./...

test-integration: ## Run integration tests
	go test -v -tags=integration ./tests/integration/...

test-all: test test-integration ## Run all tests

# Code quality targets
lint: ## Run golangci-lint (full)
	golangci-lint run --timeout=5m

lint-fast: ## Run golangci-lint (fast configuration)
	golangci-lint run --config .golangci-fast.yml --timeout=2m

lint-essential: ## Run golangci-lint (essential linters only)
	golangci-lint run --config .golangci-essential.yml --timeout=1m

fmt: ## Format code
	gofmt -s -w .
	goimports -w .

vet: ## Run go vet
	go vet ./...

security: ## Run security scans
	@echo "Installing security tools..."
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Running gosec security scanner..."
	gosec -fmt sarif -out gosec.sarif ./... || echo "Security issues found - check gosec.sarif"
	@echo "Running govulncheck..."
	govulncheck ./... || echo "Vulnerabilities found - check output above"

# Build targets
build: ## Build the binary
	go build -o cge .

build-all: ## Build for all platforms
	GOOS=linux GOARCH=amd64 go build -o cge-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o cge-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o cge-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o cge-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o cge-windows-amd64.exe .

# Coverage and reporting
coverage: test ## Generate coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

# Docker targets
docker: ## Build Docker image
	docker build -t cge:latest .

docker-test: docker ## Test Docker image
	docker run --rm cge:latest --version

# Cleanup
clean: ## Clean build artifacts
	rm -f cge cge-* coverage.out coverage.html
	go clean -cache
	go clean -testcache

# Development setup
deps: ## Download dependencies
	go mod download
	go mod verify

tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

# CI simulation
ci-test: ## Run tests like CI
	@echo "Running tests with race detection and coverage..."
	go test -v -race -coverprofile=coverage.out ./... | tee test-results.log
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/... | tee integration-test-results.log

ci-lint: ## Run linting like CI (fast)
	@echo "Running golangci-lint (fast)..."
	golangci-lint run --config .golangci-fast.yml --timeout=2m
	@echo "Running go vet..."
	go vet ./...
	@echo "Checking formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not formatted:"; \
		gofmt -s -l .; \
		exit 1; \
	fi

ci-security: ## Run security scans like CI
	@echo "Running gosec..."
	gosec -fmt sarif -out gosec.sarif ./...
	@echo "Running govulncheck..."
	govulncheck ./...

ci-all: ci-test ci-lint ci-security build ## Run all CI checks locally

# Lint fix targets
lint-fix-quick: ## Apply quick and easy lint fixes (formatting, spelling, etc.)
	@echo "🔧 Applying quick lint fixes..."
	@./scripts/quick-lint-fixes.sh

lint-fix-security: ## Apply automated security fixes (file permissions)
	@echo "🔒 Applying security fixes..."
	@./scripts/security-fixes.sh

lint-fix-all: lint-fix-quick lint-fix-security ## Apply all automated lint fixes 