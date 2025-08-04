package llm

import (
	"fmt"
	"os"
	"strings"
)

// ProviderConfig represents a single LLM provider configuration
type ProviderConfig struct {
	Type    string        `yaml:"type" json:"type" mapstructure:"type"`             // Provider type: "openai", "anthropic", etc.
	BaseURL string        `yaml:"base_url" json:"base_url" mapstructure:"base_url"` // Base URL for the API
	APIKey  string        `yaml:"api_key" json:"api_key" mapstructure:"api_key"`    // API key (can use $ENV_VAR syntax)
	Models  []ModelConfig `yaml:"models" json:"models" mapstructure:"models"`       // Available models for this provider
}

// ModelConfig represents a single model configuration
type ModelConfig struct {
	ID            string `yaml:"id" json:"id" mapstructure:"id"`                                     // Model identifier (e.g., "gpt-4", "deepseek-chat")
	Name          string `yaml:"name" json:"name" mapstructure:"name"`                               // Human-readable name
	ContextWindow int    `yaml:"context_window" json:"context_window" mapstructure:"context_window"` // Maximum context size
	MaxTokens     int    `yaml:"max_tokens" json:"max_tokens" mapstructure:"max_tokens"`             // Default max tokens for responses
}

// ModelSelection represents a model choice with provider and model ID
type ModelSelection struct {
	Provider string `yaml:"provider" json:"provider" mapstructure:"provider"` // Provider name from the providers map
	Model    string `yaml:"model" json:"model" mapstructure:"model"`          // Model ID from the provider's models list
}

// ProvidersConfig represents the complete providers configuration
type ProvidersConfig struct {
	Providers map[string]ProviderConfig `yaml:"providers" json:"providers"` // Map of provider name to config
	Models    map[string]ModelSelection `yaml:"models" json:"models"`       // Named model selections (e.g., "default", "fast")
}

// ExpandEnvVars expands environment variables in the format $VAR_NAME
func ExpandEnvVars(value string) string {
	if strings.HasPrefix(value, "$") {
		envVar := strings.TrimPrefix(value, "$")
		if val, exists := os.LookupEnv(envVar); exists {
			return val
		}
	}
	return value
}

// ExpandProviderConfig expands environment variables in provider configuration
func ExpandProviderConfig(config *ProviderConfig) {
	config.APIKey = ExpandEnvVars(config.APIKey)
	// BaseURL could also contain env vars in some cases
	config.BaseURL = os.ExpandEnv(config.BaseURL)
}

// FindModel searches for a model in the provider configuration
func (p *ProvidersConfig) FindModel(providerName, modelID string) (*ProviderConfig, *ModelConfig, error) {
	provider, exists := p.Providers[providerName]
	if !exists {
		return nil, nil, fmt.Errorf("provider %s not found", providerName)
	}

	for i := range provider.Models {
		if provider.Models[i].ID == modelID {
			return &provider, &provider.Models[i], nil
		}
	}

	return nil, nil, fmt.Errorf("model %s not found in provider %s", modelID, providerName)
}

// GetModelSelection returns the provider and model for a named selection
func (p *ProvidersConfig) GetModelSelection(name string) (*ProviderConfig, *ModelConfig, error) {
	selection, exists := p.Models[name]
	if !exists {
		return nil, nil, fmt.Errorf("model selection %s not found", name)
	}

	return p.FindModel(selection.Provider, selection.Model)
}

// ParseModelString parses a model string in the format "provider/model" or just "selection-name"
func (p *ProvidersConfig) ParseModelString(modelStr string) (*ProviderConfig, *ModelConfig, error) {
	// Check if it's a named selection first
	if provider, model, err := p.GetModelSelection(modelStr); err == nil {
		return provider, model, nil
	}

	// Try to parse as provider/model
	parts := strings.Split(modelStr, "/")
	if len(parts) == 2 {
		return p.FindModel(parts[0], parts[1])
	}

	return nil, nil, fmt.Errorf("invalid model string: %s (use 'provider/model' or a named selection)", modelStr)
}
