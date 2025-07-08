package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
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
	defaultTools := tools.GetDefaultTools()
	for _, tool := range defaultTools {
		a.tools[tool.Name()] = tool
	}

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

// ExecuteWithHistory executes a task with a given conversation history
// It returns the result and the updated conversation history
func (a *Agent) ExecuteWithHistory(ctx context.Context, conversation []openai.ChatCompletionMessage, dryrun bool) (*ExecutionResult, []openai.ChatCompletionMessage, error) {
	if dryrun {
		log.Println("Running in dry-run mode")
	}

	result := &ExecutionResult{
		Steps: make([]ExecutionStep, 0),
	}

	for step := 0; step < a.maxSteps; step++ {
		log.Printf("Processing step %d/%d", step+1, a.maxSteps)

		// Get next action from LLM
		log.Println("Calling LLM for next action...")
		response, err := a.callLLM(ctx, conversation)
		if err != nil {
			return result, conversation, fmt.Errorf("LLM call failed: %w", err)
		}

		if response.Role == "model" {
			// Add assistant response to conversation
			conversation = append(conversation, openai.ChatCompletionMessage{
				Role:      "assistant",
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})
		}

		if response.Content != "" {
			log.Printf("LLM response: %s", response.Content)
		}

		// Check if there are tool calls
		if len(response.ToolCalls) == 0 {
			// No more tool calls, agent is done
			log.Println("No more tool calls needed, task completed")
			result.Success = true
			result.Message = response.Content
			break
		}

		log.Printf("LLM requested %d tool call(s)", len(response.ToolCalls))

		// Execute tool calls
		for i, toolCall := range response.ToolCalls {
			log.Printf("Executing tool call %d/%d: %s", i+1, len(response.ToolCalls), toolCall.Function.Name)
			stepResult := a.executeToolCall(toolCall, dryrun)
			result.Steps = append(result.Steps, stepResult)

			// Add tool result to conversation
			toolResult := map[string]interface{}{
				"tool_call_id": toolCall.ID,
				"output":       stepResult.Result,
			}

			if stepResult.Error != nil {
				toolResult["error"] = stepResult.Error.Error()
			}

			conversation = append(conversation, openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       stepResult.ToolName,
				Content:    jsonString(toolResult),
				ToolCallID: toolCall.ID,
			})

			log.Printf("Executing tool call: %v", conversation)
			// Track generated files
			if toolCall.Function.Name == "write_file" && stepResult.Error == nil {
				if args, ok := stepResult.ToolArgs["path"].(string); ok {
					content := ""
					if c, ok := stepResult.ToolArgs["content"].(string); ok {
						content = c
					}
					log.Printf("File to be written: %s", args)
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
		log.Printf("WARNING: Maximum steps (%d) reached without completion", a.maxSteps)
		result.Success = false
		result.Message = "Maximum steps reached"
	}

	if result.Success {
		log.Printf("Task completed successfully with %d files generated", len(result.GeneratedFiles))
	} else {
		log.Printf("Task failed: %s", result.Message)
	}

	return result, conversation, nil
}

type LLMResponse struct {
	Role      string
	Content   string
	ToolCalls []openai.ToolCall
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
	}, nil
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

	// Execute tool
	if !tool.ReadOnly() && dryrun {
		path, _ := args["path"].(string)

		step.Result = map[string]string{
			"status":  "success",
			"path":    path,
			"message": fmt.Sprintf("File '%s' written successfully.", args["path"]),
		}
		step.Error = nil
		return step
	}

	result, err := tool.Execute(args)
	step.Result = result
	step.Error = err

	if err != nil {
		log.Printf("Tool execution failed: %s - %v", toolCall.Function.Name, err)
	} else {
		if toolCall.Function.Name == "write_file" {
			if path, ok := args["path"].(string); ok {
				log.Printf("Tool executed successfully: %s - file: %s", toolCall.Function.Name, path)
			} else {
				log.Printf("Tool executed successfully: %s", toolCall.Function.Name)
			}
		} else if toolCall.Function.Name == "run_shell" {
			if cmd, ok := args["command"].(string); ok {
				log.Printf("Tool executed successfully: %s - command: %s", toolCall.Function.Name, cmd)
			} else {
				log.Printf("Tool executed successfully: %s", toolCall.Function.Name)
			}
		} else {
			log.Printf("Tool executed successfully: %s", toolCall.Function.Name)
		}
	}

	return step
}

func jsonString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
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
