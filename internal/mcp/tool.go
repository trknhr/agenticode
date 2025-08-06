package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/tools"
)

// MCPTool wraps an MCP tool to implement the agenticode Tool interface
type MCPTool struct {
	serverName string
	tool       mcp.Tool
	mcpConfig  MCPConfig
	approver   agent.ToolApprover
	clientFunc func() (MCPClient, error) // Function to create client on demand (deprecated)
	manager    *ClientManager            // Client manager for connection reuse
}

// NewMCPTool creates a new MCP tool adapter (deprecated - use NewMCPToolWithManager)
func NewMCPTool(serverName string, tool mcp.Tool, mcpConfig MCPConfig, approver agent.ToolApprover, clientFunc func() (MCPClient, error)) *MCPTool {
	return &MCPTool{
		serverName: serverName,
		tool:       tool,
		mcpConfig:  mcpConfig,
		approver:   approver,
		clientFunc: clientFunc,
	}
}

// NewMCPToolWithManager creates a new MCP tool adapter with client manager
func NewMCPToolWithManager(serverName string, tool mcp.Tool, mcpConfig MCPConfig, approver agent.ToolApprover, manager *ClientManager) *MCPTool {
	return &MCPTool{
		serverName: serverName,
		tool:       tool,
		mcpConfig:  mcpConfig,
		approver:   approver,
		manager:    manager,
	}
}

// Name returns the tool name with MCP prefix
func (m *MCPTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", m.serverName, m.tool.Name)
}

// Description returns the tool description
func (m *MCPTool) Description() string {
	return m.tool.Description
}

// ReadOnly returns whether the tool is read-only
// For MCP tools, we default to false but could be enhanced to check tool metadata
func (m *MCPTool) ReadOnly() bool {
	// Could potentially check tool name patterns or metadata
	// For now, assume MCP tools can write
	return false
}

// Execute runs the MCP tool
func (m *MCPTool) Execute(args map[string]interface{}) (*tools.ToolResult, error) {
	ctx := context.Background()

	// Log the incoming arguments for debugging
	log.Printf("MCP tool %s executing with args: %+v", m.Name(), args)

	// For now, skip approval for MCP tools
	// TODO: Integrate with approval system properly

	// Get client from manager or create new one
	var client MCPClient
	var err error
	
	if m.manager != nil {
		// Use manager for client reuse
		client, err = m.manager.GetClient(m.serverName)
		if err != nil {
			return nil, fmt.Errorf("failed to get MCP client from manager: %w", err)
		}
		// Don't close the client when using manager - it's managed
	} else if m.clientFunc != nil {
		// Fallback to old behavior
		client, err = m.clientFunc()
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client: %w", err)
		}
		defer client.Close()
		
		// Initialize the client (only needed for non-manager clients)
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "agenticode",
			Version: "1.0.0",
		}

		_, err = client.Initialize(ctx, initRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no client manager or client function available")
	}

	// Validate required parameters before sending to MCP server
	if m.tool.InputSchema.Required != nil {
		for _, required := range m.tool.InputSchema.Required {
			if _, exists := args[required]; !exists {
				// Log detailed error for debugging
				log.Printf("MCP tool %s missing required parameter '%s'. Provided args: %+v, Required: %v", 
					m.Name(), required, args, m.tool.InputSchema.Required)
				return &tools.ToolResult{
					LLMContent:    fmt.Sprintf("Missing required parameter '%s' for MCP tool %s. Required parameters: %v", 
						required, m.tool.Name, m.tool.InputSchema.Required),
					ReturnDisplay: fmt.Sprintf("❌ Missing required parameter '%s'", required),
					Error:         fmt.Errorf("missing required parameter: %s", required),
				}, nil
			}
		}
	}

	// Prepare tool call request
	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = m.tool.Name
	toolRequest.Params.Arguments = args

	// Log the actual MCP request being sent
	log.Printf("Sending MCP request to %s: tool=%s, args=%+v", m.serverName, m.tool.Name, args)

	// Execute the tool
	result, err := client.CallTool(ctx, toolRequest)
	if err != nil {
		log.Printf("MCP tool execution error for %s: %v", m.Name(), err)
		// Check if this is a validation error from the MCP server
		if strings.Contains(err.Error(), "validation error") {
			return &tools.ToolResult{
				LLMContent:    fmt.Sprintf("MCP parameter validation error: %v\nExpected parameters: %+v\nReceived: %+v", 
					err, m.tool.InputSchema.Properties, args),
				ReturnDisplay: fmt.Sprintf("❌ Parameter validation error: %v", err),
				Error:         err,
			}, nil
		}
		return &tools.ToolResult{
			LLMContent:    fmt.Sprintf("MCP tool error: %v", err),
			ReturnDisplay: fmt.Sprintf("❌ MCP tool error: %v", err),
			Error:         err,
		}, nil
	}

	// Extract content from result
	output := ""
	for _, content := range result.Content {
		// MCP content is an interface, we need to type assert to concrete types
		if textContent, ok := content.(mcp.TextContent); ok {
			output += textContent.Text + "\n"
		} else {
			// For other content types, use string representation
			output += fmt.Sprintf("%v\n", content)
		}
	}

	// Notify approver of execution result if set
	if m.approver != nil {
		m.approver.NotifyExecution(fmt.Sprintf("mcp-call-%d", 1), output, nil)
	}

	return &tools.ToolResult{
		LLMContent:    output,
		ReturnDisplay: output,
		Error:         nil,
	}, nil
}

// GetParameters returns the tool parameters schema
func (m *MCPTool) GetParameters() map[string]interface{} {
	// Convert MCP tool input schema to agenticode format
	params := make(map[string]interface{})
	
	// MCP tools always have an InputSchema
	params["type"] = "object"
	params["properties"] = m.tool.InputSchema.Properties
	
	// Ensure required is always an array (even if empty)
	if m.tool.InputSchema.Required != nil {
		params["required"] = m.tool.InputSchema.Required
	} else {
		params["required"] = []string{}
	}
	
	// Log the schema for debugging
	log.Printf("MCP tool %s schema: properties=%+v, required=%v", 
		m.Name(), m.tool.InputSchema.Properties, m.tool.InputSchema.Required)
	
	return params
}