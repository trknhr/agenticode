package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

type Agent struct {
	llmClient llm.Client
	tools     map[string]tools.Tool
	maxSteps  int
	approver  ToolApprover
}

// NewAgentV2 creates a new event-driven agent
func NewAgent(llmClient llm.Client, opts ...Option) *Agent {
	a := &Agent{
		llmClient: llmClient,
		tools:     make(map[string]tools.Tool),
		maxSteps:  10,
	}

	for _, opt := range opts {
		opt(a)
	}

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

type AgentConfig struct {
	LLMClient llm.Client
	Tools     []tools.Tool
	MaxSteps  int
}

// Option configures an Agent
type Option func(*Agent)

// WithMaxSteps sets the maximum number of steps
func WithMaxSteps(steps int) Option {
	return func(a *Agent) {
		a.maxSteps = steps
	}
}

// WithTools sets the tools available to the agent
func WithTools(tools []tools.Tool) Option {
	return func(a *Agent) {
		for _, tool := range tools {
			a.tools[tool.Name()] = tool
		}
	}
}

// WithApprover sets the tool approver
func WithApprover(approver ToolApprover) Option {
	return func(a *Agent) {
		a.approver = approver
	}
}

type ExecutionResult struct {
	Success        bool
	Message        string
	GeneratedFiles []GeneratedFile
	Steps          []ExecutionStep
}

type GeneratedFile struct {
	Path    string
	Content string
	Action  string
}

type ExecutionStep struct {
	StepNumber int
	Action     string
	ToolName   string
	ToolArgs   map[string]interface{}
	Result     interface{}
	Error      error
}

func (a *Agent) ExecuteWithHistory(ctx context.Context, conversation []openai.ChatCompletionMessage, dryrun bool) (*ExecutionResult, []openai.ChatCompletionMessage, error) {
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
func (a *Agent) Execute(ctx context.Context, task string, dryrun bool) (*ExecutionResult, error) {
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

func (a *Agent) ExecuteTask(ctx context.Context, task string, dryrun bool) (*ExecutionResult, error) {
	log.Printf("Starting task execution: %s", task)

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

type LLMResponse struct {
	Role      string
	Content   string
	ToolCalls []openai.ToolCall
	Reasoning string
}

type Message struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
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

func (a *Agent) callLLM(ctx context.Context, messages []openai.ChatCompletionMessage) (*LLMResponse, error) {
	// Convert messages to the format expected by LLM client
	prompt := ""
	for i, msg := range messages {
		if i == 0 && msg.Role == "system" {
			continue // Skip system message as it's handled by the LLM client
		}
		prompt += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}

	// Call LLM to generate code
	resp, err := a.llmClient.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}
	msg := resp.Choices[0].Message

	// Convert result to LLMResponse
	return &LLMResponse{
		Role:      msg.Role,
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
		Reasoning: extractReasoning(msg.Content),
	}, nil
}

func extractReasoning(content string) string {
	re := regexp.MustCompile(`(?s)Reasoning:\s*(.+?)(?:\n[A-Z][a-z]+:|$)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func mapToolCalls(calls []openai.ToolCall) []ToolCall {
	var result []ToolCall
	for _, c := range calls {
		result = append(result, ToolCall{
			ID:   c.ID,
			Type: string(c.Type),
			Function: FunctionCall{
				Name:      c.Function.Name,
				Arguments: c.Function.Arguments,
			},
		})
	}
	return result
}

func (a *Agent) executeToolCall(toolCall openai.ToolCall, dryrun bool) ExecutionStep {
	step := ExecutionStep{
		Action:   "tool_call",
		ToolName: toolCall.Function.Name,
	}

	tool, exists := a.tools[toolCall.Function.Name]
	if !exists {
		step.Error = fmt.Errorf("tool not found: %s", toolCall.Function.Name)
		return step
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		step.Error = fmt.Errorf("failed to parse arguments: %w", err)
		return step
	}
	step.ToolArgs = args

	result, err := tool.Execute(args)
	if err != nil {
		step.Error = err
		step.Result = nil
	} else {
		step.Result = result
		step.Error = result.Error
	}

	if err != nil {
		log.Printf("Tool execution failed: %s - %v", toolCall.Function.Name, err)
	}

	return step
}

func jsonString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// filterConversationForLLM removes tool messages that don't have a preceding message with tool_calls
func filterConversationForLLM(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	filtered := make([]openai.ChatCompletionMessage, 0, len(messages))

	for i, msg := range messages {
		if msg.Role == "tool" {
			// Check if previous message has tool_calls
			if i > 0 && len(messages[i-1].ToolCalls) > 0 {
				filtered = append(filtered, msg)
			}
			// Skip orphaned tool messages
		} else {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// GenerateCode generates code based on the prompt and returns files without writing them
func (a *Agent) GenerateCode(ctx context.Context, prompt string, dryRun bool) (map[string]string, error) {
	result, err := a.ExecuteTask(ctx, prompt, dryRun)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("code generation failed: %s", result.Message)
	}

	// Collect generated files
	files := make(map[string]string)
	for _, file := range result.GeneratedFiles {
		files[file.Path] = file.Content
	}

	return files, nil
}

func (a *Agent) detectRepetitiveActions(steps []ExecutionStep) bool {
	if len(steps) < 3 {
		return false
	}

	// 最後の3つのステップを確認
	recent := steps[len(steps)-3:]

	// 同じコマンドが繰り返されているかチェック
	commands := make(map[string]int)
	for _, step := range recent {
		if step.ToolName == "run_shell" {
			if cmd, ok := step.ToolArgs["command"].(string); ok {
				commands[cmd]++
				if commands[cmd] >= 2 {
					return true
				}
			}
		}
	}

	return false
}
