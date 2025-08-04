package agent

import (
	"strings"
	"testing"
)

func TestGetDeveloperPrompt(t *testing.T) {
	// Test that GetDeveloperPrompt returns content
	prompt := GetDeveloperPrompt()

	// Check that prompt is not empty
	if prompt == "" {
		t.Error("GetDeveloperPrompt returned empty string")
	}

	// Check that it contains expected content from developer-prompt.md
	if !strings.Contains(prompt, "run_shell") {
		t.Error("Developer prompt doesn't contain expected 'run_shell' content")
	}

	if !strings.Contains(prompt, "Committing changes with git") {
		t.Error("Developer prompt doesn't contain expected git commit instructions")
	}

	// Log the first 200 characters for debugging
	if len(prompt) > 200 {
		t.Logf("Developer prompt starts with: %s...", prompt[:200])
	} else {
		t.Logf("Developer prompt: %s", prompt)
	}
}

func TestGetSystemPrompt(t *testing.T) {
	// Test that GetSystemPrompt returns content with proper template processing
	modelName := "test-model"
	prompt := GetSystemPrompt(modelName)

	// Check that prompt is not empty
	if prompt == "" {
		t.Error("GetSystemPrompt returned empty string")
	}

	// Check that template was processed (no template variables left)
	if strings.Contains(prompt, "{{") || strings.Contains(prompt, "}}") {
		t.Error("GetSystemPrompt contains unprocessed template variables")
	}

	// Check that model name was injected
	if !strings.Contains(prompt, modelName) {
		t.Error("GetSystemPrompt doesn't contain the provided model name")
	}
}
