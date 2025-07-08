package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

func (t *GrepTool) Execute(args map[string]interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"status":       "success",
		"pattern":      pattern,
		"path":         path,
		"include":      include,
		"total_files":  len(matches),
		"results":      matches,
	}, nil
}