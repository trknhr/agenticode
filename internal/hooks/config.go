package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigPaths defines the order of configuration files to check
var ConfigPaths = []string{
	".agenticode/settings.local.json", // Local project settings (not committed)
	".agenticode/settings.json",       // Project settings
	"~/.agenticode/settings.json",     // User settings
}

// Settings represents the complete settings structure
type Settings struct {
	Hooks  *HookConfig            `json:"hooks,omitempty"`
	OpenAI map[string]interface{} `json:"openai,omitempty"`
	// Add other settings fields as needed
}

// LoadHookConfig loads hook configuration from settings files
func LoadHookConfig(projectDir string) (*HookConfig, error) {
	// Expand home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	var mergedConfig *HookConfig

	// Load and merge configurations in order
	for _, path := range ConfigPaths {
		// Expand ~ to home directory
		if path[0] == '~' {
			path = filepath.Join(homeDir, path[1:])
		} else {
			// Relative paths are relative to project directory
			path = filepath.Join(projectDir, path)
		}

		config, err := loadConfigFromFile(path)
		if err != nil {
			// File not existing is not an error
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
		}

		if config.Hooks != nil {
			mergedConfig = mergeHookConfig(mergedConfig, config.Hooks)
		}
	}

	return mergedConfig, nil
}

// loadConfigFromFile loads settings from a single file
func loadConfigFromFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	return &settings, nil
}

// mergeHookConfig merges two hook configurations
func mergeHookConfig(base, override *HookConfig) *HookConfig {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	// Create a new config with merged values
	merged := &HookConfig{
		PreToolUse:       append(base.PreToolUse, override.PreToolUse...),
		PostToolUse:      append(base.PostToolUse, override.PostToolUse...),
		UserPromptSubmit: append(base.UserPromptSubmit, override.UserPromptSubmit...),
		Notification:     append(base.Notification, override.Notification...),
		Stop:             append(base.Stop, override.Stop...),
		SubagentStop:     append(base.SubagentStop, override.SubagentStop...),
		PreCompact:       append(base.PreCompact, override.PreCompact...),
		SessionStart:     append(base.SessionStart, override.SessionStart...),
	}

	return merged
}

// ValidateHookConfig validates a hook configuration
func ValidateHookConfig(config *HookConfig) error {
	if config == nil {
		return nil
	}

	// Validate all hook matchers
	matchers := []struct {
		name     string
		matchers []HookMatcher
	}{
		{"PreToolUse", config.PreToolUse},
		{"PostToolUse", config.PostToolUse},
		{"UserPromptSubmit", config.UserPromptSubmit},
		{"Notification", config.Notification},
		{"Stop", config.Stop},
		{"SubagentStop", config.SubagentStop},
		{"PreCompact", config.PreCompact},
		{"SessionStart", config.SessionStart},
	}

	for _, m := range matchers {
		for i, matcher := range m.matchers {
			if err := validateHookMatcher(m.name, i, matcher); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateHookMatcher validates a single hook matcher
func validateHookMatcher(event string, index int, matcher HookMatcher) error {
	// Validate hooks
	if len(matcher.Hooks) == 0 {
		return fmt.Errorf("%s[%d]: no hooks defined", event, index)
	}

	for j, hook := range matcher.Hooks {
		if hook.Type != "" && hook.Type != "command" {
			return fmt.Errorf("%s[%d].hooks[%d]: unsupported type %q (only \"command\" is supported)",
				event, index, j, hook.Type)
		}
		if hook.Command == "" {
			return fmt.Errorf("%s[%d].hooks[%d]: command is required", event, index, j)
		}
	}

	// Validate matcher pattern for tool events
	if event == "PreToolUse" || event == "PostToolUse" {
		if matcher.Matcher != "" && matcher.Matcher != "*" {
			// Try to compile as regex to validate
			_, err := json.Marshal(matcher.Matcher)
			if err != nil {
				return fmt.Errorf("%s[%d]: invalid matcher pattern: %w", event, index, err)
			}
		}
	}

	return nil
}
