package hooks

import (
	"context"
	"time"
)

// HookEvent represents the type of event that triggered the hook
type HookEvent string

const (
	// Tool-related events
	PreToolUse  HookEvent = "PreToolUse"
	PostToolUse HookEvent = "PostToolUse"

	// User interaction events
	UserPromptSubmit HookEvent = "UserPromptSubmit"
	Notification     HookEvent = "Notification"

	// Agent lifecycle events
	Stop         HookEvent = "Stop"
	SubagentStop HookEvent = "SubagentStop"
	PreCompact   HookEvent = "PreCompact"
	SessionStart HookEvent = "SessionStart"
)

// HookInput represents the data passed to a hook
type HookInput struct {
	// Common fields
	SessionID      string    `json:"session_id"`
	TranscriptPath string    `json:"transcript_path"`
	CWD            string    `json:"cwd"`
	HookEventName  HookEvent `json:"hook_event_name"`

	// Tool-specific fields
	ToolName     string                 `json:"tool_name,omitempty"`
	ToolInput    map[string]interface{} `json:"tool_input,omitempty"`
	ToolResponse map[string]interface{} `json:"tool_response,omitempty"`

	// Event-specific fields
	Message            string `json:"message,omitempty"`             // For Notification
	Prompt             string `json:"prompt,omitempty"`              // For UserPromptSubmit
	StopHookActive     bool   `json:"stop_hook_active,omitempty"`    // For Stop/SubagentStop
	Trigger            string `json:"trigger,omitempty"`             // For PreCompact
	CustomInstructions string `json:"custom_instructions,omitempty"` // For PreCompact
	Source             string `json:"source,omitempty"`              // For SessionStart
}

// HookOutput represents the output from a hook execution
type HookOutput struct {
	// Common control fields
	Continue       bool   `json:"continue,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`

	// Decision control (for PreToolUse, PostToolUse, UserPromptSubmit, Stop)
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`

	// Hook-specific output
	HookSpecificOutput interface{} `json:"hookSpecificOutput,omitempty"`
}

// PreToolUseOutput represents hook-specific output for PreToolUse events
type PreToolUseOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"` // allow, deny, ask
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// UserPromptSubmitOutput represents hook-specific output for UserPromptSubmit events
type UserPromptSubmitOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// SessionStartOutput represents hook-specific output for SessionStart events
type SessionStartOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// Hook represents a command hook configuration
type Hook struct {
	Type    string        `json:"type"`              // Currently only "command" is supported
	Command string        `json:"command"`           // The bash command to execute
	Timeout time.Duration `json:"timeout,omitempty"` // Optional timeout
}

// HookMatcher represents a hook configuration with optional matcher
type HookMatcher struct {
	Matcher string `json:"matcher,omitempty"` // Pattern to match (for tool events)
	Hooks   []Hook `json:"hooks"`
}

// HookConfig represents the complete hooks configuration
type HookConfig struct {
	PreToolUse       []HookMatcher `json:"PreToolUse,omitempty"`
	PostToolUse      []HookMatcher `json:"PostToolUse,omitempty"`
	UserPromptSubmit []HookMatcher `json:"UserPromptSubmit,omitempty"`
	Notification     []HookMatcher `json:"Notification,omitempty"`
	Stop             []HookMatcher `json:"Stop,omitempty"`
	SubagentStop     []HookMatcher `json:"SubagentStop,omitempty"`
	PreCompact       []HookMatcher `json:"PreCompact,omitempty"`
	SessionStart     []HookMatcher `json:"SessionStart,omitempty"`
}

// HookExecutor interface for executing hooks
type HookExecutor interface {
	// ExecuteHooks runs all hooks for the given event
	ExecuteHooks(ctx context.Context, event HookEvent, input HookInput) ([]HookOutput, error)
}

// HookResult represents the result of a single hook execution
type HookResult struct {
	Hook     Hook
	Output   *HookOutput
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error
	Duration time.Duration
}
