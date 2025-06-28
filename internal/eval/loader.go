package eval

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadTestCase loads a single test case from a YAML file
func LoadTestCase(path string) (*TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test case file: %w", err)
	}

	var tc TestCase
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("failed to parse test case YAML: %w", err)
	}

	// Set name from filename if not specified
	if tc.Name == "" {
		tc.Name = filepath.Base(path)
	}

	// Use description as prompt if prompt not specified
	if tc.Prompt == "" {
		tc.Prompt = tc.Description
	}

	return &tc, nil
}

// LoadTestCases loads all test cases from a directory
func LoadTestCases(dir string) ([]*TestCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read test directory: %w", err)
	}

	var testCases []*TestCase
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		tc, err := LoadTestCase(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load test case %s: %w", entry.Name(), err)
		}
		testCases = append(testCases, tc)
	}

	return testCases, nil
}
