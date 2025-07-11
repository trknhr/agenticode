# Tool Approval System

The AgentiCode tool approval system provides a security layer between the LLM's tool call requests and their execution. This ensures users have full control over what actions the agent performs on their system.

## Overview

When the LLM requests to execute tools (like reading files, writing files, or running shell commands), the approval system:

1. **Intercepts** the tool calls before execution
2. **Assesses** the risk level of each tool
3. **Prompts** the user for approval (with auto-approval for safe operations)
4. **Executes** only the approved tools
5. **Reports** the results back to the LLM

## Risk Levels

Tools are categorized into three risk levels:

- 🟢 **Low Risk** (Safe, read-only operations)
  - `read_file`, `read`, `list_files`, `grep`, `glob`, `read_many_files`
  - These are auto-approved by default
  
- 🟡 **Medium Risk** (File modifications)
  - `write_file`, `edit`, `apply_patch`
  - Require explicit approval
  
- 🔴 **High Risk** (System commands)
  - `run_shell`
  - Always require explicit approval

## User Interface

When approval is needed, you'll see:

```
──────────────────────────────────────────────────────────
🔧 TOOL APPROVAL REQUEST
──────────────────────────────────────────────────────────

1. 🟡 write_file - Moderate (modifies files)
   Arguments:
   - path: /path/to/file.txt
   - content: Hello, World!

2. 🔴 run_shell - High (system commands)
   Arguments:
   - command: npm install

──────────────────────────────────────────────────────────
Options:
  y/yes    - Approve all
  n/no     - Reject all
  s/select - Choose individual tools
  i/info   - Show more details

Your choice [y/n/s/i]: 
```

## Approval Options

### Quick Approval (y/yes)
Approves all pending tool calls at once.

### Quick Rejection (n/no)
Rejects all pending tool calls.

### Selective Approval (s/select)
Allows you to choose specific tools to approve:
```
Enter the numbers of tools to approve (comma-separated), or 'all' for all, 'none' for none:
Your selection: 1,3
```

### Detailed Information (i/info)
Shows complete details about each tool call including full arguments.

## Configuration

You can configure the approval system in `~/.agenticode.yaml`:

```yaml
approval:
  mode: "interactive"        # interactive, auto, or policy
  batch_mode: "by_type"      # all, by_type, or individual
  auto_approve:
    - "read_file"
    - "read"
    - "list_files"
    - "grep"
    - "glob"
    - "read_many_files"
  require_approval:
    - "run_shell"
    - "write_file"
    - "edit"
    - "apply_patch"
  timeout: 60                # seconds
```

## Auto-Approval

By default, read-only operations are auto-approved to maintain a smooth workflow while ensuring safety. You'll see:
```
✅ Auto-approved read-only operations
```

## Examples

### Example 1: Mixed Risk Levels
```
User: Analyze the code and fix any bugs you find

Agent requests:
1. read_file (README.md) - Auto-approved ✅
2. grep (search for errors) - Auto-approved ✅
3. edit (fix bug in main.go) - Requires approval ⚠️
```

### Example 2: High Risk Operation
```
User: Install the dependencies

Agent requests:
1. read_file (package.json) - Auto-approved ✅
2. run_shell (npm install) - Requires approval ⚠️
```

## Safety Features

1. **No Execution Without Approval**: Tools are never executed without explicit or configured approval
2. **Clear Risk Indicators**: Visual indicators (🟢🟡🔴) show risk levels
3. **Detailed Information**: View full command details before approving
4. **Rejection Handling**: The agent continues gracefully when tools are rejected
5. **Audit Trail**: All approvals and rejections are logged

## Best Practices

1. **Review Commands Carefully**: Especially for `run_shell` commands
2. **Use Selective Approval**: When you want to approve some but not all tools
3. **Configure Auto-Approval**: Add frequently used safe tools to auto-approve list
4. **Check File Paths**: Ensure write operations target the intended files
5. **Understand the Context**: Read the agent's explanation before approving

## Troubleshooting

### "Tool call rejected by user"
This message appears in the conversation when you reject a tool. The agent will try to continue with alternative approaches if possible.

### Timeout Issues
If you don't respond within the timeout period (default 60s), the tools will be rejected automatically.

### Auto-Approval Not Working
Check your configuration file and ensure the tool names match exactly.

## Future Enhancements

- Policy-based approval rules
- Command history and patterns
- Sandbox execution mode
- Rollback capabilities
- Custom risk assessment rules