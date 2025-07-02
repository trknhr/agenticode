package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/llm"
)

// Runner executes evaluation test cases
type Runner struct {
	agent         *agent.Agent
	llmClient     llm.Client
	evalLLMClient llm.Client // Separate LLM client for evaluations (can be a lighter model)
	outputPath    string
	keepFailed    bool
	useGPT        bool
	noStaticCheck bool
}

// NewRunner creates a new evaluation runner
func NewRunner(llmClient llm.Client, opts ...RunnerOption) *Runner {
	a := agent.New(llmClient, agent.WithMaxSteps(10))

	r := &Runner{
		agent:         a,
		llmClient:     llmClient,
		evalLLMClient: llmClient, // Default to same client, can be overridden
		outputPath:    ".agenticode_output",
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// RunnerOption configures the runner
type RunnerOption func(*Runner)

// WithOutputPath sets the output directory
func WithOutputPath(path string) RunnerOption {
	return func(r *Runner) {
		r.outputPath = path
	}
}

// WithKeepFailed preserves output for failed tests
func WithKeepFailed(keep bool) RunnerOption {
	return func(r *Runner) {
		r.keepFailed = keep
	}
}

// WithGPTEval enables GPT-based evaluation
func WithGPTEval(use bool) RunnerOption {
	return func(r *Runner) {
		r.useGPT = use
	}
}

// WithNoStaticCheck disables static checking
func WithNoStaticCheck(skip bool) RunnerOption {
	return func(r *Runner) {
		r.noStaticCheck = skip
	}
}

// WithEvalLLMClient sets a separate LLM client for evaluations
// This allows using a lighter/faster model for evaluations
func WithEvalLLMClient(client llm.Client) RunnerOption {
	return func(r *Runner) {
		r.evalLLMClient = client
	}
}

// Run executes a single test case
func (r *Runner) Run(ctx context.Context, tc *TestCase) (*EvalResult, error) {
	result := &EvalResult{
		TestCase:       tc,
		ExecutedAt:     time.Now(),
		GeneratedFiles: make(map[string]string),
		Errors:         []string{},
	}

	// Create output directory
	testID := strings.ReplaceAll(tc.Name, ".yaml", "")
	outputDir := filepath.Join(r.outputPath, testID)
	result.OutputDir = outputDir

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate code using agent
	files, err := r.agent.GenerateCode(ctx, tc.Prompt, true) // dry-run mode
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Code generation failed: %v", err))
		return result, nil
	}

	// Save generated files
	for path, content := range files {
		fullPath := filepath.Join(outputDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to create directory for %s: %v", path, err))
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to write file %s: %v", path, err))
			continue
		}
		result.GeneratedFiles[path] = content
	}

	// Run evaluations
	if !r.noStaticCheck {
		r.runStaticChecks(result, tc)
	}

	if r.useGPT {
		if err := r.runGPTEvaluation(ctx, result, tc); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("GPT evaluation failed: %v", err))
		}
	}

	// Calculate metrics
	r.calculateMetrics(result)

	// Clean up if successful and not keeping
	if result.Success && !r.keepFailed {
		os.RemoveAll(outputDir)
	}

	return result, nil
}

// RunAll executes all test cases
func (r *Runner) RunAll(ctx context.Context, testCases []*TestCase) ([]*EvalResult, error) {
	var results []*EvalResult

	for _, tc := range testCases {
		fmt.Printf("Running test: %s\n", tc.Name)
		result, err := r.Run(ctx, tc)
		if err != nil {
			return results, fmt.Errorf("failed to run test %s: %w", tc.Name, err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (r *Runner) runStaticChecks(result *EvalResult, tc *TestCase) {
	for _, fileExp := range tc.Expect.Files {
		content, exists := result.GeneratedFiles[fileExp.Path]

		// Check existence
		if fileExp.ShouldExist != nil {
			if *fileExp.ShouldExist && !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("Expected file %s does not exist", fileExp.Path))
				continue
			}
			if !*fileExp.ShouldExist && exists {
				result.Errors = append(result.Errors, fmt.Sprintf("File %s should not exist", fileExp.Path))
				continue
			}
		} else if !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("Expected file %s does not exist", fileExp.Path))
			continue
		}

		// Check content contains expected strings
		for _, expected := range fileExp.ShouldContain {
			if !strings.Contains(content, expected) {
				result.Errors = append(result.Errors,
					fmt.Sprintf("File %s does not contain expected string: %s", fileExp.Path, expected))
			}
		}
	}
}

func (r *Runner) runGPTEvaluation(ctx context.Context, result *EvalResult, tc *TestCase) error {
	// Build evaluation prompt
	prompt := r.buildEvaluationPrompt(tc, result.GeneratedFiles)

	// Create messages for the LLM
	messages := []openai.ChatCompletionMessage{
		{
			Role: "system",
			Content: `You are a code evaluation assistant. Your task is to evaluate generated code based on specific criteria.
Provide a detailed evaluation with:
1. An overall score from 1-10
2. Individual scores for each criterion (1-10)
3. Reasoning for your scores
4. Constructive feedback

Return your response in JSON format with the following structure:
{
  "overall_score": <number>,
  "criteria_scores": {
    "<criterion_name>": <number>
  },
  "reasoning": "<detailed reasoning>",
  "feedback": "<constructive feedback>"
}`,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Call LLM for evaluation (using eval-specific client)
	response, err := r.evalLLMClient.Generate(ctx, messages)
	if err != nil {
		return fmt.Errorf("failed to generate evaluation: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no response from LLM")
	}

	// Parse the JSON response
	var evalResult struct {
		OverallScore   int            `json:"overall_score"`
		CriteriaScores map[string]int `json:"criteria_scores"`
		Reasoning      string         `json:"reasoning"`
		Feedback       string         `json:"feedback"`
	}

	content := response.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &evalResult); err != nil {
		// Try to extract JSON from the response if it's wrapped in markdown
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start != -1 && end != -1 && end > start {
			jsonContent := content[start : end+1]
			if err := json.Unmarshal([]byte(jsonContent), &evalResult); err != nil {
				return fmt.Errorf("failed to parse evaluation response: %w", err)
			}
		} else {
			return fmt.Errorf("failed to parse evaluation response: %w", err)
		}
	}

	// Store GPT evaluation results
	result.Metrics.GPTScore = &GPTEvaluation{
		Score:          evalResult.OverallScore,
		Reasoning:      evalResult.Reasoning,
		Feedback:       evalResult.Feedback,
		CriteriaScores: evalResult.CriteriaScores,
	}

	// Update success based on score threshold (6/10 or higher)
	if evalResult.OverallScore < 6 {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("GPT evaluation score too low: %d/10", evalResult.OverallScore))
	}

	return nil
}

func (r *Runner) buildEvaluationPrompt(tc *TestCase, generatedFiles map[string]string) string {
	var sb strings.Builder

	// Add task description
	sb.WriteString("## Task Description\n")
	sb.WriteString(tc.Description + "\n\n")

	// Add original prompt
	sb.WriteString("## Original Prompt\n")
	sb.WriteString(tc.Prompt + "\n\n")

	// Add evaluation criteria
	if len(tc.Criteria) > 0 {
		sb.WriteString("## Evaluation Criteria\n")
		for _, criterion := range tc.Criteria {
			sb.WriteString(fmt.Sprintf("- %s\n", criterion))
		}
		sb.WriteString("\n")
	}

	// Add generated files
	sb.WriteString("## Generated Code\n")
	for path, content := range generatedFiles {
		sb.WriteString(fmt.Sprintf("### File: %s\n", path))
		sb.WriteString("```\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}

	// Add evaluation request
	sb.WriteString("## Evaluation Request\n")
	sb.WriteString("Please evaluate the generated code based on the criteria above. ")
	sb.WriteString("Consider correctness, completeness, code quality, and alignment with the original prompt.\n")

	return sb.String()
}

func (r *Runner) calculateMetrics(result *EvalResult) {
	// Calculate pass rate based on errors
	if len(result.Errors) == 0 {
		result.Metrics.PassRate = 1.0
		result.Success = true
	} else {
		result.Metrics.PassRate = 0.0
		result.Success = false
	}

	// TODO: Calculate other metrics
	// - Structure compliance
	// - Intent alignment
	// - Code quality
	// - Executability
}
