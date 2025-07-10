package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReadManyFilesTool struct{}

func NewReadManyFilesTool() *ReadManyFilesTool {
	return &ReadManyFilesTool{}
}

func (t *ReadManyFilesTool) Name() string {
	return "read_many_files"
}

func (t *ReadManyFilesTool) Description() string {
	return "Read contents from multiple files at once"
}

func (t *ReadManyFilesTool) ReadOnly() bool {
	return true
}

func (t *ReadManyFilesTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	// Accept either "paths" (array) or "patterns" (array of glob patterns)
	var filePaths []string

	// Check for explicit paths
	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if path, ok := p.(string); ok {
				filePaths = append(filePaths, path)
			}
		}
	}

	// Check for glob patterns
	if patterns, ok := args["patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if pattern, ok := p.(string); ok {
				matches, err := filepath.Glob(pattern)
				if err != nil {
					continue // Skip invalid patterns
				}
				filePaths = append(filePaths, matches...)
			}
		}
	}

	// If neither paths nor patterns provided
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("either 'paths' or 'patterns' array is required")
	}

	// Remove duplicates
	uniquePaths := make(map[string]bool)
	for _, path := range filePaths {
		uniquePaths[path] = true
	}

	// Read all files
	var results []map[string]interface{}
	var errors []string

	for path := range uniquePaths {
		content, err := os.ReadFile(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: stat error: %v", path, err))
			continue
		}

		results = append(results, map[string]interface{}{
			"path":    path,
			"content": string(content),
			"size":    info.Size(),
		})
	}

	// Build LLM content
	var llmContent strings.Builder
	llmContent.WriteString(fmt.Sprintf("Read %d files", len(results)))
	if len(errors) > 0 {
		llmContent.WriteString(fmt.Sprintf(" (%d errors)", len(errors)))
	}
	llmContent.WriteString(":\n")
	
	for _, result := range results {
		path := result["path"].(string)
		content := result["content"].(string)
		llmContent.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", path, content))
	}
	
	if len(errors) > 0 {
		llmContent.WriteString("\nErrors:\n")
		for _, err := range errors {
			llmContent.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	// Build display content
	var displayContent strings.Builder
	displayContent.WriteString(fmt.Sprintf("📚 **Read %d files**", len(results)))
	if len(errors) > 0 {
		displayContent.WriteString(fmt.Sprintf(" (⚠️ %d errors)", len(errors)))
	}
	displayContent.WriteString("\n\n")
	
	for _, result := range results {
		path := result["path"].(string)
		content := result["content"].(string)
		size := result["size"].(int64)
		lines := strings.Count(content, "\n") + 1
		
		displayContent.WriteString(fmt.Sprintf("### 📄 %s\n", path))
		displayContent.WriteString(fmt.Sprintf("*%d lines, %d bytes*\n", lines, size))
		displayContent.WriteString("```\n")
		
		// Add line numbers for display
		for i, line := range strings.Split(content, "\n") {
			displayContent.WriteString(fmt.Sprintf("%4d | %s\n", i+1, line))
		}
		displayContent.WriteString("```\n\n")
	}
	
	if len(errors) > 0 {
		displayContent.WriteString("### ⚠️ Errors:\n")
		for _, err := range errors {
			displayContent.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return &ToolResult{
		LLMContent:    llmContent.String(),
		ReturnDisplay: displayContent.String(),
		Error:         nil,
	}, nil
}