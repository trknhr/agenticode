package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/llm"
	"github.com/trknhr/agenticode/internal/tools"
)

var (
	dryRun bool
)

var codeCmd = &cobra.Command{
	Use:   "code [natural language description]",
	Short: "Generate code from natural language description",
	Long: `Generate code from a natural language description.
	
The command will:
1. Parse your natural language request
2. Generate appropriate code using LLM
3. Show you the diff preview
4. Apply changes after confirmation

Examples:
  agenticode code "Create a React todo list with add/complete/delete"
  agenticode code "Add authentication to the express server"
  agenticode code "Write unit tests for the user service"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		description := args[0]

		if dryRun {
			fmt.Printf("🔍 Dry run mode - no files will be modified\n")
		}

		fmt.Printf("📝 Generating code for: %s\n\n", description)

		// Initialize LLM client
		apiKey := viper.GetString("openai.api_key")
		fmt.Println(apiKey)
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			fmt.Println("❌ Error: OpenAI API key not found")
			fmt.Println("Please set OPENAI_API_KEY environment variable or add it to config file")
			os.Exit(1)
		}

		llmClient, err := llm.NewClient(llm.Config{
			Provider: "openai",
			APIKey:   apiKey,
			Model:    viper.GetString("openai.model"),
		})
		if err != nil {
			fmt.Printf("❌ Failed to initialize LLM client: %v\n", err)
			os.Exit(1)
		}

		// Create agent with tools
		codeAgent := agent.New(llmClient, 
			agent.WithTools(tools.GetDefaultTools()),
			agent.WithMaxSteps(10),
		)

		// Execute the task
		ctx := context.Background()
		result, err := codeAgent.ExecuteTask(ctx, description)
		if err != nil {
			fmt.Printf("❌ Error executing task: %v\n", err)
			os.Exit(1)
		}

		// Show generated files
		if len(result.GeneratedFiles) > 0 {
			fmt.Println("\n📁 Generated files:")
			for _, file := range result.GeneratedFiles {
				fmt.Printf("  • %s\n", file.Path)
			}

			// Show file contents
			fmt.Println("\n📄 File contents:")
			for _, file := range result.GeneratedFiles {
				fmt.Printf("\n--- %s ---\n", file.Path)
				fmt.Println(file.Content)
				fmt.Println("--- EOF ---")
			}

			if !dryRun {
				// Ask for confirmation
				fmt.Print("\n✅ Apply these changes? (y/N): ")
				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(strings.ToLower(response))

				if response == "y" || response == "yes" {
					// Apply changes
					for _, file := range result.GeneratedFiles {
						err := os.MkdirAll(filepath.Dir(file.Path), 0755)
						if err != nil {
							fmt.Printf("❌ Failed to create directory for %s: %v\n", file.Path, err)
							continue
						}

						err = os.WriteFile(file.Path, []byte(file.Content), 0644)
						if err != nil {
							fmt.Printf("❌ Failed to write %s: %v\n", file.Path, err)
						} else {
							fmt.Printf("✅ Created %s\n", file.Path)
						}
					}
				} else {
					fmt.Println("❌ Changes cancelled")
				}
			}
		} else {
			fmt.Println("\n⚠️  No files were generated")
		}

		if result.Message != "" {
			fmt.Printf("\n💬 Agent message: %s\n", result.Message)
		}
	},
}

func init() {
	rootCmd.AddCommand(codeCmd)

	codeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
}
