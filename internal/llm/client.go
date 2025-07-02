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
}

func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIClient(cfg.APIKey, cfg.Model), nil
	case "ollama":
		return nil, fmt.Errorf("Ollama client not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
