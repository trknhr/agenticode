# agenticode

> ⚠️ **Early Development Notice**: This project is under active early development. APIs, features, and behaviors are subject to change without notice. Use at your own risk in production environments.

agenticode is a CLI tool for natural language code generation using Large Language Models (LLMs). It allows developers to generate code, create pull requests, and document repositories using simple natural language commands.

## Features

- **Natural Language Code Generation**: Generate code from plain English descriptions
- **Interactive Mode**: Chat with the agent while maintaining conversation history
- **Multi-Provider LLM Support**: Currently supports OpenAI with plans for Ollama and other providers
- **Tool System**: Extensible tool interface for file operations and shell commands
- **Evaluation Framework**: Built-in evaluation system with static checks and GPT-based code quality assessment
- **Safety Features**: Tool approval system with risk assessment, auto-approval for safe operations, and user confirmation for modifications

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/trknhr/agenticode.git
cd agenticode

# Build and install
make install
```

### Pre-built Binaries

```bash
# Build for multiple platforms
make release
```

This creates binaries for:
- Darwin (macOS): amd64, arm64
- Linux: amd64, arm64  
- Windows: amd64, arm64

## Quick Start

1. Set up your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key"
```

2. Start interactive mode:
```bash
agenticode
```

3. Or generate code directly:
```bash
agenticode code "create a simple REST API server in Go with health check endpoint"
```

3. Run with dry-run mode to preview changes:
```bash
agenticode code --dry-run "add user authentication to the REST API"
```

## Configuration

Create a configuration file at `~/.agenticode.yaml`:

```yaml
openai:
  api_key: "your-api-key"
  model: "gpt-4.1"
general:
  max_steps: 10
  confirm_before_write: true
approval:
  mode: "interactive"
  auto_approve:
    - "read_file"
    - "read"
    - "list_files"
    - "grep"
    - "glob"
    - "read_many_files"
```

## Commands

### Interactive Mode (Default)
When you run `agenticode` without any subcommands, it starts an interactive session where you can have a conversation with the agent.

```bash
agenticode
```

Interactive mode commands:
- Type your requests naturally to interact with the agent
- `exit` or `quit`: End the session
- `clear`: Clear conversation history
- `history`: View conversation history

Tool Approval:
- The agent will request approval before executing tools that modify your system
- Read-only operations are auto-approved by default
- You can approve all, reject all, or select individual tools
- See [Approval System Documentation](docs/approval-system.md) for details

### `code` - Generate Code
Generate code from natural language descriptions.

```bash
agenticode code [prompt] [flags]
```

Flags:
- `--dry-run`: Preview changes without writing files
- `--max-steps`: Maximum conversation steps (default: 10)
- `--model`: LLM model to use

### `eval` - Evaluate Code Generation (Experimental)
Run evaluation tests for code generation quality.

```bash
agenticode eval [flags]
```

Flags:
- `--config`: Path to test configuration file
- `--test-case`: Run specific test case
- `--verbose`: Show detailed output
- `--use-gpt`: Enable GPT-based evaluation
- `--save-json`: Save results to JSON file

### `propose` (Coming Soon)
Create GitHub pull requests from natural language descriptions.

### `explain` (Coming Soon)
Generate documentation for repositories.

## Development

### Prerequisites
- Go 1.24 or higher
- golangci-lint (for linting)

### Common Commands

```bash
# Build
make build

# Run tests
make test

# Run with example
make dev

# Format code
make fmt

# Run linter
make lint

# Clean build artifacts
make clean
```

### Architecture

The project follows a modular architecture:

- **Agent System** (`internal/agent/`): Orchestrates LLM interactions
- **LLM Client** (`internal/llm/`): Provider abstraction layer
- **Tools** (`internal/tools/`): Extensible tool system
- **Evaluation** (`internal/eval/`): Code quality evaluation framework
- **CLI** (`cmd/`): Cobra-based command interface

## Examples

### Generate a Todo App
```bash
agenticode code "create a React todo app with add, delete, and mark complete features"
```

### Add Tests to Existing Code
```bash
agenticode code "add unit tests for all functions in main.go"
```

### Refactor Code
```bash
agenticode code "refactor the database connection code to use connection pooling"
```

## Contributing

As this project is in early development, we welcome contributions! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## Roadmap

- [ ] Enhance generating code
- [ ] Rich CLI view with bubble tea
- [ ] Enhanced evaluation metrics
- [ ] Repository documentation generation
- [ ] GitHub integration for PR creation
- [ ] Additional LLM provider support (Ollama, Anthropic, etc.)
- [ ] Plugin system for custom tools
- [ ] Fully MCP compatible
- [ ] Web UI for code generation

## License

[License information to be added]

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [OpenAI Go SDK](https://github.com/openai/openai-go) - OpenAI API client

---

⚠️ **Remember**: This tool is under active development. Features may change, and bugs may exist. Always review generated code before using it in production.