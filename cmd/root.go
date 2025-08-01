package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/hooks"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

var (
	cfgFile        string
	debugMode      bool
	promptStr      string
	maxTurns       int
	allowedTools   string
	permissionMode string
	dangerousSkip  bool
	modelSelection string
)

var rootCmd = &cobra.Command{
	Use:   "agenticode",
	Short: "A self-driving coding agent",
	Long: `agenticode is a CLI tool that can:
- Generate React applications
- Create GitHub PRs automatically
- Summarize repository contents

This application is a tool to generate the needed files
to quickly create React apps, propose changes, and more.

Usage:
  agenticode                      # Interactive mode
  agenticode -p "prompt"          # Execute a single prompt
  agenticode code "description"   # Generate code from description`,
	RunE: runInteractiveMode,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.agenticode.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug mode (pause before each LLM call)")
	rootCmd.Flags().StringVarP(&promptStr, "prompt", "p", "", "Provide a prompt to execute (non-interactive mode)")
	rootCmd.Flags().IntVar(&maxTurns, "max-turns", 20, "Maximum number of turns for non-interactive mode")
	rootCmd.Flags().StringVar(&allowedTools, "allowedTools", "", "Comma-separated list of allowed tools")
	rootCmd.Flags().StringVar(&permissionMode, "permission-mode", "", "Permission mode: bypassPermissions")
	rootCmd.Flags().BoolVar(&dangerousSkip, "dangerously-skip-permissions", false, "Skip all permission checks (use with caution)")
	rootCmd.Flags().StringVarP(&modelSelection, "model", "m", "", "Model selection (e.g., 'default', 'fast', 'groq/llama3-8b')")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".agenticode")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func runInteractiveMode(cmd *cobra.Command, args []string) error {
	// Try to load providers configuration first
	var client llm.Client
	var err error

	// Check if providers configuration exists
	providersConfig := &llm.ProvidersConfig{
		Providers: make(map[string]llm.ProviderConfig),
		Models:    make(map[string]llm.ModelSelection),
	}

	// Load providers from viper
	if !viper.IsSet("providers") {
		return fmt.Errorf("failed to see Providers. add providers on config see .agenticode.yaml")
	}

	if err := viper.UnmarshalKey("providers", &providersConfig.Providers); err != nil {
		return fmt.Errorf("failed to load providers configuration: %w", err)
	}

	// Load model selections
	if viper.IsSet("models") {
		if err := viper.UnmarshalKey("models", &providersConfig.Models); err != nil {
			return fmt.Errorf("failed to load models configuration: %w", err)
		}
	}

	// Determine which model to use
	selectedModel := modelSelection
	if selectedModel == "" {
		// Try to use default model selection
		selectedModel = "default"
	}

	// Create client with multi-provider configuration
	client, err = llm.NewClient(llm.Config{
		ProvidersConfig: providersConfig,
		ModelSelection:  selectedModel,
	})

	if err != nil {
		// If specific model selection failed, try legacy configuration
		fmt.Printf("Warning: Failed to use multi-provider configuration: %v\n", err)
		fmt.Println("Falling back to legacy configuration...")
	}

	// Create agent
	maxSteps := viper.GetInt("general.max_steps")
	if maxSteps == 0 {
		maxSteps = 15
	}

	// Override maxSteps with maxTurns if prompt is provided
	if promptStr != "" && maxTurns > 0 {
		maxSteps = maxTurns
	}

	// Create interactive approver with auto-approval for safe tools
	approver := agent.NewInteractiveApprover()

	// Configure approver based on command line flags
	if dangerousSkip || permissionMode == "bypassPermissions" {
		// Auto-approve all tools when permissions are bypassed
		approver.SetAutoApprove([]string{"write_file", "run_shell", "edit", "read_file", "read", "list_files", "grep", "glob", "read_many_files", "todo_write", "todo_read"})
	} else {
		// Default: only auto-approve safe tools
		approver.SetAutoApprove([]string{"read_file", "read", "list_files", "grep", "glob", "read_many_files", "todo_write", "todo_read"})
	}

	// Get tools
	availableTools := tools.GetDefaultTools()

	// Filter tools if allowedTools is specified
	if allowedTools != "" {
		allowedList := strings.Split(allowedTools, ",")
		filteredTools := []tools.Tool{}
		for _, tool := range availableTools {
			for _, allowed := range allowedList {
				if tool.Name() == strings.TrimSpace(allowed) {
					filteredTools = append(filteredTools, tool)
					break
				}
			}
		}
		availableTools = filteredTools
	}

	// Load hook configuration
	projectDir, _ := os.Getwd()
	sessionID := fmt.Sprintf("session_%d", os.Getpid()) // Simple session ID for now

	var hookManager *hooks.Manager
	if hookConfig, err := loadHooksFromViper(); err == nil && hookConfig != nil {
		hookManager = hooks.NewManager(hookConfig, projectDir, debugMode, sessionID)
		log.Printf("Loaded hook configuration with %d hook types", countHookTypes(hookConfig))
	}

	// Build agent options
	opts := []agent.Option{
		agent.WithMaxSteps(maxSteps),
		agent.WithApprover(approver),
		agent.WithTools(availableTools),
	}

	if debugMode {
		opts = append(opts, agent.WithDebugger(agent.NewInteractiveDebugger()))
	}

	if hookManager != nil {
		opts = append(opts, agent.WithHookManager(hookManager))
	}

	agentInstance := agent.NewAgent(client, opts...)

	// Get model name for prompts
	pc, ok := client.(*llm.ProviderClient)
	if !ok {
		return fmt.Errorf("failed to load provider client")
	}

	modelName := pc.GetCurrentModel()
	conversation := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: agent.GetSystemPrompt(modelName),
		},
		{
			Role:    "developer",
			Content: agent.GetDeveloperPrompt(),
		},
	}

	// Check if prompt was provided via command line
	if promptStr != "" {
		// Non-interactive mode: execute the prompt and exit
		ctx := context.Background()

		// Execute UserPromptSubmit hooks
		finalPrompt := promptStr
		if hookManager != nil {
			hookInput := hooks.HookInput{
				Prompt: promptStr,
			}

			outputs, err := hookManager.ExecuteHooks(ctx, hooks.UserPromptSubmit, hookInput)
			if err != nil {
				log.Printf("UserPromptSubmit hook error: %v", err)
			}

			// Check if any hook blocks the prompt
			for _, output := range outputs {
				if output.Decision == "block" {
					return fmt.Errorf("prompt blocked by hook: %s", output.Reason)
				}
			}

			// Add any additional context from hooks
			if additionalContext := hookManager.GetAdditionalContext(outputs); additionalContext != "" {
				conversation = append(conversation, openai.ChatCompletionMessage{
					Role:    "system",
					Content: additionalContext,
				})
			}
		}

		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "user",
			Content: finalPrompt,
		},
		)

		fmt.Printf("🚀 Executing prompt with max %d turns...\n", maxSteps)

		response, _, err := agentInstance.ExecuteWithHistory(ctx, conversation, false)
		if err != nil {
			return fmt.Errorf("error executing prompt: %w", err)
		}

		// Display execution result
		if response.Success {
			fmt.Println("\n✅ Task completed successfully!")
		} else {
			fmt.Println("\n⚠️  Task did not complete successfully")
		}

		// Display the response
		if response.Message != "" {
			fmt.Printf("\n💬 Final message: %s\n", response.Message)
		}

		// Show execution steps summary
		if len(response.Steps) > 0 {
			fmt.Printf("\n📊 Execution summary: %d steps taken\n", len(response.Steps))
			for i, step := range response.Steps {
				if step.ToolName != "" {
					fmt.Printf("  %d. %s", i+1, step.ToolName)
					if step.Action != "" {
						fmt.Printf(" (%s)", step.Action)
					}
					fmt.Println()
				}
			}
		}

		// Show any generated files summary
		if len(response.GeneratedFiles) > 0 {
			fmt.Printf("\n📝 Generated %d file(s):\n", len(response.GeneratedFiles))
			for _, file := range response.GeneratedFiles {
				fmt.Printf("  • %s\n", file.Path)
			}
		}

		return nil
	}

	// Start interactive session
	fmt.Println("AgentiCode Interactive Mode")
	fmt.Println("Type 'exit' or 'quit' to end the session")
	fmt.Println("Type 'clear' to clear the conversation history")
	fmt.Println("Type 'compact' to compress conversation history into a summary")
	fmt.Println("Type 'init' to generate or update AGENTIC.md documentation")
	fmt.Println("Type 'history' to view conversation history")
	fmt.Println("Type 'todos' to view the todo store")
	fmt.Println("---")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle special commands
		switch strings.ToLower(input) {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return nil
		case "clear":
			conversation = []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: agent.GetSystemPrompt(modelName),
				},
				{
					Role:    "developer",
					Content: agent.GetDeveloperPrompt(),
				},
			}
			fmt.Println("Conversation history cleared.")
			continue
		case "compact":
			fmt.Println("\n🗜️ Compressing conversation history...")
			
			// Check if there's enough conversation to summarize
			if len(conversation) < 4 { // At least system, developer, and a user-assistant exchange
				fmt.Println("❌ Conversation too short to compress. Need at least one exchange.")
				continue
			}

			// Check if a summarization model is configured
			var summarizeClient llm.Client
			useSummarizeModel := false
			
			if viper.IsSet("models.summarize") {
				// Try to create a client for the summarization model
				summarizeConfig := &llm.ProvidersConfig{
					Providers: make(map[string]llm.ProviderConfig),
					Models:    make(map[string]llm.ModelSelection),
				}
				
				if err := viper.UnmarshalKey("providers", &summarizeConfig.Providers); err == nil {
					if err := viper.UnmarshalKey("models", &summarizeConfig.Models); err == nil {
						if sumClient, err := llm.NewClient(llm.Config{
							ProvidersConfig: summarizeConfig,
							ModelSelection:  "summarize",
						}); err == nil {
							summarizeClient = sumClient
							useSummarizeModel = true
						}
					}
				}
			}

			// Perform summarization
			result, err := agent.SummarizeConversation(
				context.Background(),
				client,
				conversation,
				useSummarizeModel,
				summarizeClient,
			)
			
			if err != nil {
				fmt.Printf("❌ Failed to compress conversation: %v\n", err)
				continue
			}

			// Create new conversation with summary
			summaryMessage := agent.CreateSummaryMessage(result.Summary, result)
			newConversation := []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: agent.GetSystemPrompt(modelName),
				},
				{
					Role:    "developer",
					Content: agent.GetDeveloperPrompt(),
				},
				{
					Role:    "assistant",
					Content: summaryMessage,
				},
			}

			// Replace conversation
			conversation = newConversation
			
			fmt.Printf("\n✅ Conversation compressed successfully!\n")
			fmt.Printf("📊 %d → %d tokens (%.1fx compression, saved %d tokens)\n",
				result.OriginalTokens,
				result.SummaryTokens,
				result.CompressionRatio,
				result.TokensSaved)
			continue
		case "init":
			fmt.Println("\n🚀 Initializing AGENTIC.md generation...")
			
			// Check if AGENTIC.md already exists
			agenticPath := "AGENTIC.md"
			existingContent := ""
			if _, err := os.Stat(agenticPath); err == nil {
				fmt.Println("📄 Found existing AGENTIC.md, will analyze and suggest improvements...")
				if content, err := os.ReadFile(agenticPath); err == nil {
					existingContent = string(content)
				}
			} else {
				fmt.Println("📝 Creating new AGENTIC.md for this codebase...")
			}

			// Get the init prompt
			initPrompt := agent.GetInitPrompt()
			
			// If there's existing content, add it to the context
			if existingContent != "" {
				initPrompt = fmt.Sprintf("%s\n\n---\nExisting AGENTIC.md content:\n---\n%s", initPrompt, existingContent)
			}

			// Add the init prompt to conversation
			conversation = append(conversation, openai.ChatCompletionMessage{
				Role:    "user",
				Content: initPrompt,
			})

			// Execute task with conversation history
			ctx := context.Background()
			response, updatedConversation, err := agentInstance.ExecuteWithHistory(ctx, conversation, false)
			if err != nil {
				fmt.Printf("❌ Error generating AGENTIC.md: %v\n", err)
				// Remove the init prompt from conversation if it failed
				conversation = conversation[:len(conversation)-1]
				continue
			}

			// Update conversation
			conversation = updatedConversation

			// Display the response
			if response.Message != "" {
				fmt.Printf("\n%s\n", response.Message)
			}

			// Check if AGENTIC.md was generated
			agenticGenerated := false
			for _, file := range response.GeneratedFiles {
				if file.Path == "AGENTIC.md" || file.Path == "./AGENTIC.md" {
					agenticGenerated = true
					fmt.Printf("\n✅ AGENTIC.md has been created/updated at: %s\n", file.Path)
					break
				}
			}

			if !agenticGenerated {
				fmt.Println("\n⚠️  AGENTIC.md content was generated but not written to file.")
				fmt.Println("This might be because:")
				fmt.Println("1. File write requires approval (run with --bypass-permissions to auto-approve)")
				fmt.Println("2. The agent needs explicit instruction to write the file")
				fmt.Println("\nYou can ask the agent to 'write the AGENTIC.md file' to save it.")
			} else {
				fmt.Println("\n✅ AGENTIC.md generation complete!")
			}
			continue
		case "history":
			fmt.Println("\n--- Conversation History ---")
			msgCount := 0
			for _, msg := range conversation {
				if msg.Role == "system" || msg.Role == "tool" {
					continue
				}
				msgCount++

				// Format the role for display
				displayRole := msg.Role
				if displayRole == "assistant" {
					displayRole = "AgentiCode"
				} else if displayRole == "user" {
					displayRole = "You"
				}

				// Truncate long messages for history display
				content := strings.TrimSpace(msg.Content)
				if len(content) > 200 {
					content = content[:197] + "..."
				}

				fmt.Printf("\n[%s]: %s\n", displayRole, content)
			}
			if msgCount == 0 {
				fmt.Println("No conversation history yet.")
			}
			fmt.Println("\n--- End of History ---")
			continue
		case "todos":
			todos := tools.GlobalTodoStore.ReadAll()
			fmt.Println("\n--- Todo Store ---")
			if len(todos) == 0 {
				fmt.Println("No todos found.")
			} else {
				for _, todo := range todos {
					// Format state for display
					stateIcon := "⚪"
					switch todo.State {
					case tools.TodoPending:
						stateIcon = "⚪"
					case tools.TodoInProgress:
						stateIcon = "🔵"
					case tools.TodoCompleted:
						stateIcon = "✅"
					}

					fmt.Printf("\n%s [%s] %s\n", stateIcon, todo.ID[:8], todo.Title)
					fmt.Printf("   State: %s\n", todo.State)
					fmt.Printf("   Created: %s\n", todo.CreatedAt.Format("2006-01-02 15:04:05"))
					fmt.Printf("   Updated: %s\n", todo.UpdatedAt.Format("2006-01-02 15:04:05"))
				}
			}
			fmt.Println("\n--- End of Todos ---")
			continue
		}

		// Execute UserPromptSubmit hooks
		finalInput := input
		ctx := context.Background()

		if hookManager != nil {
			hookInput := hooks.HookInput{
				Prompt: input,
			}

			outputs, err := hookManager.ExecuteHooks(ctx, hooks.UserPromptSubmit, hookInput)
			if err != nil {
				log.Printf("UserPromptSubmit hook error: %v", err)
			}

			// Check if any hook blocks the prompt
			blocked := false
			for _, output := range outputs {
				if output.Decision == "block" {
					fmt.Printf("❌ Prompt blocked by hook: %s\n", output.Reason)
					blocked = true
					break
				}
			}

			if blocked {
				continue // Skip this prompt
			}

			// Add any additional context from hooks
			if additionalContext := hookManager.GetAdditionalContext(outputs); additionalContext != "" {
				conversation = append(conversation, openai.ChatCompletionMessage{
					Role:    "system",
					Content: additionalContext,
				})
			}
		}

		// Add user message to conversation
		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "user",
			Content: finalInput,
		})

		// Execute task with conversation history
		response, updatedConversation, err := agentInstance.ExecuteWithHistory(ctx, conversation, false)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Update our conversation with the agent's updated version
		conversation = updatedConversation

		fmt.Printf("len conversation: %d \n", len(conversation))
		// Display the response
		if response.Message != "" {
			fmt.Printf("\n%s\n", response.Message)
		}

		// Show any generated files summary
		if len(response.GeneratedFiles) > 0 {
			fmt.Printf("\n📝 Summary: Generated %d file(s)\n", len(response.GeneratedFiles))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

// loadHooksFromViper loads hook configuration from viper
func loadHooksFromViper() (*hooks.HookConfig, error) {
	// Check if hooks are configured
	if !viper.IsSet("hooks") {
		return nil, nil
	}

	var config hooks.HookConfig
	if err := viper.UnmarshalKey("hooks", &config); err != nil {
		return nil, fmt.Errorf("failed to load hooks configuration: %w", err)
	}

	// Validate the configuration
	if err := hooks.ValidateHookConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid hooks configuration: %w", err)
	}

	return &config, nil
}

// countHookTypes counts the number of configured hook types
func countHookTypes(config *hooks.HookConfig) int {
	count := 0
	if len(config.PreToolUse) > 0 {
		count++
	}
	if len(config.PostToolUse) > 0 {
		count++
	}
	if len(config.UserPromptSubmit) > 0 {
		count++
	}
	if len(config.Notification) > 0 {
		count++
	}
	if len(config.Stop) > 0 {
		count++
	}
	if len(config.SubagentStop) > 0 {
		count++
	}
	if len(config.PreCompact) > 0 {
		count++
	}
	if len(config.SessionStart) > 0 {
		count++
	}
	return count
}
