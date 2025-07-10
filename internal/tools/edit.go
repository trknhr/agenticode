package tools

import (
	"fmt"
	"os"
	"strings"
)

type EditTool struct{}

func NewEditTool() *EditTool {
	return &EditTool{}
}

func (t *EditTool) Name() string {
	return "edit"
}

func (t *EditTool) Description() string {
	return "Replace exact string matches in files"
}

func (t *EditTool) ReadOnly() bool {
	return false
}

func (t *EditTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("file_path is required")
	}

	oldString, ok := args["old_string"].(string)
	if !ok {
		return nil, fmt.Errorf("old_string is required")
	}

	newString, ok := args["new_string"].(string)
	if !ok {
		return nil, fmt.Errorf("new_string is required")
	}

	replaceAll, _ := args["replace_all"].(bool)

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fileContent := string(content)
	originalContent := fileContent

	// Check if old_string exists in the file
	if !strings.Contains(fileContent, oldString) {
		return nil, fmt.Errorf("old_string not found in file")
	}

	// Check if old_string is unique (when not replace_all)
	if !replaceAll && strings.Count(fileContent, oldString) > 1 {
		return nil, fmt.Errorf("old_string is not unique in the file. Use replace_all=true or provide more context")
	}

	// Perform replacement
	var updatedContent string
	var replacements int

	if replaceAll {
		updatedContent = strings.ReplaceAll(fileContent, oldString, newString)
		replacements = strings.Count(fileContent, oldString)
	} else {
		updatedContent = strings.Replace(fileContent, oldString, newString, 1)
		replacements = 1
	}

	// Check if content actually changed
	if updatedContent == originalContent {
		return nil, fmt.Errorf("no changes made - old_string and new_string might be identical")
	}

	// Write the updated content back
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &ToolResult{
		LLMContent:    fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", replacements, filePath),
		ReturnDisplay: fmt.Sprintf("✅ **Edited** `%s`\n\nReplaced **%d occurrence(s)** of the specified string.", filePath, replacements),
		Error:         nil,
	}, nil
}