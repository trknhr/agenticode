package tools

import (
	"encoding/json"
	"fmt"
)

// TodoWriteTool allows writing or updating todo items
type TodoWriteTool struct{}

// NewTodoWriteTool creates a new TodoWriteTool instance
func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{}
}

func (t *TodoWriteTool) Name() string {
	return "todo_write"
}

func (t *TodoWriteTool) Description() string {
	return "Write or update todo items with title and state"
}

func (t *TodoWriteTool) ReadOnly() bool {
	return false
}

func (t *TodoWriteTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type":        "array",
				"description": "List of todo items to write or update",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Optional existing todo ID (leave empty to create new)",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Todo title/description",
						},
						"state": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed"},
							"description": "State of the todo item",
						},
					},
					"required": []string{"title", "state"},
				},
			},
		},
		"required": []string{"items"},
	}
}

func (t *TodoWriteTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	rawItems, ok := args["items"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'items'")
	}

	// Convert the raw items to JSON and back to properly typed structs
	jsonBytes, err := json.Marshal(rawItems)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	var items []TodoItem
	if err := json.Unmarshal(jsonBytes, &items); err != nil {
		return nil, fmt.Errorf("failed to parse todo items: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("items array cannot be empty")
	}

	// Validate items
	for i, item := range items {
		if item.Title == "" {
			return nil, fmt.Errorf("item %d: title cannot be empty", i)
		}
		if item.State == "" {
			items[i].State = TodoPending // Default to pending if not specified
		}
		// Validate state
		switch item.State {
		case TodoPending, TodoInProgress, TodoCompleted:
			// Valid state
		default:
			return nil, fmt.Errorf("item %d: invalid state '%s'", i, item.State)
		}
	}

	// Upsert the items
	GlobalTodoStore.Upsert(items)

	// Count actions
	newCount := 0
	updateCount := 0
	for _, item := range items {
		if item.ID == "" {
			newCount++
		} else {
			updateCount++
		}
	}

	// Build response messages
	var actions []string
	if newCount > 0 {
		actions = append(actions, fmt.Sprintf("created %d new", newCount))
	}
	if updateCount > 0 {
		actions = append(actions, fmt.Sprintf("updated %d existing", updateCount))
	}

	actionSummary := ""
	if len(actions) > 0 {
		actionSummary = " (" + actions[0]
		if len(actions) > 1 {
			actionSummary += ", " + actions[1]
		}
		actionSummary += ")"
	}

	llmContent := fmt.Sprintf("Successfully wrote %d todo items%s", len(items), actionSummary)
	displayContent := fmt.Sprintf("✅ Todo list updated: %d items%s", len(items), actionSummary)

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}
