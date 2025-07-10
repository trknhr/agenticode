package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type GrepTool struct{}

func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for patterns in file contents using regular expressions"
}

func (t *GrepTool) ReadOnly() bool {
	return true
}

func (t *GrepTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern is required")
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	include, _ := args["include"].(string)

	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var matches []map[string]interface{}
	totalMatches := 0
	
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file matches include pattern
		if include != "" {
			matched, err := filepath.Match(include, filepath.Base(filePath))
			if err != nil || !matched {
				return nil
			}
		}

		// Search in file
		file, err := os.Open(filePath)
		if err != nil {
			return nil // Skip files we can't open
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var fileMatches []map[string]interface{}

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				fileMatches = append(fileMatches, map[string]interface{}{
					"line_number": lineNum,
					"line":        line,
					"match":       re.FindString(line),
				})
				totalMatches++
			}
		}

		if len(fileMatches) > 0 {
			matches = append(matches, map[string]interface{}{
				"file":    filePath,
				"matches": fileMatches,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	// Build LLM content
	var llmContent strings.Builder
	llmContent.WriteString(fmt.Sprintf("Found %d matches in %d files for pattern '%s'", totalMatches, len(matches), pattern))
	if len(matches) > 0 {
		llmContent.WriteString(":\n")
		for _, match := range matches {
			file := match["file"].(string)
			fileMatches := match["matches"].([]map[string]interface{})
			llmContent.WriteString(fmt.Sprintf("\n%s:\n", file))
			for _, m := range fileMatches {
				lineNum := m["line_number"].(int)
				line := m["line"].(string)
				llmContent.WriteString(fmt.Sprintf("  Line %d: %s\n", lineNum, line))
			}
		}
	}

	// Build display content
	var displayContent strings.Builder
	displayContent.WriteString(fmt.Sprintf("🔍 **Search Results** for `%s`", pattern))
	if include != "" {
		displayContent.WriteString(fmt.Sprintf(" in `%s` files", include))
	}
	displayContent.WriteString(fmt.Sprintf("\n\nFound **%d matches** in **%d files**\n", totalMatches, len(matches)))
	
	if len(matches) > 0 {
		for _, match := range matches {
			file := match["file"].(string)
			fileMatches := match["matches"].([]map[string]interface{})
			displayContent.WriteString(fmt.Sprintf("\n### 📄 %s\n```\n", file))
			for _, m := range fileMatches {
				lineNum := m["line_number"].(int)
				line := m["line"].(string)
				displayContent.WriteString(fmt.Sprintf("%4d | %s\n", lineNum, line))
			}
			displayContent.WriteString("```\n")
		}
	} else {
		displayContent.WriteString("\nNo matches found.")
	}

	return &ToolResult{
		LLMContent:    llmContent.String(),
		ReturnDisplay: displayContent.String(),
		Error:         nil,
	}, nil
}