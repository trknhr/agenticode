package llm

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/tools"
)

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	if model == "" {
		model = openai.GPT4TurboPreview
	}
	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
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

func (c *OpenAIClient) Generate(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error) {
	return c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:      c.model,
		Messages:   messages,
		Tools:      c.getOpenAITools(),
		ToolChoice: "auto",
	})
}

func (c *OpenAIClient) Stream(ctx context.Context, messages []openai.ChatCompletionMessage) (*openai.ChatCompletionStream, error) {
	return c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	})
}

func (c *OpenAIClient) getOpenAITools() []openai.Tool {
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
