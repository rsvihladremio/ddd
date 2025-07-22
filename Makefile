# Copyright 2025 Ryan SVIHLA Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# DDD Testing Makefile

.PHONY: test test-unit test-integration test-all test-coverage clean build help security lint fmt

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Build the application
build: ## Build the DDD application
	@echo "Building DDD application..."
	go build -o bin/ddd ./cmd/ddd

# Clean build artifacts and test data
clean: ## Clean build artifacts and test data
	@echo "Cleaning up..."
	rm -rf bin/
	rm -rf test-data/
	rm -rf coverage/
	rm -f coverage.out

# Run all tests
test-all: test-unit test-integration ## Run all tests (unit and integration)

# Run unit tests (fast tests that don't require external dependencies)
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	go test -v -race -short ./internal/config ./internal/detector

# Run integration tests (tests that use real databases, files, etc.)
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -race ./internal/database ./internal/reporters ./internal/workers ./internal/handlers ./internal/testutil



# Run all tests with coverage
test-coverage: ## Run all tests with coverage reporting
	@echo "Running tests with coverage..."
	mkdir -p coverage
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage/coverage.html
	go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated at coverage/coverage.html"

# Run tests in watch mode (requires entr)
test-watch: ## Run tests in watch mode (requires 'entr' to be installed)
	@echo "Running tests in watch mode..."
	@echo "Watching for changes in Go files..."
	find . -name "*.go" | entr -c make test-unit

# Run specific test package
test-package: ## Run tests for a specific package (usage: make test-package PKG=./internal/database)
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-package PKG=./internal/database"; \
		exit 1; \
	fi
	@echo "Running tests for package: $(PKG)"
	go test -v -race $(PKG)

# Run tests with verbose output and race detection
test-verbose: ## Run all tests with verbose output
	@echo "Running all tests with verbose output..."
	go test -v -race ./...

# Run benchmarks
test-bench: ## Run benchmark tests
	@echo "Running benchmark tests..."
	go test -bench=. -benchmem ./...

# Lint the code
lint: ## Run linting tools
	@echo "Running linting..."
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not properly formatted:"; \
		gofmt -l .; \
		echo "Run 'make fmt' to fix formatting issues."; \
		exit 1; \
	fi
	@echo "Code formatting is correct ✓"
	@echo "Running go vet..."
	go vet ./...

# Check copyright headers
check-headers: ## Check that all Go files have the required copyright header
	@echo "Checking copyright headers..."
	@missing_headers=0; \
	for file in $$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*"); do \
		if ! head -n 1 "$$file" | grep -q "Copyright 2025 Ryan SVIHLA Corporation"; then \
			echo "Missing copyright header in: $$file"; \
			missing_headers=$$((missing_headers + 1)); \
		fi; \
	done; \
	if [ $$missing_headers -gt 0 ]; then \
		echo "Found $$missing_headers files missing copyright headers"; \
		exit 1; \
	else \
		echo "All Go files have copyright headers ✓"; \
	fi

# Add copyright headers to all Go files
add-headers: ## Add copyright headers to all Go files that are missing them
	@echo "Adding copyright headers to Go files..."
	@header='//\tCopyright 2025 Ryan SVIHLA Corporation\n//\n// Licensed under the Apache License, Version 2.0 (the "License");\n// you may not use this file except in compliance with the License.\n// You may obtain a copy of the License at\n//\n//\thttp://www.apache.org/licenses/LICENSE-2.0\n//\n// Unless required by applicable law or agreed to in writing, software\n// distributed under the License is distributed on an "AS IS" BASIS,\n// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n// See the License for the specific language governing permissions and\n// limitations under the License.\n'; \
	for file in $$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*"); do \
		if ! head -n 1 "$$file" | grep -q "Copyright 2025 Ryan SVIHLA Corporation"; then \
			echo "Adding header to: $$file"; \
			temp_file=$$(mktemp); \
			printf "$$header\n" > "$$temp_file"; \
			cat "$$file" >> "$$temp_file"; \
			mv "$$temp_file" "$$file"; \
		fi; \
	done; \
	echo "Copyright headers added ✓"

# Format the code
fmt: ## Format Go code
	@echo "Formatting code..."
	gofmt -w .

# Run security checks
security: ## Run security checks
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@v2.22.5"; \
	fi

# Install test dependencies
install-test-deps: ## Install testing dependencies
	@echo "Installing testing dependencies..."
	go install github.com/securego/gosec/v2/cmd/gosec@v2.22.5

# Install test dependencies for CI
install-test-deps-ci: ## Install testing dependencies for CI environment
	@echo "Installing testing dependencies for CI..."
	go install github.com/securego/gosec/v2/cmd/gosec@v2.22.5

# Setup test environment
setup-test-env: install-test-deps ## Setup complete test environment
	@echo "Setting up test environment..."
	mkdir -p test-data/uploads
	mkdir -p coverage

# Setup test environment for CI
setup-test-env-ci: install-test-deps-ci ## Setup test environment for CI
	@echo "Setting up test environment for CI..."
	mkdir -p test-data/uploads
	mkdir -p coverage

# Run tests in CI environment
test-ci: ## Run tests suitable for CI environment
	@echo "Running tests in CI mode..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./internal/...

# Generate test report
test-report: test-coverage ## Generate comprehensive test report
	@echo "Generating test report..."
	@echo "=== Test Coverage Summary ===" > test-report.txt
	go tool cover -func=coverage.out >> test-report.txt
	@echo "" >> test-report.txt
	@echo "=== Test Results ===" >> test-report.txt
	go test -v ./... >> test-report.txt 2>&1 || true
	@echo "Test report generated: test-report.txt"

# Quick test (unit tests only)
test: test-unit ## Run quick tests (alias for test-unit)

# Development workflow
dev-test: fmt lint test-unit ## Run development workflow (format, lint, test)
	@echo "Development tests completed successfully!"

# Pre-commit checks
pre-commit: fmt lint test-integration ## Run pre-commit checks
	@echo "Pre-commit checks completed successfully!"

# Full validation (everything)
validate: clean fmt lint test-all test-coverage ## Run full validation suite
	@echo "Full validation completed successfully!"

# Docker-based testing (if you want to add Docker support later)
test-docker: ## Run tests in Docker container
	@echo "Docker testing not implemented yet"
	@echo "This would run tests in a clean Docker environment"

# Performance testing
test-perf: ## Run performance tests
	@echo "Running performance tests..."
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./...
	@echo "Performance profiles generated: cpu.prof, mem.prof"

# Test specific file pattern
test-file: ## Test files matching pattern (usage: make test-file PATTERN=*_test.go)
	@if [ -z "$(PATTERN)" ]; then \
		echo "Usage: make test-file PATTERN=*database*"; \
		exit 1; \
	fi
	@echo "Running tests matching pattern: $(PATTERN)"
	go test -v -race -run $(PATTERN) ./...
