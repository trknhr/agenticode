# AgentiCode Hooks

AgentiCode supports hooks that allow you to execute custom commands at various points during agent execution. This is similar to Claude Code's hooks system but adapted for AgentiCode's architecture.

## Configuration

Hooks are configured in your `.agenticode.yaml` configuration file under the `hooks` section:

```yaml
hooks:
  PreToolUse:
    - matcher: "write_file|edit"     # Match write_file or edit tools
      hooks:
        - type: command
          command: "echo 'About to modify files' >> ~/.agenticode/hooks.log"
  
  PostToolUse:
    - matcher: "run_shell"           # Match run_shell tool
      hooks:
        - type: command
          command: "$AGENTICODE_PROJECT_DIR/.agenticode/hooks/check-shell.sh"
          timeout: 30                # Optional timeout in seconds
```

## Hook Events

### PreToolUse

Runs before a tool is executed. Can block or auto-approve tool execution.

**Matchers supported:**
- `write_file` - File creation/writing
- `run_shell` - Shell command execution
- `read_file`, `read` - File reading
- `edit` - File editing
- `grep`, `glob` - Search operations
- `list_files` - Directory listing
- `agent_tool` - Sub-agent spawning

### PostToolUse

Runs after a tool completes. Can provide feedback to the agent.

### UserPromptSubmit

Runs when the user submits a prompt. Can block prompts or add context.

### Stop

Runs when the agent completes its task. Can request continuation.

### SubagentStop

Runs when a sub-agent completes. Similar to Stop but for sub-agents.

### SessionStart

Runs when starting a new session. Can inject initial context.

### Notification

Runs when notifications are sent (not yet implemented).

### PreCompact

Runs before conversation compaction (not yet implemented).

## Hook Input

Hooks receive JSON via stdin with event-specific data:

```json
{
  "session_id": "session_12345",
  "transcript_path": "~/.agenticode/sessions/session_12345.jsonl",
  "cwd": "/current/working/directory",
  "hook_event_name": "PreToolUse",
  "tool_name": "write_file",
  "tool_input": {
    "path": "example.txt",
    "content": "Hello world"
  }
}
```

## Hook Output

### Simple: Exit Codes

- **Exit code 0**: Success, execution continues
- **Exit code 2**: Blocking error, feedback sent to agent
- **Other codes**: Non-blocking error, logged but execution continues

### Advanced: JSON Output

Hooks can return JSON for more control:

```json
{
  "decision": "block",
  "reason": "File pattern not allowed",
  "continue": false,
  "stopReason": "Security policy violation"
}
```

## Environment Variables

- `AGENTICODE_PROJECT_DIR`: Absolute path to the project directory

## Examples

### Example 1: Log all file modifications

```bash
#!/bin/bash
# Save as .agenticode/hooks/log-files.sh

input=$(cat)
tool_name=$(echo "$input" | jq -r '.tool_name')
file_path=$(echo "$input" | jq -r '.tool_input.path // empty')

if [[ -n "$file_path" ]]; then
    echo "[$(date)] $tool_name: $file_path" >> ~/.agenticode/file-operations.log
fi
exit 0
```

### Example 2: Add timestamp context to prompts

```bash
#!/bin/bash
# Hook for UserPromptSubmit

echo "Current time: $(date '+%Y-%m-%d %H:%M:%S')"
echo "Working directory: $(pwd)"
exit 0
```

### Example 3: Validate shell commands

```python
#!/usr/bin/env python3
import json
import sys

input_data = json.load(sys.stdin)
if input_data.get("tool_name") == "run_shell":
    command = input_data.get("tool_input", {}).get("command", "")
    
    # Block dangerous commands
    dangerous = ["rm -rf", "sudo", "chmod 777"]
    for pattern in dangerous:
        if pattern in command:
            print(f"Dangerous command pattern detected: {pattern}", file=sys.stderr)
            sys.exit(2)  # Block execution

sys.exit(0)
```

### Example 4: Auto-approve safe file reads

```python
#!/usr/bin/env python3
import json
import sys

input_data = json.load(sys.stdin)
if input_data.get("tool_name") == "read_file":
    file_path = input_data.get("tool_input", {}).get("path", "")
    
    # Auto-approve documentation files
    safe_extensions = [".md", ".txt", ".json", ".yaml"]
    if any(file_path.endswith(ext) for ext in safe_extensions):
        output = {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "allow",
                "permissionDecisionReason": "Safe file type auto-approved"
            }
        }
        print(json.dumps(output))

sys.exit(0)
```

## Security Considerations

1. **Validate inputs**: Always validate hook inputs before processing
2. **Use timeouts**: Set reasonable timeouts to prevent hanging
3. **Secure scripts**: Store hook scripts with appropriate permissions
4. **Escape variables**: Always quote shell variables properly
5. **Audit logs**: Consider logging hook executions for security auditing

## Debugging

Enable debug mode to see hook execution details:

```bash
agenticode --debug -p "your prompt"
```

This will show:
- Which hooks are triggered
- Hook execution times
- Exit codes and outputs
- Any errors encountered