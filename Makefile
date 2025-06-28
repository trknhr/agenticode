.PHONY: build run test clean install deps fmt lint help eval eval-all eval-verbose eval-report debug-eval

# Variables
BINARY_NAME=agenticode
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) .

## run: Run the application
run: build
	./$(BINARY_NAME)

## test: Run tests
test:
	$(GO) test $(GOFLAGS) ./...

## test-verbose: Run tests with verbose output
test-verbose:
	$(GO) test -v ./...

## coverage: Run tests with coverage
coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## deps: Download dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## fmt: Format code
fmt:
	$(GO) fmt ./...

## lint: Run linter
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## install: Install the binary to GOPATH/bin
install: build
	$(GO) install

## clean: Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf dist/
	rm -rf .agenticode_output/

## dev: Run with example (requires OPENAI_API_KEY)
dev: build
	./$(BINARY_NAME) code  --config ./.agenticode.yaml "Create a simple hello world in Go"

## dev-react: Run React todo app example
dev-react: build
	./$(BINARY_NAME) code "Create a React todo list with add/complete/delete functionality"

## dev-dry: Run in dry-run mode
dev-dry: build
	./$(BINARY_NAME) code --dry-run "Create a REST API server in Go with user CRUD operations"

## eval: Run a single evaluation test
eval: build
	./$(BINARY_NAME) eval tests/codegen/http-server.yaml --config ./.agenticode.yaml

## debug-eval: Debug eval command with dlv
debug-eval:
	dlv debug . -- eval tests/codegen/http-server.yaml --config ./.agenticode.yaml

## eval-all: Run all evaluation tests
eval-all: build
	./$(BINARY_NAME) eval-all tests/codegen/

## eval-verbose: Run all tests with verbose output
eval-verbose: build
	./$(BINARY_NAME) eval-all tests/codegen/ --verbose --keep-failed

## eval-report: Run all tests and save JSON report
eval-report: build
	./$(BINARY_NAME) eval-all tests/codegen/ --save-json=eval-results.json --verbose
	@echo "Evaluation results saved to eval-results.json"

## release: Build for multiple platforms
release:
	@echo "Building for multiple platforms..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Release builds completed in dist/"