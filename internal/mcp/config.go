package mcp

import (
	"os"
	"strings"
)

// MCPType represents the type of MCP server connection
type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPHttp  MCPType = "http"
	MCPSse   MCPType = "sse"
)

// MCPConfig represents configuration for a single MCP server
type MCPConfig struct {
	Type     MCPType           `yaml:"type" mapstructure:"type"`         // Connection type: stdio, http, or sse
	Command  string            `yaml:"command" mapstructure:"command"`   // Command to run (for stdio)
	Args     []string          `yaml:"args" mapstructure:"args"`         // Arguments for command (for stdio)
	URL      string            `yaml:"url" mapstructure:"url"`           // URL for http/sse connections
	Env      map[string]string `yaml:"env" mapstructure:"env"`           // Environment variables
	Headers  map[string]string `yaml:"headers" mapstructure:"headers"`   // HTTP headers (for http/sse)
	Disabled bool              `yaml:"disabled" mapstructure:"disabled"` // Whether this server is disabled
}

// MCPServersConfig represents the complete MCP configuration
type MCPServersConfig struct {
	Servers map[string]MCPConfig `yaml:"servers" mapstructure:"servers"`
}

// ResolvedEnv returns environment variables with expanded values
func (m MCPConfig) ResolvedEnv() map[string]string {
	resolved := make(map[string]string)
	
	// Start with current environment
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			resolved[parts[0]] = parts[1]
		}
	}
	
	// Override with configured environment variables
	for k, v := range m.Env {
		// Expand environment variables in the value
		resolved[k] = os.ExpandEnv(v)
	}
	
	return resolved
}

// ResolvedHeaders returns headers with expanded environment variables
func (m MCPConfig) ResolvedHeaders() map[string]string {
	resolved := make(map[string]string)
	for k, v := range m.Headers {
		// Expand environment variables in header values
		resolved[k] = os.ExpandEnv(v)
	}
	return resolved
}

// Validate checks if the MCP configuration is valid
func (m MCPConfig) Validate() error {
	switch m.Type {
	case MCPStdio:
		if m.Command == "" {
			return &ConfigError{Field: "command", Message: "command is required for stdio type"}
		}
	case MCPHttp, MCPSse:
		if m.URL == "" {
			return &ConfigError{Field: "url", Message: "url is required for http/sse type"}
		}
	default:
		return &ConfigError{Field: "type", Message: "invalid type: must be stdio, http, or sse"}
	}
	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "mcp config error: " + e.Field + ": " + e.Message
}