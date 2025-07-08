package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func (t *GlobTool) Execute(args map[string]interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"status":  "success",
		"pattern": pattern,
		"path":    path,
		"matches": sortedMatches,
		"count":   len(sortedMatches),
	}, nil
}