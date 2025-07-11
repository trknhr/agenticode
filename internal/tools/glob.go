package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type GlobTool struct{}

func NewGlobTool() *GlobTool {
	return &GlobTool{}
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Description() string {
	return "Find files matching glob patterns"
}

func (t *GlobTool) ReadOnly() bool {
	return true
}

func (t *GlobTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "The glob pattern to match files against",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory to search in (defaults to current directory)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern is required")
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	var matches []string
	
	// If pattern contains directory separators, use filepath.Glob
	if filepath.IsAbs(pattern) || filepath.Dir(pattern) != "." {
		// Pattern includes path components
		fullPattern := filepath.Join(path, pattern)
		globMatches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern: %w", err)
		}
		matches = globMatches
	} else {
		// Walk directory tree and match pattern against filenames
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files we can't access
			}

			// Check if the base name matches the pattern
			matched, err := filepath.Match(pattern, filepath.Base(filePath))
			if err != nil {
				return fmt.Errorf("pattern matching error: %w", err)
			}

			if matched {
				matches = append(matches, filePath)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory: %w", err)
		}
	}

	// Sort matches by modification time (newest first)
	type fileInfo struct {
		path    string
		modTime int64
	}

	var fileInfos []fileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil {
			fileInfos = append(fileInfos, fileInfo{
				path:    match,
				modTime: info.ModTime().Unix(),
			})
		}
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime > fileInfos[j].modTime
	})

	sortedMatches := make([]string, len(fileInfos))
	for i, fi := range fileInfos {
		sortedMatches[i] = fi.path
	}

	// Build LLM content
	llmContent := fmt.Sprintf("Found %d files matching pattern '%s' in %s", len(sortedMatches), pattern, path)
	if len(sortedMatches) > 0 {
		llmContent += fmt.Sprintf(": %s", strings.Join(sortedMatches, ", "))
		if len(sortedMatches) > 10 {
			llmContent = fmt.Sprintf("Found %d files matching pattern '%s' in %s: %s... and %d more", 
				len(sortedMatches), pattern, path, strings.Join(sortedMatches[:10], ", "), len(sortedMatches)-10)
		}
	}

	// Build display content
	displayContent := fmt.Sprintf("🔍 **Glob Results** for `%s` in `%s`\n\nFound **%d files**\n", pattern, path, len(sortedMatches))
	if len(sortedMatches) > 0 {
		displayContent += "```\n"
		for _, match := range sortedMatches {
			displayContent += match + "\n"
		}
		displayContent += "```"
	} else {
		displayContent += "\nNo files found matching the pattern."
	}

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}