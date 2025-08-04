# AgentiCode Hooks Implementation Summary

This document summarizes the implementation of the hooks system in AgentiCode, which allows users to execute custom commands at various points during agent execution.

## Architecture Overview

### Core Components

1. **Hook Types** (`internal/hooks/types.go`)
   - Defines hook events, input/output structures
   - Supports 8 event types: PreToolUse, PostToolUse, UserPromptSubmit, Stop, SubagentStop, SessionStart, Notification, PreCompact

2. **Hook Manager** (`internal/hooks/manager.go`)
   - Manages hook execution with timeout support
   - Handles parallel execution of multiple hooks
   - Processes hook outputs (exit codes and JSON)

3. **Hook Configuration** (`internal/hooks/config.go`)
   - Loads hooks from YAML configuration files
   - Validates hook configuration
   - Supports multiple config file locations

### Integration Points

1. **Agent Integration** (`internal/agent/agent.go`)
   - Added `hookManager` field to Agent struct
   - Executes Stop/SubagentStop hooks after execution
   - Passes hook manager to turn handlers

2. **Turn Handler Integration** (`internal/agent/handlers.go`)
   - PreToolUse hooks before tool execution
   - PostToolUse hooks after tool completion
   - Supports blocking and auto-approval

3. **Command Integration** (`cmd/root.go`)
   - Loads hook configuration from viper
   - UserPromptSubmit hooks for both interactive and non-interactive modes
   - Creates hook manager with session context

## Configuration Example

```yaml
hooks:
  PreToolUse:
    - matcher: "write_file|edit"
      hooks:
        - type: command
          command: "echo 'Modifying files' >> log.txt"
          timeout: 30
  
  UserPromptSubmit:
    - hooks:
        - type: command
          command: "date '+Current time: %Y-%m-%d %H:%M:%S'"
```

## Hook Flow

1. **Hook Execution**:
   - Event occurs → Manager finds matching hooks → Execute in parallel
   - Each hook receives JSON input via stdin
   - Hooks return exit codes or JSON output

2. **Exit Code Handling**:
   - 0: Success, continue execution
   - 2: Blocking error, provide feedback
   - Other: Non-blocking error, log and continue

3. **JSON Output Support**:
   - Advanced control with structured responses
   - Support for decisions (allow/deny/block)
   - Additional context injection

## Example Hooks

The implementation includes example hook scripts in `.agenticode/hooks/`:
- `check-style.sh`: Validates Go code formatting
- `validate-commands.py`: Security validation for shell commands
- `add-context.sh`: Adds timestamp and environment context

## Testing

- `test_hooks.sh`: Demonstrates various hook scenarios
- `.agenticode.hooks.example.yaml`: Example configuration file

## Security Considerations

- Hooks execute with user permissions
- Timeout protection (default 60s)
- Input validation in hook scripts
- Environment variable isolation

## Future Enhancements

1. **Notification** and **PreCompact** events are defined but not yet integrated
2. **SessionStart** hook for initial context loading
3. MCP tool support (tools with `mcp__` prefix)
4. Hook chaining and dependencies
5. Async hook execution for non-blocking operations

## Usage

1. Copy `.agenticode.hooks.example.yaml` to `~/.agenticode.yaml`
2. Create hook scripts in `.agenticode/hooks/`
3. Make scripts executable with `chmod +x`
4. Run agenticode with `--debug` to see hook execution

The hooks system provides a powerful extension mechanism while maintaining security and performance.