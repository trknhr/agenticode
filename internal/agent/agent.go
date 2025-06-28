package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

type Agent struct {
	llmClient llm.Client
	tools     map[string]tools.Tool
	maxSteps  int
}

type AgentConfig struct {
	LLMClient llm.Client
	Tools     []tools.Tool
	MaxSteps  int
}

func New(llmClient llm.Client, opts ...Option) *Agent {
	a := &Agent{
		llmClient: llmClient,
		tools:     make(map[string]tools.Tool),
		maxSteps:  10,
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	// Initialize default tools
	a.tools["write_file"] = tools.NewWriteFileTool()
	a.tools["run_shell"] = tools.NewRunShellTool()

	return a
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

func (a *Agent) ExecuteTask(ctx context.Context, task string) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Steps: make([]ExecutionStep, 0),
	}

	conversation := []Message{
		{
			Role: "system",
			Content: `You are a code generation agent. Your task is to generate code based on the user's requirements.
Use the available tools to create files and execute commands as needed.
Think step by step and explain your actions.`,
		},
		{
			Role:    "user",
			Content: task,
		},
	}

	for step := 0; step < a.maxSteps; step++ {
		// Get next action from LLM
		response, err := a.callLLM(ctx, conversation)
		if err != nil {
			return result, fmt.Errorf("LLM call failed: %w", err)
		}

		// Add assistant response to conversation
		conversation = append(conversation, Message{
			Role:    "assistant",
			Content: response.Content,
		})

		// Check if there are tool calls
		if len(response.ToolCalls) == 0 {
			// No more tool calls, agent is done
			result.Success = true
			result.Message = response.Content
			break
		}

		// Execute tool calls
		for _, toolCall := range response.ToolCalls {
			stepResult := a.executeToolCall(toolCall)
			result.Steps = append(result.Steps, stepResult)

			// Add tool result to conversation
			toolResult := map[string]interface{}{
				"tool_call_id": toolCall.ID,
				"output":       stepResult.Result,
			}

			if stepResult.Error != nil {
				toolResult["error"] = stepResult.Error.Error()
			}

			conversation = append(conversation, Message{
				Role:    "tool",
				Content: jsonString(toolResult),
			})

			// Track generated files
			if toolCall.Function.Name == "write_file" && stepResult.Error == nil {
				if args, ok := stepResult.ToolArgs["path"].(string); ok {
					content := ""
					if c, ok := stepResult.ToolArgs["content"].(string); ok {
						content = c
					}
					result.GeneratedFiles = append(result.GeneratedFiles, GeneratedFile{
						Path:    args,
						Content: content,
						Action:  "create",
					})
				}
			}
		}
	}

	if len(result.Steps) >= a.maxSteps {
		result.Success = false
		result.Message = "Maximum steps reached"
	}

	return result, nil
}

type LLMResponse struct {
	Content   string
	ToolCalls []ToolCall
}

type Message struct {
	Role    string `json:"role"`
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

func (a *Agent) callLLM(ctx context.Context, messages []Message) (*LLMResponse, error) {
	// Convert messages to the format expected by LLM client
	prompt := ""
	for i, msg := range messages {
		if i == 0 && msg.Role == "system" {
			continue // Skip system message as it's handled by the LLM client
		}
		prompt += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}

	// Call LLM to generate code
	result, err := a.llmClient.GenerateCode(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Convert result to LLMResponse
	response := &LLMResponse{
		Content:   result.Summary,
		ToolCalls: []ToolCall{},
	}

	// Convert file changes to tool calls
	for i, file := range result.Files {
		response.ToolCalls = append(response.ToolCalls, ToolCall{
			ID:   fmt.Sprintf("call_%d", i+1),
			Type: "function",
			Function: FunctionCall{
				Name:      "write_file",
				Arguments: fmt.Sprintf(`{"path": "%s", "content": %s}`, file.Path, jsonString(file.Content)),
			},
		})
	}

	return response, nil
}

func (a *Agent) executeToolCall(toolCall ToolCall) ExecutionStep {
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

	// Execute tool
	result, err := tool.Execute(args)
	step.Result = result
	step.Error = err

	if err != nil {
		log.Printf("Tool execution failed: %s - %v", toolCall.Function.Name, err)
	} else {
		log.Printf("Tool executed successfully: %s", toolCall.Function.Name)
	}

	return step
}

func jsonString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// GenerateCode generates code based on the prompt and returns files without writing them
func (a *Agent) GenerateCode(ctx context.Context, prompt string, dryRun bool) (map[string]string, error) {
	result, err := a.ExecuteTask(ctx, prompt)
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
