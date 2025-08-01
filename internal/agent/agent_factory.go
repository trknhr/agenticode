package agent

import (
	"context"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

// AgentFactoryAdapter adapts the agent package for use by the tools package
type AgentFactoryAdapter struct {
	systemPrompt   func(string) string
	developerPrompt func() string
}

// NewAgentFactoryAdapter creates a new adapter
func NewAgentFactoryAdapter() *AgentFactoryAdapter {
	return &AgentFactoryAdapter{
		systemPrompt:    GetSystemPrompt,
		developerPrompt: GetDeveloperPrompt,
	}
}

// CreateAgentTool creates an agent tool with the proper factory function
func (afa *AgentFactoryAdapter) CreateAgentTool(llmClient interface{}) tools.Tool {
	// Create a factory function that creates sub-agents
	agentFactory := func(llmClientInterface interface{}, agentType string) (tools.AgentInterface, error) {
		// Type assert back to the actual LLM client
		client, ok := llmClientInterface.(llm.Client)
		if !ok {
			return nil, nil
		}

		// Create appropriate approver based on agent type
		var approver ToolApprover
		if agentType == "searcher" || agentType == "analyzer" {
			// For read-only agents, create an approver that only allows safe tools
			approver = &RestrictedAutoApprover{allowedTools: getToolsForAgentType(agentType)}
		} else {
			// For general-purpose and executor agents, allow all tools
			approver = &SimpleAutoApprover{}
		}
		
		// Configure max steps based on agent type
		maxSteps := 10 // default
		switch agentType {
		case "searcher":
			maxSteps = 15 // May need more steps for thorough searching
		case "analyzer":
			maxSteps = 20 // Analysis can be complex
		case "executor":
			maxSteps = 5 // Execution should be quick
		}
		
		// Create sub-agent with appropriate configuration
		opts := []Option{
			WithMaxSteps(maxSteps),
			WithApprover(approver),
		}
		
		// For restricted agent types, only provide allowed tools
		if agentType == "searcher" || agentType == "analyzer" {
			allowedTools := getToolsForAgentType(agentType)
			filteredTools := []tools.Tool{}
			
			// Get default tools and filter them
			defaultTools := tools.GetDefaultTools()
			for _, tool := range defaultTools {
				for _, allowed := range allowedTools {
					if tool.Name() == allowed {
						filteredTools = append(filteredTools, tool)
						break
					}
				}
			}
			opts = append(opts, WithTools(filteredTools))
		}
		
		subAgent := NewAgent(client, opts...)
		
		// Create an adapter that implements tools.AgentInterface
		return &agentInterfaceAdapter{
			agent:           subAgent,
			systemPrompt:    afa.systemPrompt,
			developerPrompt: afa.developerPrompt,
		}, nil
	}

	return tools.NewAgentTool(llmClient, agentFactory)
}

// agentInterfaceAdapter adapts our Agent to the tools.AgentInterface
type agentInterfaceAdapter struct {
	agent           *Agent
	systemPrompt    func(string) string
	developerPrompt func() string
}

// ExecuteWithHistory adapts the agent's ExecuteWithHistory to use interface{} types
func (a *agentInterfaceAdapter) ExecuteWithHistory(ctx context.Context, conversation []interface{}, dryrun bool) (*tools.AgentExecutionResult, []interface{}, error) {
	// Convert interface{} conversation to OpenAI messages
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(conversation))
	
	// Get model name for system prompt
	modelName := "gpt-4" // default
	if pc, ok := a.agent.llmClient.(*llm.ProviderClient); ok {
		modelName = pc.GetCurrentModel()
	}
	
	// Add system and developer prompts
	openAIMessages = append(openAIMessages, 
		openai.ChatCompletionMessage{
			Role:    "system",
			Content: a.systemPrompt(modelName),
		},
		openai.ChatCompletionMessage{
			Role:    "developer",
			Content: a.developerPrompt(),
		},
	)
	
	// Convert the user-provided conversation
	for _, msg := range conversation {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			role, _ := msgMap["role"].(string)
			content, _ := msgMap["content"].(string)
			
			// Skip system messages as we already added our own
			if role == "system" {
				continue
			}
			
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    role,
				Content: content,
			})
		}
	}
	
	// Execute with the real agent
	result, updatedConv, err := a.agent.ExecuteWithHistory(ctx, openAIMessages, dryrun)
	if err != nil {
		return nil, nil, err
	}
	
	// Convert ExecutionResult to tools.AgentExecutionResult
	toolsResult := &tools.AgentExecutionResult{
		Success: result.Success,
		Message: result.Message,
		GeneratedFiles: make([]tools.GeneratedFile, len(result.GeneratedFiles)),
		Steps: make([]tools.ExecutionStep, len(result.Steps)),
	}
	
	// Convert generated files
	for i, file := range result.GeneratedFiles {
		toolsResult.GeneratedFiles[i] = tools.GeneratedFile{
			Path:    file.Path,
			Content: file.Content,
			Action:  file.Action,
		}
	}
	
	// Convert steps
	for i, step := range result.Steps {
		toolsResult.Steps[i] = tools.ExecutionStep{
			StepNumber: step.StepNumber,
			Action:     step.Action,
			ToolName:   step.ToolName,
			ToolArgs:   step.ToolArgs,
			Result:     step.Result,
			Error:      step.Error,
		}
	}
	
	// Convert updated conversation back to interface{}
	updatedInterface := make([]interface{}, len(updatedConv))
	for i, msg := range updatedConv {
		updatedInterface[i] = msg
	}
	
	return toolsResult, updatedInterface, nil
}

// SimpleAutoApprover automatically approves all tool calls for sub-agents
type SimpleAutoApprover struct{}

func (s *SimpleAutoApprover) RequestApproval(ctx context.Context, request ApprovalRequest) (ApprovalResponse, error) {
	response := ApprovalResponse{
		RequestID:   request.RequestID,
		Approved:    true,
		ApprovedIDs: make([]string, len(request.ToolCalls)),
		RejectedIDs: []string{},
		Reason:      "Auto-approved for sub-agent",
	}
	
	for i, call := range request.ToolCalls {
		response.ApprovedIDs[i] = call.ID
	}
	
	return response, nil
}

func (s *SimpleAutoApprover) NotifyExecution(toolCallID string, result interface{}, err error) {
	// No-op for auto approver
}

// RestrictedAutoApprover only approves specific tools
type RestrictedAutoApprover struct {
	allowedTools []string
}

func (r *RestrictedAutoApprover) RequestApproval(ctx context.Context, request ApprovalRequest) (ApprovalResponse, error) {
	response := ApprovalResponse{
		RequestID:   request.RequestID,
		Approved:    false,
		ApprovedIDs: []string{},
		RejectedIDs: []string{},
	}
	
	// Check each tool call
	for _, call := range request.ToolCalls {
		toolName := call.ToolCall.Function.Name
		allowed := false
		
		// Check if tool is in allowed list
		for _, allowedTool := range r.allowedTools {
			if toolName == allowedTool {
				allowed = true
				break
			}
		}
		
		if allowed {
			response.ApprovedIDs = append(response.ApprovedIDs, call.ID)
		} else {
			response.RejectedIDs = append(response.RejectedIDs, call.ID)
		}
	}
	
	// Set approved if at least one tool was approved
	response.Approved = len(response.ApprovedIDs) > 0
	if len(response.RejectedIDs) > 0 {
		response.Reason = "Some tools are not allowed for this agent type"
	}
	
	return response, nil
}

func (r *RestrictedAutoApprover) NotifyExecution(toolCallID string, result interface{}, err error) {
	// No-op for auto approver
}

// getToolsForAgentType returns allowed tools for each agent type
func getToolsForAgentType(agentType string) []string {
	switch agentType {
	case "searcher":
		return []string{"read_file", "read", "list_files", "grep", "glob", "read_many_files"}
	case "analyzer":
		return []string{"read_file", "read", "list_files", "grep", "glob", "read_many_files", "todo_read"}
	case "executor":
		return []string{"run_shell", "read_file", "list_files"}
	default:
		// general-purpose gets all tools
		return []string{}
	}
}