package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/Masterminds/sprig/v3"
)

//go:embed prompts/system-prompt.md
var systemPromptTemplate string

//go:embed prompts/developer-prompt.md
var developerPromptTemplate string

// PromptData contains the data for template variables
type PromptData struct {
	WorkingDir       string
	IsGitRepo        bool
	Platform         string
	OSVersion        string
	Date             string
	ModelName        string
	CurrentBranch    string
	MainBranch       string
	GitStatus        string
	GitRecentCommits string
}

func GetSystemPrompt(modelName string) string {
	// Read the template file
	templateContent := systemPromptTemplate

	// Gather system information
	workingDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get working directory: %v", err))
	}

	// Prepare template data
	data := PromptData{
		WorkingDir: workingDir,
		IsGitRepo:  isGitRepository(),
		Platform:   runtime.GOOS,
		OSVersion:  getOSVersion(),
		Date:       time.Now().Format("2006-01-02"),
		ModelName:  modelName,
	}

	// Get git information if in a git repo
	if data.IsGitRepo {
		data.CurrentBranch = getGitCurrentBranch()
		data.MainBranch = getGitMainBranch()
		data.GitStatus = getGitStatus()
		data.GitRecentCommits = getGitRecentCommits()
	}

	// Create template with sprig functions
	tmpl, err := template.New("system-prompt").Funcs(sprig.FuncMap()).Parse(string(templateContent))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse system prompt template: %v", err))
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("Failed to execute system prompt template: %v", err))
	}

	return buf.String()
}

func GetDeveloperPrompt() string {
	return developerPromptTemplate
}

func isGitRepository() bool {
	_, err := os.Stat(".git")
	return err == nil
}

func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sw_vers", "-productVersion").Output()
		if err == nil {
			return fmt.Sprintf("Darwin %s", strings.TrimSpace(string(out)))
		}
	case "linux":
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			return fmt.Sprintf("Linux %s", strings.TrimSpace(string(out)))
		}
	case "windows":
		out, err := exec.Command("ver").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return runtime.GOOS
}

func getGitCurrentBranch() string {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func getGitMainBranch() string {
	// Try to get the main branch from git config
	out, err := exec.Command("git", "config", "--get", "init.defaultBranch").Output()
	if err == nil && len(out) > 0 {
		return strings.TrimSpace(string(out))
	}

	// Check if main or master exists
	branches, err := exec.Command("git", "branch", "-a").Output()
	if err != nil {
		return "main"
	}

	branchList := string(branches)
	if strings.Contains(branchList, "main") {
		return "main"
	} else if strings.Contains(branchList, "master") {
		return "master"
	}

	return "main"
}

func getGitStatus() string {
	out, err := exec.Command("git", "status", "--short").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getGitRecentCommits() string {
	out, err := exec.Command("git", "log", "--oneline", "-5").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
