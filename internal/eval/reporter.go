package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// Reporter handles evaluation result reporting
type Reporter struct {
	verbose bool
}

// NewReporter creates a new reporter
func NewReporter(verbose bool) *Reporter {
	return &Reporter{verbose: verbose}
}

// Report prints evaluation results
func (r *Reporter) Report(results []*EvalResult) {
	if len(results) == 0 {
		fmt.Println("No test results to report")
		return
	}

	fmt.Println("\n📊 Evaluation Results")
	fmt.Println(strings.Repeat("=", 80))

	// Summary table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Test Case\tStatus\tPass Rate\tErrors")
	fmt.Fprintln(w, "---------\t------\t---------\t------")

	totalPassed := 0
	for _, result := range results {
		status := "❌ FAIL"
		if result.Success {
			status = "✅ PASS"
			totalPassed++
		}

		errorCount := len(result.Errors)
		fmt.Fprintf(w, "%s\t%s\t%.1f%%\t%d\n",
			result.TestCase.Name,
			status,
			result.Metrics.PassRate*100,
			errorCount,
		)
	}
	w.Flush()

	// Overall summary
	fmt.Printf("\nOverall: %d/%d passed (%.1f%%)\n",
		totalPassed,
		len(results),
		float64(totalPassed)/float64(len(results))*100,
	)

	// Detailed results if verbose
	if r.verbose {
		r.reportDetailed(results)
	}
}

func (r *Reporter) reportDetailed(results []*EvalResult) {
	for _, result := range results {
		fmt.Printf("\n\n📝 %s\n", result.TestCase.Name)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("Description: %s\n", result.TestCase.Description)
		fmt.Printf("Output Dir: %s\n", result.OutputDir)

		if len(result.Errors) > 0 {
			fmt.Println("\n❌ Errors:")
			for _, err := range result.Errors {
				fmt.Printf("  - %s\n", err)
			}
		}

		if len(result.GeneratedFiles) > 0 {
			fmt.Println("\n📁 Generated Files:")
			for path := range result.GeneratedFiles {
				fmt.Printf("  - %s\n", path)
			}
		}

		if result.Metrics.GPTScore != nil {
			fmt.Printf("\n🤖 GPT Evaluation:\n")
			fmt.Printf("  Score: %d/5\n", result.Metrics.GPTScore.Score)
			fmt.Printf("  Reasoning: %s\n", result.Metrics.GPTScore.Reasoning)
		}
	}
}

// SaveJSON saves results as JSON
func (r *Reporter) SaveJSON(results []*EvalResult, outputPath string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON results: %w", err)
	}

	return nil
}

// GenerateSummary creates a summary report
type Summary struct {
	TotalTests     int     `json:"total_tests"`
	Passed         int     `json:"passed"`
	Failed         int     `json:"failed"`
	PassRate       float64 `json:"pass_rate"`
	AverageMetrics Metrics `json:"average_metrics"`
}

func (r *Reporter) GenerateSummary(results []*EvalResult) Summary {
	summary := Summary{
		TotalTests: len(results),
	}

	var totalPassRate float64
	for _, result := range results {
		if result.Success {
			summary.Passed++
		} else {
			summary.Failed++
		}
		totalPassRate += result.Metrics.PassRate
	}

	if len(results) > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.TotalTests)
		summary.AverageMetrics.PassRate = totalPassRate / float64(len(results))
	}

	return summary
}
