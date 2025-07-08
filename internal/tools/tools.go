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

func (t *WriteFileTool) ReadOnly() bool {
	return false
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

	return map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("File '%s' written successfully.", args["path"]),
		"path":    path,
	}, nil
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

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read content from a file"
}

func (t *ReadFileTool) ReadOnly() bool {
	return true
}

func (t *ReadFileTool) Execute(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return map[string]interface{}{
		"status":  "success",
		"path":    path,
		"content": string(content),
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

func (t *ListFilesTool) Execute(args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []map[string]interface{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, map[string]interface{}{
			"name":    entry.Name(),
			"is_dir":  entry.IsDir(),
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"modtime": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return map[string]interface{}{
		"status": "success",
		"path":   path,
		"files":  files,
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

func (t *ApplyPatchTool) Execute(args map[string]interface{}) (interface{}, error) {
	// TODO: Implement patch application
	return nil, fmt.Errorf("not yet implemented")
}

func GetDefaultTools() []Tool {
	return []Tool{
		&WriteFileTool{},
		&RunShellTool{},
		&ReadFileTool{},
		&ListFilesTool{},
		&GrepTool{},
		&GlobTool{},
		&EditTool{},
		&ReadManyFilesTool{},
		&ApplyPatchTool{},
	}
}
