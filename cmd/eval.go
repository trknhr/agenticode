package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trknhr/agenticode/internal/eval"
	"github.com/trknhr/agenticode/internal/llm"
)

var (
	evalOutputPath string
	evalKeepFailed bool
	evalUseGPT     bool
	evalNoStatic   bool
	evalVerbose    bool
	evalSaveJSON   string
)

var evalCmd = &cobra.Command{
	Use:   "eval [test-file]",
	Short: "Evaluate code generation quality",
	Long: `Run evaluation tests to assess code generation quality.
	
Example:
  agenticode eval tests/codegen/http-server.yaml
  agenticode eval tests/codegen/http-server.yaml --use-gpt --verbose`,
	Args: cobra.ExactArgs(1),
	RunE: runEval,
}

var evalAllCmd = &cobra.Command{
	Use:   "eval-all [test-directory]",
	Short: "Run all evaluation tests in a directory",
	Long: `Run all evaluation tests in a directory.
	
Example:
  agenticode eval-all tests/codegen/
  agenticode eval-all tests/codegen/ --keep-failed --save-json=results.json`,
	Args: cobra.ExactArgs(1),
	RunE: runEvalAll,
}

func init() {
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(evalAllCmd)

	// Common flags for both eval commands
	for _, cmd := range []*cobra.Command{evalCmd, evalAllCmd} {
		cmd.Flags().StringVar(&evalOutputPath, "output-path", ".agenticode_output", "Output directory for generated files")
		cmd.Flags().BoolVar(&evalKeepFailed, "keep-failed", false, "Keep output files for failed tests")
		cmd.Flags().BoolVar(&evalUseGPT, "use-gpt", false, "Enable GPT-based evaluation")
		cmd.Flags().BoolVar(&evalNoStatic, "no-static-check", false, "Skip static checks")
		cmd.Flags().BoolVarP(&evalVerbose, "verbose", "v", false, "Verbose output")
		cmd.Flags().StringVar(&evalSaveJSON, "save-json", "", "Save results as JSON to specified file")
	}
}

func runEval(cmd *cobra.Command, args []string) error {
	testFile := args[0]

	// Load test case
	testCase, err := eval.LoadTestCase(testFile)
	if err != nil {
		return fmt.Errorf("failed to load test case: %w", err)
	}

	// Create LLM client
	llmClient, err := createLLMClient()
	if err != nil {
		return err
	}

	// Create runner with options
	runnerOpts := []eval.RunnerOption{
		eval.WithOutputPath(evalOutputPath),
		eval.WithKeepFailed(evalKeepFailed),
		eval.WithGPTEval(evalUseGPT),
		eval.WithNoStaticCheck(evalNoStatic),
	}

	// Create separate evaluation client if eval model is configured
	if evalUseGPT {
		evalModel := viper.GetString("openai.eval_model")
		mainModel := viper.GetString("openai.model")
		
		// Only create separate client if a different model is specified
		if evalModel != "" && evalModel != mainModel {
			evalClient, err := createEvalLLMClient(evalModel)
			if err != nil {
				return fmt.Errorf("failed to create evaluation LLM client: %w", err)
			}
			runnerOpts = append(runnerOpts, eval.WithEvalLLMClient(evalClient))
		}
	}

	runner := eval.NewRunner(llmClient, runnerOpts...)

	// Run evaluation
	ctx := context.Background()
	result, err := runner.Run(ctx, testCase)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	// Report results
	reporter := eval.NewReporter(evalVerbose)
	reporter.Report([]*eval.EvalResult{result})

	// Save JSON if requested
	if evalSaveJSON != "" {
		if err := reporter.SaveJSON([]*eval.EvalResult{result}, evalSaveJSON); err != nil {
			return fmt.Errorf("failed to save JSON results: %w", err)
		}
		fmt.Printf("\nResults saved to: %s\n", evalSaveJSON)
	}

	if !result.Success {
		return fmt.Errorf("evaluation failed")
	}

	return nil
}

func runEvalAll(cmd *cobra.Command, args []string) error {
	testDir := args[0]

	// Load all test cases
	testCases, err := eval.LoadTestCases(testDir)
	if err != nil {
		return fmt.Errorf("failed to load test cases: %w", err)
	}

	if len(testCases) == 0 {
		return fmt.Errorf("no test cases found in %s", testDir)
	}

	fmt.Printf("Found %d test cases\n", len(testCases))

	// Create LLM client
	llmClient, err := createLLMClient()
	if err != nil {
		return err
	}

	// Create runner with options
	runnerOpts := []eval.RunnerOption{
		eval.WithOutputPath(evalOutputPath),
		eval.WithKeepFailed(evalKeepFailed),
		eval.WithGPTEval(evalUseGPT),
		eval.WithNoStaticCheck(evalNoStatic),
	}

	// Create separate evaluation client if eval model is configured
	if evalUseGPT {
		evalModel := viper.GetString("openai.eval_model")
		mainModel := viper.GetString("openai.model")
		
		// Only create separate client if a different model is specified
		if evalModel != "" && evalModel != mainModel {
			evalClient, err := createEvalLLMClient(evalModel)
			if err != nil {
				return fmt.Errorf("failed to create evaluation LLM client: %w", err)
			}
			runnerOpts = append(runnerOpts, eval.WithEvalLLMClient(evalClient))
		}
	}

	runner := eval.NewRunner(llmClient, runnerOpts...)

	// Run all evaluations
	ctx := context.Background()
	results, err := runner.RunAll(ctx, testCases)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	// Report results
	reporter := eval.NewReporter(evalVerbose)
	reporter.Report(results)

	// Save JSON if requested
	if evalSaveJSON != "" {
		if err := reporter.SaveJSON(results, evalSaveJSON); err != nil {
			return fmt.Errorf("failed to save JSON results: %w", err)
		}
		fmt.Printf("\nResults saved to: %s\n", evalSaveJSON)
	}

	// Generate summary
	summary := reporter.GenerateSummary(results)
	if summary.Failed > 0 {
		return fmt.Errorf("%d/%d tests failed", summary.Failed, summary.TotalTests)
	}

	return nil
}

func createLLMClient() (llm.Client, error) {
	apiKey := viper.GetString("openai.api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not set. Please set OPENAI_API_KEY environment variable or add it to ~/.agenticode.yaml")
	}

	model := viper.GetString("openai.model")
	if model == "" {
		model = "gpt-4-turbo-preview"
	}

	return llm.NewOpenAIClient(apiKey, model), nil
}

func createEvalLLMClient(model string) (llm.Client, error) {
	apiKey := viper.GetString("openai.api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not set. Please set OPENAI_API_KEY environment variable or add it to ~/.agenticode.yaml")
	}

	// Default to gpt-3.5-turbo if no model specified
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	return llm.NewOpenAIClient(apiKey, model), nil
}
