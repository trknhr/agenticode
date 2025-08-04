package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MultiEditTool struct{}

func NewMultiEditTool() *MultiEditTool {
	return &MultiEditTool{}
}

func (t *MultiEditTool) Name() string {
	return "multi_edit"
}

func (t *MultiEditTool) Description() string {
	return "Make multiple edits to a single file in one operation"
}

func (t *MultiEditTool) ReadOnly() bool {
	return false
}

func (t *MultiEditTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
			"edits": map[string]interface{}{
				"type":        "array",
				"description": "Array of edit operations to perform sequentially on the file",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"old_string": map[string]interface{}{
							"type":        "string",
							"description": "The text to replace",
						},
						"new_string": map[string]interface{}{
							"type":        "string",
							"description": "The text to replace it with",
						},
						"replace_all": map[string]interface{}{
							"type":        "boolean",
							"description": "Replace all occurrences of old_string (default false)",
							"default":     false,
						},
					},
					"required": []string{"old_string", "new_string"},
				},
			},
		},
		"required": []string{"file_path", "edits"},
	}
}

func (t *MultiEditTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("file_path is required and must be a string")
	}

	editsRaw, ok := args["edits"]
	if !ok {
		return nil, fmt.Errorf("edits is required")
	}

	edits, ok := editsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("edits must be an array")
	}

	if len(edits) == 0 {
		return nil, fmt.Errorf("edits array cannot be empty")
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		// Check if file doesn't exist and first edit has empty old_string (file creation)
		if os.IsNotExist(err) && len(edits) > 0 {
			firstEdit, ok := edits[0].(map[string]interface{})
			if ok {
				oldString, _ := firstEdit["old_string"].(string)
				if oldString == "" {
					// This is a file creation, start with empty content
					content = []byte{}
				} else {
					return nil, fmt.Errorf("failed to read file: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to read file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	}

	fileContent := string(content)
	originalContent := fileContent

	// Track all replacements
	totalReplacements := 0
	editResults := []string{}

	// Apply each edit in sequence
	for i, editRaw := range edits {
		edit, ok := editRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("edit at index %d must be an object", i)
		}

		oldString, ok := edit["old_string"].(string)
		if !ok {
			return nil, fmt.Errorf("old_string is required for edit at index %d", i)
		}

		newString, ok := edit["new_string"].(string)
		if !ok {
			return nil, fmt.Errorf("new_string is required for edit at index %d", i)
		}

		replaceAll, _ := edit["replace_all"].(bool)

		// Special case for file creation
		if i == 0 && oldString == "" && originalContent == "" {
			fileContent = newString
			editResults = append(editResults, "Created new file")
			totalReplacements++
			continue
		}

		// Check if old_string and new_string are the same
		if oldString == newString {
			return nil, fmt.Errorf("edit at index %d: old_string and new_string are identical", i)
		}

		// Check if old_string exists in the current content
		if !strings.Contains(fileContent, oldString) {
			return nil, fmt.Errorf("edit at index %d: old_string not found in file", i)
		}

		// Check if old_string is unique (when not replace_all)
		occurrences := strings.Count(fileContent, oldString)
		if !replaceAll && occurrences > 1 {
			return nil, fmt.Errorf("edit at index %d: old_string is not unique in the file (found %d occurrences). Use replace_all=true or provide more context", i, occurrences)
		}

		// Perform replacement
		var replacements int
		if replaceAll {
			fileContent = strings.ReplaceAll(fileContent, oldString, newString)
			replacements = occurrences
		} else {
			fileContent = strings.Replace(fileContent, oldString, newString, 1)
			replacements = 1
		}

		totalReplacements += replacements
		editResults = append(editResults, fmt.Sprintf("Edit %d: replaced %d occurrence(s)", i+1, replacements))
	}

	// Check if content actually changed
	if fileContent == originalContent && originalContent != "" {
		return nil, fmt.Errorf("no changes made after applying all edits")
	}

	// Create directory if needed
	dir := strings.TrimSpace(filePath)
	if dir != "" {
		dirPath := filepath.Dir(dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Write the updated content back
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Build result message
	resultDetails := strings.Join(editResults, "\n")

	return &ToolResult{
		LLMContent:    fmt.Sprintf("Successfully applied %d edits to %s with %d total replacements", len(edits), filePath, totalReplacements),
		ReturnDisplay: fmt.Sprintf("✅ **Multi-edited** `%s`\n\nApplied **%d edits** with **%d total replacements**:\n%s", filePath, len(edits), totalReplacements, resultDetails),
		Error:         nil,
	}, nil
}
