package llm

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type Client interface {
	Generate(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error)
	Stream(ctx context.Context, messages []openai.ChatCompletionMessage) (*openai.ChatCompletionStream, error)
}

type CodeGeneration struct {
	Files   []FileChange
	Summary string
}

type FileChange struct {
	Path    string
	Content string
	Action  FileAction
}

type FileAction string

const (
	FileActionCreate FileAction = "create"
	FileActionUpdate FileAction = "update"
	FileActionDelete FileAction = "delete"
)

type StreamChunk struct {
	Content string
	Error   error
	Done    bool
}

type Config struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string

	// New fields for multi-provider support
	ProvidersConfig *ProvidersConfig
	ModelSelection  string // Can be "provider/model" or a named selection like "default", "fast", etc.
}

// NewClient creates a client using the new multi-provider configuration
func NewClient(cfg Config) (Client, error) {
	// If ProvidersConfig is provided, use the new multi-provider system
	if cfg.ProvidersConfig != nil && cfg.ModelSelection != "" {
		provider, model, err := cfg.ProvidersConfig.ParseModelString(cfg.ModelSelection)
		if err != nil {
			return nil, fmt.Errorf("failed to parse model selection '%s': %w", cfg.ModelSelection, err)
		}

		return NewProviderClient(provider, model)
	}

	// Legacy configuration support
	if cfg.Provider == "openai" && cfg.APIKey != "" && cfg.Model != "" {
		// If BaseURL is provided, create a custom provider config
		if cfg.BaseURL != "" {
			provider := &ProviderConfig{
				Type:    "openai",
				BaseURL: cfg.BaseURL,
				APIKey:  cfg.APIKey,
				Models: []ModelConfig{
					{
						ID:            cfg.Model,
						Name:          cfg.Model,
						ContextWindow: 128000,
						MaxTokens:     4096,
					},
				},
			}
			return NewProviderClient(provider, &provider.Models[0])
		}

		// Otherwise use legacy OpenAI client
		return NewOpenAIClient(cfg.APIKey, cfg.Model), nil
	}

	// Handle other legacy providers
	switch cfg.Provider {
	case "ollama":
		return nil, fmt.Errorf("ollama client not yet implemented")
	case "anthropic":
		return nil, fmt.Errorf("anthoropic client not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
