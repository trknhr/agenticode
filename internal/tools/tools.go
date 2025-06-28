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
	Execute(args map[string]interface{}) (interface{}, error)
}

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file"
}

func (t *WriteFileTool) Execute(args map[string]interface{}) (interface{}, error) {
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

	return map[string]string{"status": "success", "path": path}, nil
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

func (t *RunShellTool) Execute(args map[string]interface{}) (interface{}, error) {
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

	result := map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
		"status": "success",
	}

	if err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
	}

	return result, nil
}

type ApplyPatchTool struct{}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Description() string {
	return "Apply a unified diff patch to files"
}

func (t *ApplyPatchTool) Execute(args map[string]interface{}) (interface{}, error) {
	// TODO: Implement patch application
	return nil, fmt.Errorf("not yet implemented")
}

func GetDefaultTools() []Tool {
	return []Tool{
		&WriteFileTool{},
		&RunShellTool{},
		&ApplyPatchTool{},
	}
}
