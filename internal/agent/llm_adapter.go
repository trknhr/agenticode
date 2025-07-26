package agent

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

// LLMAdapter adapts an llm.Client to the tools.LLMProcessor interface
type LLMAdapter struct {
	client llm.Client
}

// NewLLMAdapter creates a new LLM adapter
func NewLLMAdapter(client llm.Client) tools.LLMProcessor {
	return &LLMAdapter{client: client}
}

// ProcessContent implements the LLMProcessor interface
func (a *LLMAdapter) ProcessContent(ctx context.Context, content, prompt string) (string, error) {
	// Prepare messages
	messages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: "You are a helpful assistant that analyzes web content. Be concise and focus on answering the user's specific question about the content.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Here is the web content:\n\n%s\n\n%s", content, prompt),
		},
	}

	// Call LLM
	response, err := a.client.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return response.Choices[0].Message.Content, nil
}