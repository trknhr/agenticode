package agent

// ApprovalConfig contains configuration for the approval system
type ApprovalConfig struct {
	// Mode can be "interactive", "auto", or "policy"
	Mode string `yaml:"mode" json:"mode"`

	// BatchMode can be "all", "by_type", or "individual"
	BatchMode string `yaml:"batch_mode" json:"batch_mode"`

	// AutoApprove lists tool names that should be automatically approved
	AutoApprove []string `yaml:"auto_approve" json:"auto_approve"`

	// RequireApproval lists tool names that always require approval
	RequireApproval []string `yaml:"require_approval" json:"require_approval"`

	// DefaultApprove determines the default action when no specific rule applies
	DefaultApprove bool `yaml:"default_approve" json:"default_approve"`

	// TimeoutSeconds is the timeout for approval requests
	TimeoutSeconds int `yaml:"timeout" json:"timeout"`
}

// DefaultApprovalConfig returns the default approval configuration
func DefaultApprovalConfig() *ApprovalConfig {
	return &ApprovalConfig{
		Mode:      "interactive",
		BatchMode: "by_type",
		AutoApprove: []string{
			"read_file",
			"read",
			"list_files",
			"grep",
			"glob",
			"read_many_files",
			"todo_write",
			"todo_read",
		},
		RequireApproval: []string{
			"run_shell",
			"write_file",
			"edit",
			"apply_patch",
		},
		DefaultApprove: false,
		TimeoutSeconds: 60,
	}
}

// NewApproverFromConfig creates an approver based on configuration
func NewApproverFromConfig(config *ApprovalConfig) ToolApprover {
	switch config.Mode {
	case "interactive":
		approver := NewInteractiveApprover()
		approver.SetAutoApprove(config.AutoApprove)
		approver.SetAutoReject([]string{}) // Could be configured
		return approver
	case "auto":
		// Future: implement auto approver based on policy
		return NewInteractiveApprover() // Fallback for now
	default:
		return NewInteractiveApprover()
	}
}
