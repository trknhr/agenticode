package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/tools"
)

// TurnHandler coordinates the handling of events from a Turn
type TurnHandler struct {
	tools            map[string]tools.Tool
	approver         ToolApprover
	scheduler        *ToolCallScheduler
	pendingApprovals map[string]ToolCallRequestEvent
	turn             *Turn
	toolResponses    []openai.ChatCompletionMessage
}

// NewTurnHandler creates a new turn handler
func NewTurnHandler(tools map[string]tools.Tool, approver ToolApprover) *TurnHandler {
	return &TurnHandler{
		tools:            tools,
		approver:         approver,
		scheduler:        NewToolCallScheduler(),
		pendingApprovals: make(map[string]ToolCallRequestEvent),
		toolResponses:    []openai.ChatCompletionMessage{},
	}
}

// HandleTurn processes all events from a turn
func (h *TurnHandler) HandleTurn(ctx context.Context, turn *Turn) error {
	h.turn = turn
	h.toolResponses = []openai.ChatCompletionMessage{} // Reset for new turn
	events := turn.Run(ctx)

	for event := range events {
		if err := h.handleEvent(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// handleEvent processes a single event
func (h *TurnHandler) handleEvent(ctx context.Context, event Event) error {
	switch e := event.(type) {
	case ContentEvent:
		return h.handleContent(e)
	case ToolCallRequestEvent:
		return h.handleToolCallRequest(ctx, e)
	case ToolCallConfirmationEvent:
		return h.handleToolCallConfirmation(ctx, e)
	case ErrorEvent:
		return h.handleError(e)
	case UserCancelledEvent:
		return h.handleUserCancelled()
	default:
		log.Printf("Unhandled event type: %T", event)
		return nil
	}
}

// handleContent displays content from the LLM
func (h *TurnHandler) handleContent(event ContentEvent) error {
	fmt.Println(event.Content)
	return nil
}

// handleToolCallRequest processes a tool call request
func (h *TurnHandler) handleToolCallRequest(ctx context.Context, event ToolCallRequestEvent) error {
	// For low-risk tools that don't need confirmation, execute immediately
	risk := AssessToolCallRisk(event.Name)
	if risk == RiskLow {
		return h.executeToolCall(ctx, event)
	}

	// For other tools, store for pending approval
	h.pendingApprovals[event.CallID] = event
	return nil
}

// handleToolCallConfirmation handles approval requests
func (h *TurnHandler) handleToolCallConfirmation(ctx context.Context, event ToolCallConfirmationEvent) error {
	// Schedule the tool call
	pendingCalls := h.scheduler.ScheduleToolCalls(ctx, []openai.ToolCall{{
		ID: event.Request.CallID,
		Function: openai.FunctionCall{
			Name:      event.Request.Name,
			Arguments: jsonString(event.Request.Args),
		},
	}})

	// Create approval request with confirmation details
	approvalReq := ApprovalRequest{
		RequestID:           event.Request.CallID,
		ToolCalls:           pendingCalls,
		Risks:               map[string]RiskLevel{event.Request.CallID: event.Details.GetRisk()},
		ConfirmationDetails: event.Details,
	}

	// Request approval
	approval, err := h.approver.RequestApproval(ctx, approvalReq)
	if err != nil {
		return fmt.Errorf("approval error: %w", err)
	}

	// Process approval response
	if len(approval.ApprovedIDs) > 0 {
		h.scheduler.ApproveCalls(approval.ApprovedIDs)
		// Execute approved tool
		if req, exists := h.pendingApprovals[event.Request.CallID]; exists {
			if err := h.executeToolCall(ctx, req); err != nil {
				return err
			}
		}
	} else {
		// Tool was rejected
		h.scheduler.RejectCalls([]string{event.Request.CallID})
		// Add rejection to tool responses
		h.toolResponses = append(h.toolResponses, openai.ChatCompletionMessage{
			Role:       "tool",
			Name:       event.Request.Name,
			Content:    "Tool call rejected by user",
			ToolCallID: event.Request.CallID,
		})
	}

	delete(h.pendingApprovals, event.Request.CallID)
	return nil
}

// executeToolCall executes an approved tool call
func (h *TurnHandler) executeToolCall(_ context.Context, event ToolCallRequestEvent) error {
	tool, exists := h.tools[event.Name]
	if !exists {
		log.Printf("ERROR: Tool not found: %s (CallID: %s)", event.Name, event.CallID)
		return fmt.Errorf("tool not found: %s", event.Name)
	}

	log.Printf("Executing tool: %s (CallID: %s)", event.Name, event.CallID)

	// Execute the tool
	result, err := tool.Execute(event.Args)
	if err != nil {
		log.Printf("Tool execution failed: %v", err)
		result = &tools.ToolResult{
			LLMContent:    fmt.Sprintf("Error: %v", err),
			ReturnDisplay: fmt.Sprintf("❌ Error: %v", err),
			Error:         err,
		}
	}

	// Display result to user
	if result.ReturnDisplay != "" {
		fmt.Println(result.ReturnDisplay)
	}

	// Create tool response message
	content := result.LLMContent
	if result.Error != nil {
		content = fmt.Sprintf("Error: %v", result.Error)
	}

	toolResponse := openai.ChatCompletionMessage{
		Role:       "tool",
		Name:       event.Name,
		Content:    content,
		ToolCallID: event.CallID,
	}

	// Store the tool response
	h.toolResponses = append(h.toolResponses, toolResponse)
	log.Printf("Added tool response for %s (CallID: %s), total responses: %d", event.Name, event.CallID, len(h.toolResponses))

	// Mark as executed in scheduler
	h.scheduler.MarkExecuted(event.CallID, result, err)

	return nil
}

// handleError handles error events
func (h *TurnHandler) handleError(event ErrorEvent) error {
	log.Printf("Error: %s", event.Message)
	fmt.Printf("❌ Error: %s\n", event.Message)
	return event.Error
}

// handleUserCancelled handles cancellation
func (h *TurnHandler) handleUserCancelled() error {
	log.Println("User cancelled operation")
	fmt.Println("❌ Operation cancelled")
	return fmt.Errorf("cancelled by user")
}

// GetToolResponses returns all tool response messages
func (h *TurnHandler) GetToolResponses() []openai.ChatCompletionMessage {
	return h.toolResponses
}

// jsonString converts a map to JSON string
func jsonString(args map[string]interface{}) string {
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(data)
}
