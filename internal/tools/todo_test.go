package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTodoTools(t *testing.T) {
	// Clear any existing todos
	GlobalTodoStore.Clear()

	// Test TodoWriteTool
	writeTool := NewTodoWriteTool()
	
	// Test creating new todos
	writeArgs := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"title": "Implement feature X",
				"state": "pending",
			},
			{
				"title": "Write tests for feature X",
				"state": "pending",
			},
			{
				"title": "Review code",
				"state": "in_progress",
			},
		},
	}
	
	writeResult, err := writeTool.Execute(writeArgs)
	if err != nil {
		t.Fatalf("TodoWriteTool.Execute() failed: %v", err)
	}
	
	if !strings.Contains(writeResult.LLMContent, "3 todo items") {
		t.Errorf("Expected LLMContent to mention 3 items, got: %s", writeResult.LLMContent)
	}
	
	// Test TodoReadTool
	readTool := NewTodoReadTool()
	readResult, err := readTool.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("TodoReadTool.Execute() failed: %v", err)
	}
	
	// Parse the JSON response
	var todos []TodoItem
	if err := json.Unmarshal([]byte(readResult.LLMContent), &todos); err != nil {
		t.Fatalf("Failed to parse TodoReadTool JSON response: %v", err)
	}
	
	if len(todos) != 3 {
		t.Errorf("Expected 3 todos, got %d", len(todos))
	}
	
	// Verify display content
	if !strings.Contains(readResult.ReturnDisplay, "In Progress:") {
		t.Errorf("Expected display to contain 'In Progress:', got: %s", readResult.ReturnDisplay)
	}
	
	if !strings.Contains(readResult.ReturnDisplay, "Pending:") {
		t.Errorf("Expected display to contain 'Pending:', got: %s", readResult.ReturnDisplay)
	}
	
	// Test updating existing todo
	// First, get the ID of one todo
	firstTodoID := todos[0].ID
	
	updateArgs := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"id":    firstTodoID,
				"title": "Implement feature X",
				"state": "completed",
			},
		},
	}
	
	updateResult, err := writeTool.Execute(updateArgs)
	if err != nil {
		t.Fatalf("TodoWriteTool.Execute() for update failed: %v", err)
	}
	
	if !strings.Contains(updateResult.LLMContent, "updated 1 existing") {
		t.Errorf("Expected update message, got: %s", updateResult.LLMContent)
	}
	
	// Read again to verify update
	readResult2, err := readTool.Execute(map[string]interface{}{})
	if err != nil {
		t.Fatalf("TodoReadTool.Execute() after update failed: %v", err)
	}
	
	if !strings.Contains(readResult2.ReturnDisplay, "Completed:") {
		t.Errorf("Expected display to contain 'Completed:', got: %s", readResult2.ReturnDisplay)
	}
}

func TestTodoWriteValidation(t *testing.T) {
	writeTool := NewTodoWriteTool()
	
	// Test missing items parameter
	_, err := writeTool.Execute(map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "missing required parameter") {
		t.Errorf("Expected missing parameter error, got: %v", err)
	}
	
	// Test empty items array
	_, err = writeTool.Execute(map[string]interface{}{
		"items": []map[string]interface{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected empty items error, got: %v", err)
	}
	
	// Test empty title
	_, err = writeTool.Execute(map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"title": "",
				"state": "pending",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "title cannot be empty") {
		t.Errorf("Expected empty title error, got: %v", err)
	}
	
	// Test invalid state
	_, err = writeTool.Execute(map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"title": "Test",
				"state": "invalid_state",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Errorf("Expected invalid state error, got: %v", err)
	}
}