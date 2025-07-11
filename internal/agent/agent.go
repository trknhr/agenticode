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
	scheduler *ToolCallScheduler
	approver  ToolApprover
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
		scheduler: NewToolCallScheduler(),
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

	// Set default approver if not provided
	if a.approver == nil {
		a.approver = NewInteractiveApprover()
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

		// Filter conversation to remove orphaned tool messages
		// (tool messages without preceding tool_calls)
		filteredConversation := filterConversationForLLM(conversation)

		response, err := a.callLLM(ctx, filteredConversation)
		if err != nil {
			return result, conversation, fmt.Errorf("LLM call failed: %w", err)
		}

		// Add assistant response to conversation
		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		if response.Content != "" {
			log.Printf("LLM response: %s", response.Content)
		}

		if a.detectRepetitiveActions(result.Steps) {
			log.Println("Detected repetitive actions, stopping execution")
			result.Success = false
			result.Message = "Stopped due to repetitive actions. Please check if the task is already complete."
			break
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

		// Schedule tool calls for approval
		pendingCalls := a.scheduler.ScheduleToolCalls(ctx, response.ToolCalls)
		
		// Create approval request
		approvalReq := ApprovalRequest{
			RequestID: fmt.Sprintf("step-%d", len(result.Steps)+1),
			ToolCalls: pendingCalls,
			Risks:     make(map[string]RiskLevel),
		}
		
		// Assess risks for each tool call
		for _, call := range pendingCalls {
			approvalReq.Risks[call.ID] = AssessToolCallRisk(call.ToolCall.Function.Name)
		}
		
		// Request approval
		approval, err := a.approver.RequestApproval(ctx, approvalReq)
		if err != nil {
			log.Printf("Error requesting approval: %v", err)
			result.Success = false
			result.Message = fmt.Sprintf("Approval error: %v", err)
			break
		}
		
		// Handle rejection
		if !approval.Approved && len(approval.ApprovedIDs) == 0 {
			log.Println("All tool calls rejected by user")
			result.Success = false
			result.Message = "Tool calls rejected by user"
			break
		}
		
		// Update scheduler with approval decisions
		a.scheduler.ApproveCalls(approval.ApprovedIDs)
		a.scheduler.RejectCalls(approval.RejectedIDs)
		
		// Execute approved tool calls
		for i, toolCall := range response.ToolCalls {
			// Check if this call was approved
			approved := false
			for _, approvedID := range approval.ApprovedIDs {
				if toolCall.ID == approvedID {
					approved = true
					break
				}
			}
			
			if !approved {
				log.Printf("Skipping rejected tool call: %s", toolCall.Function.Name)
				// Add rejection message to conversation
				conversation = append(conversation, openai.ChatCompletionMessage{
					Role:       "tool",
					Name:       toolCall.Function.Name,
					Content:    "Tool call rejected by user",
					ToolCallID: toolCall.ID,
				})
				continue
			}
			
			log.Printf("Executing approved tool call %d/%d: %s", i+1, len(response.ToolCalls), toolCall.Function.Name)
			stepResult := a.executeToolCall(toolCall, dryrun)
			result.Steps = append(result.Steps, stepResult)
			
			// Mark as executed in scheduler
			a.scheduler.MarkExecuted(toolCall.ID, stepResult.Result, stepResult.Error)
			
			// Notify approver of execution result
			a.approver.NotifyExecution(toolCall.ID, stepResult.Result, stepResult.Error)

			// Add tool result to conversation
			var toolContent string
			if stepResult.Error != nil {
				toolContent = fmt.Sprintf("Error: %v", stepResult.Error)
			} else if stepResult.Result != nil {
				// Use the LLMContent for the conversation history
				if toolResult, ok := stepResult.Result.(*tools.ToolResult); ok {
					toolContent = toolResult.LLMContent
				} else {
					// Fallback for any tools that might not return ToolResult yet
					toolContent = jsonString(stepResult.Result)
				}
			}

			conversation = append(conversation, openai.ChatCompletionMessage{
				Role:       "tool",
				Name:       stepResult.ToolName,
				Content:    toolContent,
				ToolCallID: toolCall.ID,
			})

			log.Printf("Executing tool call: %v", conversation[len(conversation)-1])
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
