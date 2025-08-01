package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Tool interface {
	Name() string
	Description() string
	ReadOnly() bool
	Execute(args map[string]interface{}) (*ToolResult, error)
	GetParameters() map[string]interface{}
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	// LLMContent is the factual content to be included in the LLM history
	LLMContent string
	// ReturnDisplay is the user-friendly display content (can be markdown)
	ReturnDisplay string
	// Error indicates if the tool execution failed
	Error error
}

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Create a new file or overwrite an existing file with specified content (WARNING: destroys existing content)"
}

func (t *WriteFileTool) ReadOnly() bool {
	return false
}

func (t *WriteFileTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Count lines in the content
	lines := strings.Count(content, "\n") + 1

	return &ToolResult{
		LLMContent:    fmt.Sprintf("Successfully wrote %d lines to %s", lines, path),
		ReturnDisplay: fmt.Sprintf("✅ Created file: `%s` (%d lines)", path, lines),
		Error:         nil,
	}, nil
}

func (t *WriteFileTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file path to write to",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

type RunShellTool struct{}

func NewRunShellTool() *RunShellTool {
	return &RunShellTool{}
}

func (t *RunShellTool) Name() string {
	return "run_shell"
}

func (t *RunShellTool) Description() string {
	return "Execute a shell command"
}

func (t *RunShellTool) ReadOnly() bool {
	return false
}

func (t *RunShellTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command is required")
	}

	// Security: Basic command validation
	dangerousCommands := []string{"rm -rf", "sudo", "chmod 777", "curl | sh", "wget | sh"}
	lowerCommand := strings.ToLower(command)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCommand, dangerous) {
			return nil, fmt.Errorf("potentially dangerous command blocked: %s", command)
		}
	}

	// Execute command
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Build LLM content
	llmContent := fmt.Sprintf("Executed: %s", command)
	if stdoutStr != "" {
		llmContent += fmt.Sprintf("\nStdout:\n%s", stdoutStr)
	}
	if stderrStr != "" {
		llmContent += fmt.Sprintf("\nStderr:\n%s", stderrStr)
	}
	if err != nil {
		llmContent += fmt.Sprintf("\nError: %v", err)
	}

	// Build display content
	var displayContent string
	if err != nil {
		displayContent = fmt.Sprintf("❌ Command failed: `%s`\n", command)
		if stderrStr != "" {
			displayContent += fmt.Sprintf("```\n%s\n```", stderrStr)
		}
		displayContent += fmt.Sprintf("\nError: %v", err)
	} else {
		displayContent = fmt.Sprintf("✅ Executed: `%s`\n", command)
		if stdoutStr != "" {
			displayContent += fmt.Sprintf("```\n%s\n```", stdoutStr)
		}
	}

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         err,
	}, nil
}

func (t *RunShellTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read and display the contents of an existing file (use this to view, explain, or analyze files)"
}

func (t *ReadFileTool) ReadOnly() bool {
	return true
}

func (t *ReadFileTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The file path to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	lines := strings.Count(contentStr, "\n") + 1

	// For display, show line numbers
	var displayLines []string
	for i, line := range strings.Split(contentStr, "\n") {
		displayLines = append(displayLines, fmt.Sprintf("%4d | %s", i+1, line))
	}
	displayContent := fmt.Sprintf("📄 **%s** (%d lines):\n```\n%s\n```", path, lines, strings.Join(displayLines, "\n"))

	return &ToolResult{
		LLMContent:    fmt.Sprintf("File content of %s:\n%s", path, contentStr),
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}

type ListFilesTool struct{}

func NewListFilesTool() *ListFilesTool {
	return &ListFilesTool{}
}

func (t *ListFilesTool) Name() string {
	return "list_files"
}

func (t *ListFilesTool) Description() string {
	return "List files in a directory"
}

func (t *ListFilesTool) ReadOnly() bool {
	return true
}

func (t *ListFilesTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The directory path to list (defaults to current directory)",
			},
		},
	}
}

func (t *ListFilesTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	var displayLines []string
	dirCount := 0
	fileCount := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
			dirCount++
			displayLines = append(displayLines, fmt.Sprintf("📁 %s", name))
		} else {
			fileCount++
			info, _ := entry.Info()
			size := ""
			if info != nil {
				size = fmt.Sprintf(" (%d bytes)", info.Size())
			}
			displayLines = append(displayLines, fmt.Sprintf("📄 %s%s", name, size))
		}
		files = append(files, name)
	}

	llmContent := fmt.Sprintf("Directory listing of %s: %s", path, strings.Join(files, ", "))
	displayContent := fmt.Sprintf("📂 **%s** (%d directories, %d files):\n```\n%s\n```",
		path, dirCount, fileCount, strings.Join(displayLines, "\n"))

	return &ToolResult{
		LLMContent:    llmContent,
		ReturnDisplay: displayContent,
		Error:         nil,
	}, nil
}

type ApplyPatchTool struct{}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Description() string {
	return "Apply a unified diff patch to files"
}

func (t *ApplyPatchTool) ReadOnly() bool {
	return false
}

func (t *ApplyPatchTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"patch": map[string]interface{}{
				"type":        "string",
				"description": "The unified diff patch to apply",
			},
		},
		"required": []string{"patch"},
	}
}

func (t *ApplyPatchTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	// TODO: Implement patch application
	return nil, fmt.Errorf("not yet implemented")
}

func GetDefaultTools() []Tool {
	return []Tool{
		&WriteFileTool{},
		&RunShellTool{},
		&ReadTool{},
		&ReadFileTool{},
		&ListFilesTool{},
		&GrepTool{},
		&GlobTool{},
		&EditTool{},
		&MultiEditTool{},
		&ReadManyFilesTool{},
		&ApplyPatchTool{},
		&TodoWriteTool{},
		&TodoReadTool{},
	}
}

// GetDefaultToolsWithLLM returns default tools including those that need LLM access
func GetDefaultToolsWithLLM(llmClient interface{}) []Tool {
	tools := GetDefaultTools()

	// Add tools that require LLM client
	if llmClient != nil {
		// We need to import llm package here, but to avoid circular dependency,
		// we'll use interface{} and type assertion in web_fetch
		tools = append(tools, NewWebFetchTool(llmClient))
	}

	return tools
}
