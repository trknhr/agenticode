package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

// AgentV2 is the refactored agent using event-driven architecture
type AgentV2 struct {
	llmClient llm.Client
	tools     map[string]tools.Tool
	maxSteps  int
	approver  ToolApprover
}

// NewAgentV2 creates a new event-driven agent
func NewAgentV2(llmClient llm.Client, opts ...Option) *AgentV2 {
	a := &AgentV2{
		llmClient: llmClient,
		tools:     make(map[string]tools.Tool),
		maxSteps:  10,
	}

	// Apply options using a temporary Agent for compatibility
	tempAgent := &Agent{
		llmClient: a.llmClient,
		tools:     a.tools,
		maxSteps:  a.maxSteps,
		approver:  a.approver,
	}

	for _, opt := range opts {
		opt(tempAgent)
	}

	// Copy back the values
	a.llmClient = tempAgent.llmClient
	a.tools = tempAgent.tools
	a.maxSteps = tempAgent.maxSteps
	a.approver = tempAgent.approver

	// Initialize default tools
	defaultTools := tools.GetDefaultTools()
	for _, tool := range defaultTools {
		a.tools[tool.Name()] = tool
	}

	// Set default approver if not provided
	if a.approver == nil {
		a.approver = NewInteractiveApprover()
	}

	return a
}

// ExecuteWithHistory executes a task using the event-driven architecture
func (a *AgentV2) ExecuteWithHistory(ctx context.Context, conversation []openai.ChatCompletionMessage, dryrun bool) (*ExecutionResult, []openai.ChatCompletionMessage, error) {
	result := &ExecutionResult{
		Success:        false,
		GeneratedFiles: []GeneratedFile{},
		Steps:          []ExecutionStep{},
	}

	// Create handler
	handler := NewTurnHandler(a.tools, a.approver)

	// Main execution loop
	for i := 0; i < a.maxSteps; i++ {
		log.Printf("Starting turn %d/%d", i+1, a.maxSteps)

		// Create a new turn
		turn := NewTurn(a.llmClient, a.tools, conversation)

		// Handle the turn
		if err := handler.HandleTurn(ctx, turn); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Turn failed: %v", err)
			return result, conversation, err
		}

		// Update conversation from turn (includes assistant response)
		conversation = turn.GetConversation()

		// Add tool responses to conversation
		toolResponses := handler.GetToolResponses()
		conversation = append(conversation, toolResponses...)

		// Check if there were any pending calls
		pendingCalls := turn.GetPendingCalls()
		if len(pendingCalls) == 0 {
			// No tool calls means the agent is done
			log.Println("No tool calls in this turn, task completed")
			result.Success = true
			// Extract final message from conversation
			if len(conversation) > 0 {
				lastMsg := conversation[len(conversation)-1]
				if lastMsg.Role == "assistant" {
					result.Message = lastMsg.Content
				}
			}
			break
		}

		// Track executed tools
		for _, call := range pendingCalls {
			result.Steps = append(result.Steps, ExecutionStep{
				StepNumber: len(result.Steps) + 1,
				Action:     "tool_call",
				ToolName:   call.Name,
				ToolArgs:   call.Args,
				// Result will be updated by handler
			})

			// Track generated files
			if call.Name == "write_file" {
				if path, ok := call.Args["path"].(string); ok {
					content := ""
					if c, ok := call.Args["content"].(string); ok {
						content = c
					}
					result.GeneratedFiles = append(result.GeneratedFiles, GeneratedFile{
						Path:    path,
						Content: content,
						Action:  "create",
					})
				}
			}
		}
	}

	if len(result.Steps) >= a.maxSteps {
		log.Printf("WARNING: Maximum steps (%d) reached without completion", a.maxSteps)
		result.Success = false
		result.Message = "Maximum steps reached"
	}

	return result, conversation, nil
}

// Execute runs a single task (for compatibility)
func (a *AgentV2) Execute(ctx context.Context, task string, dryrun bool) (*ExecutionResult, error) {
	conversation := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: GetCoreSystemPrompt(),
		},
		{
			Role:    "user",
			Content: task,
		},
	}

	result, _, err := a.ExecuteWithHistory(ctx, conversation, dryrun)
	return result, err
}
