package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// clientWrapper wraps the mcp-go client to implement our MCPClient interface
type clientWrapper struct {
	client *client.Client
}

func (c *clientWrapper) Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	return c.client.Initialize(ctx, request)
}

func (c *clientWrapper) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return c.client.ListTools(ctx, request)
}

func (c *clientWrapper) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return c.client.CallTool(ctx, request)
}

func (c *clientWrapper) Close() error {
	return c.client.Close()
}

func (c *clientWrapper) Start(ctx context.Context) error {
	return c.client.Start(ctx)
}

// MCPClient is an interface for MCP client operations
type MCPClient interface {
	Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	Close() error
	// Start is called before Initialize for clients that need it (e.g., stdio)
	Start(ctx context.Context) error
}

// CreateClient creates an MCP client based on the configuration
func CreateClient(config MCPConfig) (MCPClient, error) {
	switch config.Type {
	case MCPStdio:
		return createStdioClient(config)
	case MCPHttp:
		return createHTTPClient(config)
	case MCPSse:
		return createSSEClient(config)
	default:
		return nil, fmt.Errorf("unsupported MCP type: %s", config.Type)
	}
}

// createStdioClient creates a stdio-based MCP client
func createStdioClient(config MCPConfig) (MCPClient, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("command is required for stdio MCP client")
	}

	log.Printf("Creating stdio MCP client: %s %v", config.Command, config.Args)
	
	// Create environment slice from map
	envSlice := []string{}
	for k, v := range config.ResolvedEnv() {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	
	c, err := client.NewStdioMCPClient(
		config.Command,
		envSlice,
		config.Args...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdio MCP client: %w", err)
	}
	
	return &clientWrapper{client: c}, nil
}

// createHTTPClient creates an HTTP-based MCP client
func createHTTPClient(config MCPConfig) (MCPClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for HTTP MCP client")
	}

	log.Printf("Creating HTTP MCP client: %s", config.URL)
	
	// Use variadic arguments directly
	c, err := client.NewStreamableHttpClient(config.URL, transport.WithHTTPHeaders(config.ResolvedHeaders()))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP MCP client: %w", err)
	}
	
	return &clientWrapper{client: c}, nil
}

// createSSEClient creates an SSE-based MCP client
func createSSEClient(config MCPConfig) (MCPClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required for SSE MCP client")
	}

	log.Printf("Creating SSE MCP client: %s", config.URL)
	
	// Use variadic arguments directly
	c, err := client.NewSSEMCPClient(config.URL, client.WithHeaders(config.ResolvedHeaders()))
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE MCP client: %w", err)
	}
	
	return &clientWrapper{client: c}, nil
}

// GetTools retrieves available tools from an MCP server
func GetTools(ctx context.Context, name string, config MCPConfig) ([]mcp.Tool, error) {
	client, err := CreateClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for %s: %w", name, err)
	}
	defer client.Close()

	// Start the client (required for stdio clients)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start client %s: %w", name, err)
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "agenticode",
		Version: "1.0.0",
	}

	_, err = client.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client %s: %w", name, err)
	}

	// List available tools
	toolsRequest := mcp.ListToolsRequest{}
	result, err := client.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from %s: %w", name, err)
	}

	log.Printf("Found %d tools from MCP server %s", len(result.Tools), name)
	for _, tool := range result.Tools {
		log.Printf("  - %s: %s", tool.Name, tool.Description)
	}

	return result.Tools, nil
}