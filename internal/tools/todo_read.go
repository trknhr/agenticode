package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TodoReadTool reads the current todo list
type TodoReadTool struct{}

// NewTodoReadTool creates a new TodoReadTool instance
func NewTodoReadTool() *TodoReadTool {
	return &TodoReadTool{}
}

func (t *TodoReadTool) Name() string {
	return "todo_read"
}

func (t *TodoReadTool) Description() string {
	return "Display current todo list"
}

func (t *TodoReadTool) ReadOnly() bool {
	return true
}

func (t *TodoReadTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *TodoReadTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	// Get all todos
	todos := GlobalTodoStore.ReadAll()
	fmt.Println("=====================todo_read=================")

	// Sort by creation time for consistent ordering
	sort.Slice(todos, func(i, j int) bool {
		return todos[i].CreatedAt.Before(todos[j].CreatedAt)
	})

	// Create JSON for LLM
	jsonData, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal todos: %w", err)
	}

	// Create user-friendly display
	var displayLines []string
	displayLines = append(displayLines, "📋 **Current Todos:**")
	displayLines = append(displayLines, "")

	if len(todos) == 0 {
		displayLines = append(displayLines, "_No todos yet._")
	} else {
		// Group by state
		var pending, inProgress, completed []TodoItem
		for _, todo := range todos {
			switch todo.State {
			case TodoPending:
				pending = append(pending, todo)
			case TodoInProgress:
				inProgress = append(inProgress, todo)
			case TodoCompleted:
				completed = append(completed, todo)
			}
		}

		// Display in progress items first
		if len(inProgress) > 0 {
			displayLines = append(displayLines, "**🔄 In Progress:**")
			for _, todo := range inProgress {
				displayLines = append(displayLines, fmt.Sprintf("- ⏳ %s", todo.Title))
			}
			displayLines = append(displayLines, "")
		}

		// Then pending items
		if len(pending) > 0 {
			displayLines = append(displayLines, "**📝 Pending:**")
			for _, todo := range pending {
				displayLines = append(displayLines, fmt.Sprintf("- ☐ %s", todo.Title))
			}
			displayLines = append(displayLines, "")
		}

		// Finally completed items
		if len(completed) > 0 {
			displayLines = append(displayLines, "**✅ Completed:**")
			for _, todo := range completed {
				displayLines = append(displayLines, fmt.Sprintf("- ☑ %s", todo.Title))
			}
			displayLines = append(displayLines, "")
		}

		// Add summary
		displayLines = append(displayLines, fmt.Sprintf("_Total: %d items (%d pending, %d in progress, %d completed)_",
			len(todos), len(pending), len(inProgress), len(completed)))
	}

	displayContent := strings.Join(displayLines, "\n")

	return &ToolResult{
		LLMContent:    string(jsonData),
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}
