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

// Turn manages a single interaction turn with the LLM
type Turn struct {
	llmClient    llm.Client
	tools        map[string]tools.Tool
	conversation []openai.ChatCompletionMessage
	pendingCalls []ToolCallRequestEvent
	eventStream  *EventStream
}

// NewTurn creates a new Turn instance
func NewTurn(llmClient llm.Client, availableTools map[string]tools.Tool, conversation []openai.ChatCompletionMessage) *Turn {
	return &Turn{
		llmClient:    llmClient,
		tools:        availableTools,
		conversation: conversation,
		pendingCalls: []ToolCallRequestEvent{},
		eventStream:  NewEventStream(),
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

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		t.eventStream.Emit(ErrorEvent{
			Error:   err,
			Message: fmt.Sprintf("Failed to parse tool arguments: %v", err),
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
	
	// Check if tool needs confirmation
	tool, exists := t.tools[toolCall.Function.Name]
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
		t.eventStream.Emit(ToolCallConfirmationEvent{
			Request: event,
			Details: ToolCallConfirmationDetails{
				ToolName:    toolCall.Function.Name,
				Risk:        risk,
				Description: tool.Description(),
				Arguments:   args,
			},
		})
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