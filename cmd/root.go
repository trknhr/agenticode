package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

var (
	cfgFile   string
	debugMode bool
	promptStr string
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
	// Initialize OpenAI client
	apiKey := viper.GetString("openai.api_key")
	if apiKey == "" {
		if apiKey = os.Getenv("OPENAI_API_KEY"); apiKey == "" {
			return fmt.Errorf("OpenAI API key not found. Set OPENAI_API_KEY environment variable or add it to config file")
		}
	}

	model := viper.GetString("openai.model")
	if model == "" {
		model = "gpt-4.1"
	}

	// Create LLM client
	client := llm.NewOpenAIClient(apiKey, model)

	// Create agent
	maxSteps := viper.GetInt("general.max_steps")
	if maxSteps == 0 {
		maxSteps = 15
	}

	// Create interactive approver with auto-approval for safe tools
	approver := agent.NewInteractiveApprover()
	approver.SetAutoApprove([]string{"read_file", "read", "list_files", "grep", "glob", "read_many_files", "todo_write", "todo_read"})
	
	// Build agent options
	opts := []agent.Option{
		agent.WithMaxSteps(maxSteps),
		agent.WithApprover(approver),
		agent.WithTools(tools.GetDefaultTools()),
	}
	
	if debugMode {
		opts = append(opts, agent.WithDebugger(agent.NewInteractiveDebugger()))
	}

	agentInstance := agent.NewAgent(client, opts...)

	// Check if prompt was provided via command line
	if promptStr != "" {
		// Non-interactive mode: execute the prompt and exit
		ctx := context.Background()
		conversation := []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: agent.GetCoreSystemPrompt(),
			},
			{
				Role:    "user",
				Content: promptStr,
			},
		}
		
		response, _, err := agentInstance.ExecuteWithHistory(ctx, conversation, false)
		if err != nil {
			return fmt.Errorf("error executing prompt: %w", err)
		}
		
		// Display the response
		if response.Message != "" {
			fmt.Printf("%s\n", response.Message)
		}
		
		// Show any generated files summary
		if len(response.GeneratedFiles) > 0 {
			fmt.Printf("\n📝 Summary: Generated %d file(s)\n", len(response.GeneratedFiles))
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
	fmt.Println("Type 'history' to view conversation history")
	fmt.Println("Type 'todos' to view the todo store")
	fmt.Println("---")

	scanner := bufio.NewScanner(os.Stdin)
	conversation := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: agent.GetCoreSystemPrompt(),
		},
	}

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
					Content: agent.GetCoreSystemPrompt(),
				},
			}
			fmt.Println("Conversation history cleared.")
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

		// Add user message to conversation
		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    "user",
			Content: input,
		})

		// Execute task with conversation history
		ctx := context.Background()
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
