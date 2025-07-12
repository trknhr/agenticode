package agent

import (
	"context"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// ToolCallStatus represents the status of a tool call
type ToolCallStatus int

const (
	StatusPending ToolCallStatus = iota
	StatusApproved
	StatusRejected
	StatusExecuted
	StatusFailed
)

// PendingToolCall represents a tool call awaiting approval
type PendingToolCall struct {
	ID           string
	ToolCall     openai.ToolCall
	Context      context.Context
	Status       ToolCallStatus
	Result       interface{}
	Error        error
	CreatedAt    time.Time
	ApprovedAt   *time.Time
	ExecutedAt   *time.Time
}

// ToolCallScheduler manages pending tool calls
type ToolCallScheduler struct {
	pendingCalls map[string]*PendingToolCall
	mu           sync.RWMutex
}

// NewToolCallScheduler creates a new scheduler
func NewToolCallScheduler() *ToolCallScheduler {
	return &ToolCallScheduler{
		pendingCalls: make(map[string]*PendingToolCall),
	}
}

// ScheduleToolCalls adds tool calls to the scheduler
func (s *ToolCallScheduler) ScheduleToolCalls(ctx context.Context, toolCalls []openai.ToolCall) []*PendingToolCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheduled := make([]*PendingToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		pending := &PendingToolCall{
			ID:        tc.ID,
			ToolCall:  tc,
			Context:   ctx,
			Status:    StatusPending,
			CreatedAt: time.Now(),
		}
		s.pendingCalls[tc.ID] = pending
		scheduled = append(scheduled, pending)
	}
	return scheduled
}

// GetPendingCalls returns all pending calls
func (s *ToolCallScheduler) GetPendingCalls() []*PendingToolCall {
	s.mu.RLock()
	defer s.mu.RUnlock()

	calls := make([]*PendingToolCall, 0, len(s.pendingCalls))
	for _, call := range s.pendingCalls {
		if call.Status == StatusPending {
			calls = append(calls, call)
		}
	}
	return calls
}

// ApproveCalls marks specified calls as approved
func (s *ToolCallScheduler) ApproveCalls(callIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, id := range callIDs {
		if call, exists := s.pendingCalls[id]; exists && call.Status == StatusPending {
			call.Status = StatusApproved
			call.ApprovedAt = &now
		}
	}
}

// RejectCalls marks specified calls as rejected
func (s *ToolCallScheduler) RejectCalls(callIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range callIDs {
		if call, exists := s.pendingCalls[id]; exists && call.Status == StatusPending {
			call.Status = StatusRejected
		}
	}
}

// MarkExecuted marks a call as executed with its result
func (s *ToolCallScheduler) MarkExecuted(callID string, result interface{}, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if call, exists := s.pendingCalls[callID]; exists {
		now := time.Now()
		call.ExecutedAt = &now
		call.Result = result
		call.Error = err
		if err != nil {
			call.Status = StatusFailed
		} else {
			call.Status = StatusExecuted
		}
	}
}

// GetApprovedCalls returns all approved but not yet executed calls
func (s *ToolCallScheduler) GetApprovedCalls() []*PendingToolCall {
	s.mu.RLock()
	defer s.mu.RUnlock()

	calls := make([]*PendingToolCall, 0)
	for _, call := range s.pendingCalls {
		if call.Status == StatusApproved {
			calls = append(calls, call)
		}
	}
	return calls
}

// Clear removes all completed calls (executed, failed, rejected)
func (s *ToolCallScheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, call := range s.pendingCalls {
		if call.Status != StatusPending && call.Status != StatusApproved {
			delete(s.pendingCalls, id)
		}
	}
}

// ApprovalRequest represents a request for user approval
type ApprovalRequest struct {
	RequestID           string
	ToolCalls           []*PendingToolCall
	Description         string
	Risks               map[string]RiskLevel
	ConfirmationDetails ToolCallConfirmationDetails
}

// ApprovalResponse represents the user's approval decision
type ApprovalResponse struct {
	RequestID   string
	Approved    bool
	ApprovedIDs []string
	RejectedIDs []string
	Reason      string
}

// RiskLevel represents the risk level of a tool
type RiskLevel int

const (
	RiskLow RiskLevel = iota    // Read-only operations
	RiskMedium                  // File modifications
	RiskHigh                    // System commands
)

// ToolApprover interface for different approval implementations
type ToolApprover interface {
	RequestApproval(ctx context.Context, request ApprovalRequest) (ApprovalResponse, error)
	NotifyExecution(toolCallID string, result interface{}, err error)
}

// AssessToolCallRisk evaluates the risk level of a tool call
func AssessToolCallRisk(toolName string) RiskLevel {
	switch toolName {
	case "read_file", "read", "list_files", "grep", "glob", "read_many_files":
		return RiskLow
	case "write_file", "edit", "apply_patch":
		return RiskMedium
	case "run_shell":
		return RiskHigh
	default:
		return RiskMedium // Default to medium for unknown tools
	}
}

// GetRiskIcon returns an icon for the risk level
func GetRiskIcon(level RiskLevel) string {
	switch level {
	case RiskLow:
		return "🟢"
	case RiskMedium:
		return "🟡"
	case RiskHigh:
		return "🔴"
	default:
		return "⚪"
	}
}

// GetRiskDescription returns a description for the risk level
func GetRiskDescription(level RiskLevel) string {
	switch level {
	case RiskLow:
		return "Safe (read-only)"
	case RiskMedium:
		return "Moderate (modifies files)"
	case RiskHigh:
		return "High (system commands)"
	default:
		return "Unknown"
	}
}