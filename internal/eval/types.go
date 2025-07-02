package eval

import (
	"time"
)

// TestCase represents a single evaluation test case
type TestCase struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Prompt      string       `yaml:"prompt"`
	Expect      Expectations `yaml:"expect"`
	Criteria    []string     `yaml:"criteria"`
}

// Expectations defines what to check in generated files
type Expectations struct {
	Files []FileExpectation `yaml:"files"`
}

// FileExpectation defines expectations for a single file
type FileExpectation struct {
	Path          string   `yaml:"path"`
	ShouldContain []string `yaml:"should_contain"`
	ShouldExist   *bool    `yaml:"should_exist,omitempty"`
}

// EvalResult represents the evaluation result for a test case
type EvalResult struct {
	TestCase       *TestCase
	Success        bool
	Errors         []string
	Metrics        Metrics
	GeneratedFiles map[string]string
	OutputDir      string
	ExecutedAt     time.Time
}

// Metrics contains evaluation metrics
type Metrics struct {
	PassRate            float64
	StructureCompliance float64
	IntentAlignment     float64
	CodeQuality         float64
	Executability       bool
	GPTScore            *GPTEvaluation
}

// GPTEvaluation contains results from GPT-based evaluation
type GPTEvaluation struct {
	Score          int
	Reasoning      string
	Feedback       string
	CriteriaScores map[string]int
}
