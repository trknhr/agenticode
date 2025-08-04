package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Manager manages hook execution
type Manager struct {
	config     *HookConfig
	projectDir string
	debug      bool
	sessionID  string
	transcript string
	mu         sync.RWMutex
}

// NewManager creates a new hook manager
func NewManager(config *HookConfig, projectDir string, debug bool, sessionID string) *Manager {
	return &Manager{
		config:     config,
		projectDir: projectDir,
		debug:      debug,
		sessionID:  sessionID,
		transcript: filepath.Join(os.Getenv("HOME"), ".agenticode", "sessions", sessionID+".jsonl"),
	}
}

// SetConfig updates the hook configuration
func (m *Manager) SetConfig(config *HookConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// ExecuteHooks runs all hooks for the given event
func (m *Manager) ExecuteHooks(ctx context.Context, event HookEvent, input HookInput) ([]HookOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil, nil
	}

	// Set common fields
	input.SessionID = m.sessionID
	input.TranscriptPath = m.transcript
	input.CWD, _ = os.Getwd()
	input.HookEventName = event

	// Get hooks for this event
	matchers := m.getHookMatchers(event)
	if len(matchers) == 0 {
		return nil, nil
	}

	// Find matching hooks
	var hooks []Hook
	for _, matcher := range matchers {
		if m.matchesPattern(matcher.Matcher, input.ToolName, event) {
			hooks = append(hooks, matcher.Hooks...)
		}
	}

	if len(hooks) == 0 {
		return nil, nil
	}

	if m.debug {
		log.Printf("[DEBUG] Executing %d hooks for %s", len(hooks), event)
	}

	// Execute hooks in parallel
	var wg sync.WaitGroup
	results := make([]HookResult, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(idx int, h Hook) {
			defer wg.Done()
			results[idx] = m.executeHook(ctx, h, input)
		}(i, hook)
	}

	wg.Wait()

	// Process results
	var outputs []HookOutput
	for _, result := range results {
		output := m.processHookResult(event, result)
		if output != nil {
			outputs = append(outputs, *output)
		}
	}

	return outputs, nil
}

// getHookMatchers returns the hook matchers for a given event
func (m *Manager) getHookMatchers(event HookEvent) []HookMatcher {
	if m.config == nil {
		return nil
	}

	switch event {
	case PreToolUse:
		return m.config.PreToolUse
	case PostToolUse:
		return m.config.PostToolUse
	case UserPromptSubmit:
		return m.config.UserPromptSubmit
	case Notification:
		return m.config.Notification
	case Stop:
		return m.config.Stop
	case SubagentStop:
		return m.config.SubagentStop
	case PreCompact:
		return m.config.PreCompact
	case SessionStart:
		return m.config.SessionStart
	default:
		return nil
	}
}

// matchesPattern checks if a pattern matches the given value
func (m *Manager) matchesPattern(pattern, value string, event HookEvent) bool {
	// For non-tool events, always match (no matcher needed)
	if event != PreToolUse && event != PostToolUse {
		return true
	}

	// Empty pattern or "*" matches all
	if pattern == "" || pattern == "*" {
		return true
	}

	// Exact match
	if pattern == value {
		return true
	}

	// Try as regex
	if matched, _ := regexp.MatchString(pattern, value); matched {
		return true
	}

	return false
}

// executeHook executes a single hook command
func (m *Manager) executeHook(ctx context.Context, hook Hook, input HookInput) HookResult {
	result := HookResult{
		Hook: hook,
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	// Prepare input JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal input: %w", err)
		return result
	}

	// Set timeout
	timeout := hook.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", hook.Command)
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CLAUDE_PROJECT_DIR=%s", m.projectDir),
		fmt.Sprintf("AGENTICODE_PROJECT_DIR=%s", m.projectDir),
	)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err = cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err
		}
	} else {
		result.ExitCode = 0
	}

	if m.debug {
		log.Printf("[DEBUG] Hook command completed with status %d: %s", result.ExitCode, hook.Command)
		if result.Stdout != "" {
			log.Printf("[DEBUG] Stdout: %s", result.Stdout)
		}
		if result.Stderr != "" {
			log.Printf("[DEBUG] Stderr: %s", result.Stderr)
		}
	}

	// Try to parse JSON output if exit code is 0
	if result.ExitCode == 0 && result.Stdout != "" {
		var output HookOutput
		if err := json.Unmarshal([]byte(result.Stdout), &output); err == nil {
			result.Output = &output
		}
	}

	return result
}

// processHookResult processes the result of a hook execution
func (m *Manager) processHookResult(event HookEvent, result HookResult) *HookOutput {
	// Handle errors
	if result.Error != nil {
		log.Printf("Hook execution error: %v", result.Error)
		return nil
	}

	// If we have parsed JSON output, return it
	if result.Output != nil {
		return result.Output
	}

	// Otherwise, create output based on exit code
	output := &HookOutput{}

	switch result.ExitCode {
	case 0:
		// Success
		output.Continue = true
		// For UserPromptSubmit and SessionStart, stdout becomes additional context
		if event == UserPromptSubmit && result.Stdout != "" {
			output.HookSpecificOutput = UserPromptSubmitOutput{
				HookEventName:     string(UserPromptSubmit),
				AdditionalContext: result.Stdout,
			}
		} else if event == SessionStart && result.Stdout != "" {
			output.HookSpecificOutput = SessionStartOutput{
				HookEventName:     string(SessionStart),
				AdditionalContext: result.Stdout,
			}
		}

	case 2:
		// Blocking error
		output.Continue = false
		output.StopReason = result.Stderr

		// For PreToolUse, this means deny
		if event == PreToolUse {
			output.HookSpecificOutput = PreToolUseOutput{
				HookEventName:            string(PreToolUse),
				PermissionDecision:       "deny",
				PermissionDecisionReason: result.Stderr,
			}
		} else if event == PostToolUse || event == Stop || event == SubagentStop {
			output.Decision = "block"
			output.Reason = result.Stderr
		}

	default:
		// Non-blocking error
		output.Continue = true
		if result.Stderr != "" {
			log.Printf("Hook warning: %s", result.Stderr)
		}
	}

	return output
}

// ShouldBlockToolExecution checks if any hook output blocks tool execution
func (m *Manager) ShouldBlockToolExecution(outputs []HookOutput) (bool, string) {
	for _, output := range outputs {
		// Check common continue field
		if !output.Continue {
			return true, output.StopReason
		}

		// Check PreToolUse specific output
		if preToolOutput, ok := output.HookSpecificOutput.(PreToolUseOutput); ok {
			if preToolOutput.PermissionDecision == "deny" {
				return true, preToolOutput.PermissionDecisionReason
			}
		}

		// Check decision field
		if output.Decision == "deny" || output.Decision == "block" {
			return true, output.Reason
		}
	}
	return false, ""
}

// ShouldAutoApprove checks if any hook output auto-approves the action
func (m *Manager) ShouldAutoApprove(outputs []HookOutput) (bool, string) {
	for _, output := range outputs {
		// Check PreToolUse specific output
		if preToolOutput, ok := output.HookSpecificOutput.(PreToolUseOutput); ok {
			if preToolOutput.PermissionDecision == "allow" {
				return true, preToolOutput.PermissionDecisionReason
			}
		}

		// Check deprecated decision field
		if output.Decision == "approve" || output.Decision == "allow" {
			return true, output.Reason
		}
	}
	return false, ""
}

// GetAdditionalContext extracts additional context from hook outputs
func (m *Manager) GetAdditionalContext(outputs []HookOutput) string {
	var contexts []string

	for _, output := range outputs {
		// Check UserPromptSubmit specific output
		if promptOutput, ok := output.HookSpecificOutput.(UserPromptSubmitOutput); ok {
			if promptOutput.AdditionalContext != "" {
				contexts = append(contexts, promptOutput.AdditionalContext)
			}
		}

		// Check SessionStart specific output
		if sessionOutput, ok := output.HookSpecificOutput.(SessionStartOutput); ok {
			if sessionOutput.AdditionalContext != "" {
				contexts = append(contexts, sessionOutput.AdditionalContext)
			}
		}
	}

	return strings.Join(contexts, "\n")
}
