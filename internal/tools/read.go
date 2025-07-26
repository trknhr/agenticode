package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadTool is a simple tool for reading file contents
// Input parameters:
// {
//   // The absolute path to the file to read
//   file_path: string;
//   // The line number to start reading from. Only provide if the file is too large to read at once
//   offset?: number;
//   // The number of lines to read. Only provide if the file is too large to read at once.
//   limit?: number;
// }
type ReadTool struct{}

func NewReadTool() *ReadTool {
	return &ReadTool{}
}

func (t *ReadTool) Name() string {
	return "read"
}

func (t *ReadTool) Description() string {
	return "Read a file and return its full content (simple version without line numbers)"
}

func (t *ReadTool) ReadOnly() bool {
	return true
}

func (t *ReadTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to read",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "The line number to start reading from. Only provide if the file is too large to read at once",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "The number of lines to read. Only provide if the file is too large to read at once",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	// Get the file path
	path, ok := args["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("file_path is required")
	}

	// Convert to absolute path if needed
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}
		path = absPath
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	fileSize := info.Size()

	// Build simple LLM content
	llmContent := fmt.Sprintf("Content of %s:\n%s", path, contentStr)

	// Build simple display content
	displayContent := fmt.Sprintf("📄 **%s** (%d bytes)\n\n%s", path, fileSize, contentStr)

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}
