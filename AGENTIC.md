# AGENTIC.md

This file provides guidance to agentic coding agents (e.g., agenticode) when working with code in this repository.

## Essential Commands

### Build and Development
```bash
# Build the binary
make build

# Install to GOPATH/bin
make install

# Run tests
make test

# Run tests with coverage
make coverage

# Run linter
make lint

# Format code
make fmt

# Clean build artifacts
make clean

# Run in development mode with example
make dev

# Create release builds for multiple platforms
make release
```

### Running the Agent
```bash
# Interactive mode (default)
./agenticode

# Execute a single prompt
./agenticode -p "create a REST API server"

# Use specific model
./agenticode -m fast -p "optimize this code"

# Dry run mode (preview changes)
./agenticode code --dry-run "add tests"
```

### Testing Evaluation
```bash
# Run single evaluation test
make eval

# Run all evaluation tests
make eval-all

# Run with verbose output
make eval-verbose

# Generate evaluation report
make eval-report
```

## Architecture Overview

### Core Components

**Agent System** (`internal/agent/`)
- Event-driven architecture with `Agent` struct as the main orchestrator
- Handles LLM interactions, tool execution, and conversation management
- Supports sub-agents for parallel task execution

**LLM Abstraction** (`internal/llm/`)
- Multi-provider support (OpenAI, DeepSeek, Groq, local)
- Configuration-driven model selection via YAML
- Provider-agnostic client interface

**Tool System** (`internal/tools/`)
- Extensible tool registry with common operations
- Tools: read, write, edit, grep, glob, run_shell, web_fetch, todo_* operations
- Each tool implements the `Tool` interface with Name/Description/Execute

**Hook System** (`internal/hooks/`)
- Lifecycle hooks for tool execution, user prompts, session management
- Configurable via YAML for custom workflows
- Supports command execution and external integrations

### Configuration System

**Providers Configuration**
- Multi-provider YAML configuration in `~/.agenticode.yaml`
- Supports different models per provider with context windows and token limits
- Environment variable substitution for API keys

**Model Selection**
- Named model selections (default, fast, powerful, summarize, local)
- Can be overridden via command line with `-m` flag

### Command Structure

**Root Command** (`cmd/root.go`)
- Uses Cobra CLI framework
- Handles both interactive and non-interactive modes
- Configures tool permissions and approval system

**Interactive Mode Features**
- Conversation history management with `clear`, `compact`, `history` commands
- Built-in `init` command for AGENTIC.md generation
- Todo system integration with `todos` command

### Key Files

- `main.go` - Entry point, delegates to `cmd/root.go`
- `cmd/root.go` - CLI command handling and configuration
- `internal/agent/agent.go` - Main agent orchestration
- `internal/llm/client.go` - LLM provider abstraction
- `internal/tools/tools.go` - Tool system registry and utilities

### Development Setup

1. Install dependencies: `go mod download`
2. Set up configuration: `cp .agenticode.yaml.example ~/.agenticode.yaml`
3. Configure API keys in `~/.agenticode.yaml`
4. Build: `make build`
5. Test: `make test`