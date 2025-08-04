package agent

import "fmt"

// ToolFileConfirmationDetails represents file modification confirmation (for both edit_file and write_file)
type ToolFileConfirmationDetails struct {
	ToolName        string
	FilePath        string
	FileDiff        string // Unified diff format (empty for new files)
	IsNewFile       bool   // true if creating new file
	OriginalContent string // Current file content (empty if new)
	NewContent      string // What will be written
	Risk            RiskLevel
}

func (d *ToolFileConfirmationDetails) Type() string { return "file" }

func (d *ToolFileConfirmationDetails) Title() string {
	if d.IsNewFile {
		return fmt.Sprintf("Create new file: %s", d.FilePath)
	}
	return fmt.Sprintf("Modify file: %s", d.FilePath)
}

func (d *ToolFileConfirmationDetails) GetRisk() RiskLevel { return d.Risk }

// ToolExecConfirmationDetails represents command execution confirmation
type ToolExecConfirmationDetails struct {
	ToolName   string
	Command    string
	WorkingDir string
	Risk       RiskLevel
}

func (d *ToolExecConfirmationDetails) Type() string { return "exec" }

func (d *ToolExecConfirmationDetails) Title() string {
	return fmt.Sprintf("Execute command: %s", d.Command)
}

func (d *ToolExecConfirmationDetails) GetRisk() RiskLevel { return d.Risk }

// ToolInfoConfirmationDetails represents info/read operation confirmation
type ToolInfoConfirmationDetails struct {
	ToolName    string
	Description string
	Parameters  map[string]interface{}
	Risk        RiskLevel
}

func (d *ToolInfoConfirmationDetails) Type() string { return "info" }

func (d *ToolInfoConfirmationDetails) Title() string { return d.Description }

func (d *ToolInfoConfirmationDetails) GetRisk() RiskLevel { return d.Risk }
