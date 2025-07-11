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
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "agenticode",
	Short: "A self-driving coding agent",
	Long: `agenticode is a CLI tool that can:
- Generate React applications
- Create GitHub PRs automatically
- Summarize repository contents

This application is a tool to generate the needed files
to quickly create React apps, propose changes, and more.`,
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
	approver.SetAutoApprove([]string{"read_file", "read", "list_files", "grep", "glob", "read_many_files"})
	
	agentInstance := agent.New(client, agent.WithMaxSteps(maxSteps), agent.WithApprover(approver))

	// Start interactive session
	fmt.Println("AgentiCode Interactive Mode")
	fmt.Println("Type 'exit' or 'quit' to end the session")
	fmt.Println("Type 'clear' to clear the conversation history")
	fmt.Println("Type 'history' to view conversation history")
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
