package eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/llm"
)

// Runner executes evaluation test cases
type Runner struct {
	agent         *agent.Agent
	outputPath    string
	keepFailed    bool
	useGPT        bool
	noStaticCheck bool
}

// NewRunner creates a new evaluation runner
func NewRunner(llmClient llm.Client, opts ...RunnerOption) *Runner {
	a := agent.New(llmClient, agent.WithMaxSteps(10))

	r := &Runner{
		agent:      a,
		outputPath: ".agenticode_output",
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

	if r.useGPT && tc.EvalMode == "gpt" {
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
	// TODO: Implement GPT evaluation
	// This would use the LLM client to evaluate the generated code
	// based on the criteria specified in the test case
	return nil
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
