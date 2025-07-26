# Todo Tools Demo

This demonstrates how the TodoWrite and TodoRead tools work in agenticode.

## Example Usage

When the LLM agent is working on a complex task, it can use these tools to track progress:

### 1. Creating Initial Todos

```json
// TodoWrite tool call
{
  "items": [
    {
      "title": "Analyze existing codebase",
      "state": "pending"
    },
    {
      "title": "Design new API endpoints", 
      "state": "pending"
    },
    {
      "title": "Implement user authentication",
      "state": "pending"
    }
  ]
}
```

### 2. Updating Progress

```json
// TodoWrite tool call with ID to update
{
  "items": [
    {
      "id": "01HGXYZ123...",
      "title": "Analyze existing codebase",
      "state": "completed"
    },
    {
      "id": "01HGXYZ124...",
      "title": "Design new API endpoints",
      "state": "in_progress"
    }
  ]
}
```

### 3. Reading Current Status

```json
// TodoRead tool call
{}
```

Returns a formatted display like:

```
📋 **Current Todos:**

**🔄 In Progress:**
- ⏳ Design new API endpoints

**📝 Pending:**
- ☐ Implement user authentication

**✅ Completed:**
- ☑ Analyze existing codebase

_Total: 3 items (1 pending, 1 in progress, 1 completed)_
```

## Benefits

1. **Transparency**: Users can see what the agent is planning and working on
2. **Progress Tracking**: Clear visibility of completed vs remaining tasks
3. **Organization**: Complex multi-step operations become manageable
4. **State Management**: The agent can track which tasks are in progress

## Integration with Agent

The agent can use these tools automatically during code generation:

```go
// In the agent's execution flow
result, err := todoWriteTool.Execute(map[string]interface{}{
    "items": []map[string]interface{}{
        {
            "title": "Create user model",
            "state": "pending",
        },
        {
            "title": "Add database migrations",
            "state": "pending",
        },
    },
})
```

This provides a Claude Code-like experience where the AI assistant manages its own task list while working on complex requests.