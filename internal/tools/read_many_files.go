package tools

import (
	"fmt"
	"os"
	"path/filepath"
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

func (t *ReadManyFilesTool) Execute(args map[string]interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"status":      "success",
		"files_read":  len(results),
		"errors":      errors,
		"results":     results,
	}, nil
}