package tools

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// generateSubAgentID creates a unique identifier for sub-agents
func generateSubAgentID() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("SA-%04d", rand.Intn(10000))
}

// getSystemPromptForAgentType returns appropriate system prompt based on agent type
func getSystemPromptForAgentType(agentType string) string {
	switch agentType {
	case "searcher":
		return "You are a specialized search agent. Your task is to efficiently search for files, code, and information. Use read-only tools like grep, glob, read_file, and list_files. Be thorough in your search and report all findings clearly."
	case "analyzer":
		return "You are a code analysis agent. Your task is to analyze code structure, patterns, and quality. Read files carefully, identify patterns, potential issues, and provide insights. Focus on understanding the codebase architecture and design."
	case "executor":
		return "You are an execution agent specialized in running commands and tests. Use run_shell to execute commands, run tests, and gather execution results. Report outputs, errors, and status clearly."
	case "general-purpose":
		fallthrough
	default:
		return "You are a helpful AI assistant performing a sub-task. Complete the task efficiently using all available tools and report your findings clearly."
	}
}

// AgentTool allows spawning sub-agents for complex tasks
type AgentTool struct {
	// We'll use interfaces to avoid circular dependencies
	llmClient    interface{}
	agentFactory func(llmClient interface{}, agentType string) (AgentInterface, error)
}

// AgentInterface defines the minimal interface needed for sub-agents
type AgentInterface interface {
	ExecuteWithHistory(ctx context.Context, conversation []interface{}, dryrun bool) (*AgentExecutionResult, []interface{}, error)
}

// AgentExecutionResult mirrors the agent.ExecutionResult structure
type AgentExecutionResult struct {
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

// NewAgentTool creates a new agent tool
func NewAgentTool(llmClient interface{}, agentFactory func(interface{}, string) (AgentInterface, error)) *AgentTool {
	return &AgentTool{
		llmClient:    llmClient,
		agentFactory: agentFactory,
	}
}

func (t *AgentTool) Name() string {
	return "agent_tool"
}

func (t *AgentTool) Description() string {
	return "Launch a new agent to handle complex, multi-step tasks autonomously"
}

func (t *AgentTool) ReadOnly() bool {
	return false // Sub-agents can perform write operations
}

func (t *AgentTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"description": map[string]interface{}{
				"type":        "string",
				"description": "A short (3-5 word) description of the task",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
			"agent_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of agent: general-purpose, searcher, analyzer, executor (default: general-purpose)",
				"enum":        []string{"general-purpose", "searcher", "analyzer", "executor"},
			},
		},
		"required": []string{"description", "prompt"},
	}
}

func (t *AgentTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	description, ok := args["description"].(string)
	if !ok {
		return nil, fmt.Errorf("description is required")
	}

	prompt, ok := args["prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("prompt is required")
	}

	// Get agent type, default to general-purpose
	agentType, _ := args["agent_type"].(string)
	if agentType == "" {
		agentType = "general-purpose"
	}

	// Generate unique sub-agent ID
	subAgentID := generateSubAgentID()

	log.Printf("[%s] 🚀 LAUNCHING %s sub-agent for task: %s", subAgentID, agentType, description)
	log.Printf("[%s] Prompt: %s", subAgentID, prompt)

	// Create the sub-agent using the factory with agent type
	log.Printf("[%s] Creating sub-agent instance...", subAgentID)
	subAgent, err := t.agentFactory(t.llmClient, agentType)
	if err != nil {
		log.Printf("[%s] ❌ Failed to create sub-agent: %v", subAgentID, err)
		return nil, fmt.Errorf("failed to create sub-agent: %w", err)
	}
	log.Printf("[%s] ✅ Sub-agent instance created", subAgentID)

	// Create initial conversation for sub-agent based on type
	systemPrompt := getSystemPromptForAgentType(agentType)
	conversation := []interface{}{
		map[string]interface{}{
			"role":    "system",
			"content": systemPrompt,
		},
		map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
		map[string]interface{}{
			"role":    "system",
			"content": fmt.Sprintf("[SUB-AGENT-CONTEXT] You are sub-agent %s", subAgentID),
		},
	}

	// Execute the sub-agent task
	ctx := context.Background()
	log.Printf("[%s] 🔄 Starting sub-agent execution...", subAgentID)
	startTime := time.Now()

	result, _, err := subAgent.ExecuteWithHistory(ctx, conversation, false)

	duration := time.Since(startTime)
	if err != nil {
		log.Printf("[%s] ❌ Sub-agent execution FAILED after %v: %v", subAgentID, duration, err)
		return &ToolResult{
			LLMContent:    fmt.Sprintf("Sub-agent %s failed for task '%s': %v", subAgentID, description, err),
			ReturnDisplay: fmt.Sprintf("❌ Sub-agent %s failed: %v", subAgentID, err),
			Error:         err,
		}, nil
	}

	log.Printf("[%s] ✅ Sub-agent execution COMPLETED in %v", subAgentID, duration)

	// Build response based on execution result
	llmContent := fmt.Sprintf("Sub-agent %s completed task '%s'.\n", subAgentID, description)
	displayContent := fmt.Sprintf("✅ Sub-agent %s completed: %s\n", subAgentID, description)

	// Log result summary
	log.Printf("[%s] 📊 EXECUTION SUMMARY:", subAgentID)
	log.Printf("[%s]   - Success: %v", subAgentID, result.Success)
	log.Printf("[%s]   - Total steps: %d", subAgentID, len(result.Steps))

	if result.Message != "" {
		log.Printf("[%s]   - Result message: %s", subAgentID, result.Message)
		llmContent += fmt.Sprintf("Result: %s", result.Message)
		displayContent += fmt.Sprintf("\n📋 Result:\n%s", result.Message)
	}

	// Log step details
	if len(result.Steps) > 0 {
		log.Printf("[%s] 📝 Steps executed:", subAgentID)
		for i, step := range result.Steps {
			log.Printf("[%s]   Step %d: %s (tool: %s)", subAgentID, i+1, step.Action, step.ToolName)
		}
		llmContent += fmt.Sprintf("\nExecuted %d steps", len(result.Steps))
		displayContent += fmt.Sprintf("\n\n🔧 Execution summary: %d steps", len(result.Steps))
	}

	// Include file generation summary if any
	if len(result.GeneratedFiles) > 0 {
		log.Printf("[%s]   - Files generated: %d", subAgentID, len(result.GeneratedFiles))
		for _, file := range result.GeneratedFiles {
			log.Printf("[%s]     • %s", subAgentID, file.Path)
		}
		llmContent += fmt.Sprintf("\nGenerated %d files", len(result.GeneratedFiles))
		displayContent += fmt.Sprintf("\n\n📄 Generated %d file(s):", len(result.GeneratedFiles))
		for _, file := range result.GeneratedFiles {
			displayContent += fmt.Sprintf("\n  • %s", file.Path)
		}
	}

	log.Printf("[%s] 🏁 SUB-AGENT FINISHED - Returning control to parent", subAgentID)

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}
