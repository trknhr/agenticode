package llm

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/tools"
)

// ProviderClient is a provider-agnostic client that works with any OpenAI-compatible API
type ProviderClient struct {
	client         *openai.Client
	providerConfig *ProviderConfig
	modelConfig    *ModelConfig
	currentModel   string
}

// NewProviderClient creates a new provider-agnostic client
func NewProviderClient(provider *ProviderConfig, model *ModelConfig) (*ProviderClient, error) {
	if provider == nil || model == nil {
		return nil, fmt.Errorf("provider and model configs are required")
	}

	// Expand environment variables in provider config
	ExpandProviderConfig(provider)

	// Validate provider has the requested model
	modelFound := false
	for _, m := range provider.Models {
		if m.ID == model.ID {
			modelFound = true
			break
		}
	}
	if !modelFound {
		return nil, fmt.Errorf("model %s not found in provider %s", model.ID, provider.Type)
	}

	fmt.Println("provider.APIKey", provider.APIKey)
	// Create OpenAI-compatible client with custom base URL
	config := openai.DefaultConfig(provider.APIKey)
	if provider.BaseURL != "" {
		config.BaseURL = provider.BaseURL
	}

	return &ProviderClient{
		client:         openai.NewClientWithConfig(config),
		providerConfig: provider,
		modelConfig:    model,
		currentModel:   model.ID,
	}, nil
}

// // Legacy constructor for backwards compatibility
func NewOpenAIClient(apiKey, model string) *ProviderClient {
	// Create a legacy OpenAI provider config
	provider := &ProviderConfig{
		Type:    "openai",
		BaseURL: "https://api.openai.com/v1",
		APIKey:  apiKey,
		Models: []ModelConfig{
			{
				ID:            model,
				Name:          model,
				ContextWindow: 128000,
				MaxTokens:     4096,
			},
		},
	}

	modelConfig := &provider.Models[0]

	client, _ := NewProviderClient(provider, modelConfig)
	return client
}

type Message struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`    // optional: for tool messages
	Content string `json:"content,omitempty"` // system/user/assistant/tool messages
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int        `json:"index"`
	Message      Message    `json:"message"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Generate sends a chat completion request to the provider
func (c *ProviderClient) Generate(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:      c.currentModel,
		Messages:   messages,
		Tools:      c.getOpenAITools(),
		ToolChoice: "auto",
	}

	// Apply model-specific settings
	if c.modelConfig.MaxTokens > 0 {
		req.MaxTokens = c.modelConfig.MaxTokens
	}

	return c.client.CreateChatCompletion(ctx, req)
}

// Stream sends a streaming chat completion request to the provider
func (c *ProviderClient) Stream(ctx context.Context, messages []openai.ChatCompletionMessage) (*openai.ChatCompletionStream, error) {
	return c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    c.currentModel,
		Messages: messages,
		Stream:   true,
	})
}

// GetCurrentModel returns the currently active model ID
func (c *ProviderClient) GetCurrentModel() string {
	return c.currentModel
}

// GetProviderName returns the provider name
func (c *ProviderClient) GetProviderName() string {
	return c.providerConfig.Type
}

// SwitchModel switches to a different model within the same provider
func (c *ProviderClient) SwitchModel(modelID string) error {
	for _, model := range c.providerConfig.Models {
		if model.ID == modelID {
			c.currentModel = modelID
			c.modelConfig = &model
			return nil
		}
	}
	return fmt.Errorf("model %s not found in provider", modelID)
}

func (c *ProviderClient) getOpenAITools() []openai.Tool {
	// Get all available tools from the tools package
	defaultTools := tools.GetDefaultTools()

	// Convert to OpenAI tool definitions
	openAITools := make([]openai.Tool, 0, len(defaultTools))
	for _, tool := range defaultTools {
		// Skip tools that are not yet implemented
		if tool.Name() == "apply_patch" {
			continue
		}

		openAITools = append(openAITools, openai.Tool{
			Type: "function",
			Function: openai.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.GetParameters(),
			},
		})
	}

	return openAITools
}
