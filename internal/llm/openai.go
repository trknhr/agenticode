package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
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
	Content string `json:"content"`
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

func (c *OpenAIClient) GenerateCode(ctx context.Context, prompt string) (*CodeGeneration, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: `You are an expert code generation assistant. Generate clean, well-structured code based on the user's requirements. 
When generating code:
1. Create complete, working implementations
2. Follow best practices and conventions
3. Include necessary imports and dependencies
4. Add appropriate error handling
5. Use the provided tools to create or modify files`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	tools := c.getOpenAITools()

	req := openai.ChatCompletionRequest{
		Model:      c.model,
		Messages:   messages,
		Tools:      tools,
		ToolChoice: "auto",
	}

	// Execute the chat completion with tool calls
	generation := &CodeGeneration{
		Files: []FileChange{},
	}

	for i := 0; i < 10; i++ { // Max 10 iterations
		resp, err := c.client.CreateChatCompletion(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("OpenAI API error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no response from OpenAI")
		}

		choice := resp.Choices[0]
		req.Messages = append(req.Messages, choice.Message)

		// Check if there are tool calls
		if len(choice.Message.ToolCalls) == 0 {
			// No more tool calls, we're done
			generation.Summary = choice.Message.Content
			break
		}

		// Process tool calls
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Function.Name == "write_file" {
				var args map[string]string
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					continue
				}

				generation.Files = append(generation.Files, FileChange{
					Path:    args["path"],
					Content: args["content"],
					Action:  FileActionCreate,
				})

				// Add tool response
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    `{"status": "success"}`,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}

	return generation, nil
}

func (c *OpenAIClient) StreamGenerateCode(ctx context.Context, prompt string) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: `You are an expert code generation assistant. Generate clean, well-structured code based on the user's requirements.`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err == io.EOF {
				ch <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err}
				return
			}

			if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
				ch <- StreamChunk{Content: response.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}

func (c *OpenAIClient) getOpenAITools() []openai.Tool {
	return []openai.Tool{
		{
			Type: "function",
			Function: openai.FunctionDefinition{
				Name:        "write_file",
				Description: "Write content to a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The file content",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: openai.FunctionDefinition{
				Name:        "run_shell",
				Description: "Execute a shell command",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}
}
