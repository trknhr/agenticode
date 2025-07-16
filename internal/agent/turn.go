package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

// Turn manages a single interaction turn with the LLM
type Turn struct {
	llmClient    llm.Client
	tools        map[string]tools.Tool
	conversation []openai.ChatCompletionMessage
	pendingCalls []ToolCallRequestEvent
	eventStream  *EventStream
	debugger     Debugger
}

// NewTurn creates a new Turn instance
func NewTurn(llmClient llm.Client, availableTools map[string]tools.Tool, conversation []openai.ChatCompletionMessage, debugger Debugger) *Turn {
	return &Turn{
		llmClient:    llmClient,
		tools:        availableTools,
		conversation: conversation,
		pendingCalls: []ToolCallRequestEvent{},
		eventStream:  NewEventStream(),
		debugger:     debugger,
	}
}

// Run executes the turn and yields events
func (t *Turn) Run(ctx context.Context) <-chan Event {
	go t.run(ctx)
	return t.eventStream.Events()
}

// run is the internal implementation
func (t *Turn) run(ctx context.Context) {
	defer t.eventStream.Close()

	// Call LLM
	response, err := t.callLLM(ctx)
	if err != nil {
		t.eventStream.Emit(ErrorEvent{
			Error:   err,
			Message: fmt.Sprintf("LLM call failed: %v", err),
		})
		return
	}

	// Add assistant response to conversation
	t.conversation = append(t.conversation, openai.ChatCompletionMessage{
		Role:      "assistant",
		Content:   response.Content,
		ToolCalls: response.ToolCalls,
	})

	// Emit content if present
	if response.Content != "" {
		t.eventStream.Emit(ContentEvent{
			Content: response.Content,
		})
	}

	// Handle tool calls
	for _, toolCall := range response.ToolCalls {
		t.handleToolCall(toolCall)
	}
}

// callLLM makes the actual LLM call
func (t *Turn) callLLM(ctx context.Context) (*LLMResponse, error) {
	// Filter conversation for LLM
	filteredConversation := filterConversationForLLM(t.conversation)
	
	// Check with debugger before making LLM call
	if t.debugger != nil && !t.debugger.ShouldContinue(filteredConversation) {
		return nil, fmt.Errorf("LLM call cancelled by debugger")
	}
	
	log.Printf("Calling LLM with %d messages in conversation", len(filteredConversation))
	resp, err := t.llmClient.Generate(ctx, filteredConversation)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices from LLM")
	}

	choice := resp.Choices[0]
	return &LLMResponse{
		Role:      choice.Message.Role,
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
	}, nil
}

// handleToolCall processes a single tool call request
func (t *Turn) handleToolCall(toolCall openai.ToolCall) {
	// Generate call ID if not present
	callID := toolCall.ID
	if callID == "" {
		callID = fmt.Sprintf("%s-%d", toolCall.Function.Name, len(t.pendingCalls))
	}
	
	// Log tool call for debugging
	log.Printf("Processing tool call: ID=%s, Name=%s", callID, toolCall.Function.Name)

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		t.eventStream.Emit(ErrorEvent{
			Error:   err,
			Message: fmt.Sprintf("Failed to parse tool arguments for %s: %v", toolCall.Function.Name, err),
		})
		return
	}

	// Create tool call request event
	event := ToolCallRequestEvent{
		CallID:            callID,
		Name:              toolCall.Function.Name,
		Args:              args,
		IsClientInitiated: false,
	}

	t.pendingCalls = append(t.pendingCalls, event)
	
	// Check if tool exists
	_, exists := t.tools[toolCall.Function.Name]
	if !exists {
		t.eventStream.Emit(ErrorEvent{
			Error:   fmt.Errorf("tool not found: %s", toolCall.Function.Name),
			Message: fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name),
		})
		return
	}

	// Emit tool call request
	t.eventStream.Emit(event)

	// Emit confirmation request if needed (based on risk level)
	risk := AssessToolCallRisk(toolCall.Function.Name)
	if risk != RiskLow {
		// Create confirmation details based on tool type
		details := t.createConfirmationDetails(toolCall.Function.Name, args, risk)
		if details != nil {
			t.eventStream.Emit(ToolCallConfirmationEvent{
				Request: event,
				Details: details,
			})
		}
	}
}

// GetPendingCalls returns the list of pending tool calls
func (t *Turn) GetPendingCalls() []ToolCallRequestEvent {
	return t.pendingCalls
}

// AddToolResponse adds a tool response to the conversation
func (t *Turn) AddToolResponse(callID string, toolName string, result *tools.ToolResult) {
	content := result.LLMContent
	if result.Error != nil {
		content = fmt.Sprintf("Error: %v", result.Error)
	}

	t.conversation = append(t.conversation, openai.ChatCompletionMessage{
		Role:       "tool",
		Name:       toolName,
		Content:    content,
		ToolCallID: callID,
	})
}

// GetConversation returns the current conversation state
func (t *Turn) GetConversation() []openai.ChatCompletionMessage {
	return t.conversation
}

// createConfirmationDetails creates appropriate confirmation details based on tool type
func (t *Turn) createConfirmationDetails(toolName string, args map[string]interface{}, risk RiskLevel) ToolCallConfirmationDetails {
	switch toolName {
	case "write_file", "edit":
		return t.createFileConfirmationDetails(toolName, args, risk)
	case "run_shell":
		return t.createExecConfirmationDetails(toolName, args, risk)
	default:
		// For other tools, create basic info confirmation
		return &ToolInfoConfirmationDetails{
			ToolName:    toolName,
			Description: fmt.Sprintf("%s: %v", toolName, args),
			Parameters:  args,
			Risk:        risk,
		}
	}
}

// createFileConfirmationDetails creates confirmation details for file operations
func (t *Turn) createFileConfirmationDetails(toolName string, args map[string]interface{}, risk RiskLevel) *ToolFileConfirmationDetails {
	details := &ToolFileConfirmationDetails{
		ToolName: toolName,
		Risk:     risk,
	}

	// Extract file path
	if toolName == "write_file" {
		if path, ok := args["path"].(string); ok {
			details.FilePath = path
		}
		if content, ok := args["content"].(string); ok {
			details.NewContent = content
		}
		
		// Check if file exists
		if _, err := os.Stat(details.FilePath); err == nil {
			// File exists, read current content
			currentContent, err := os.ReadFile(details.FilePath)
			if err == nil {
				details.OriginalContent = string(currentContent)
				details.IsNewFile = false
				
				// Generate diff
				diffGen := NewDiffGenerator()
				details.FileDiff = diffGen.GenerateColoredDiff(details.OriginalContent, details.NewContent, details.FilePath)
			}
		} else {
			// New file
			details.IsNewFile = true
		}
	} else if toolName == "edit" {
		if path, ok := args["file_path"].(string); ok {
			details.FilePath = path
		}
		
		// Read current file content
		currentContent, err := os.ReadFile(details.FilePath)
		if err != nil {
			return nil // Can't edit non-existent file
		}
		
		details.OriginalContent = string(currentContent)
		details.IsNewFile = false
		
		// Calculate new content
		oldString, _ := args["old_string"].(string)
		newString, _ := args["new_string"].(string)
		replaceAll, _ := args["replace_all"].(bool)
		
		if replaceAll {
			details.NewContent = strings.ReplaceAll(details.OriginalContent, oldString, newString)
		} else {
			details.NewContent = strings.Replace(details.OriginalContent, oldString, newString, 1)
		}
		
		// Generate diff
		diffGen := NewDiffGenerator()
		details.FileDiff = diffGen.GenerateColoredDiff(details.OriginalContent, details.NewContent, details.FilePath)
	}
	
	return details
}

// createExecConfirmationDetails creates confirmation details for command execution
func (t *Turn) createExecConfirmationDetails(toolName string, args map[string]interface{}, risk RiskLevel) *ToolExecConfirmationDetails {
	details := &ToolExecConfirmationDetails{
		ToolName: toolName,
		Risk:     risk,
	}
	
	if cmd, ok := args["command"].(string); ok {
		details.Command = cmd
	}
	
	if wd, ok := args["working_directory"].(string); ok {
		details.WorkingDir = wd
	} else {
		// Get current working directory as default
		if cwd, err := os.Getwd(); err == nil {
			details.WorkingDir = cwd
		}
	}
	
	return details
}