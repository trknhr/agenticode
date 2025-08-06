package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ClientState represents the current state of an MCP client
type ClientState int

const (
	StateDisabled ClientState = iota
	StateStarting
	StateConnected
	StateError
)

func (s ClientState) String() string {
	switch s {
	case StateDisabled:
		return "disabled"
	case StateStarting:
		return "starting"
	case StateConnected:
		return "connected"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ClientInfo holds information about an MCP client's state
type ClientInfo struct {
	Name        string
	State       ClientState
	Error       error
	Client      MCPClient
	ToolCount   int
	ConnectedAt time.Time
}

// ClientManager manages MCP client connections
type ClientManager struct {
	clients sync.Map // map[string]MCPClient
	states  sync.Map // map[string]ClientInfo
	mu      sync.RWMutex
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{}
}

// InitializeClient creates and initializes an MCP client
func (m *ClientManager) InitializeClient(ctx context.Context, name string, config MCPConfig) error {
	// Update state to starting
	m.updateState(name, StateStarting, nil, nil, 0)

	// Create the client
	client, err := CreateClient(config)
	if err != nil {
		m.updateState(name, StateError, err, nil, 0)
		return fmt.Errorf("failed to create client for %s: %w", name, err)
	}

	// Start the client (required for stdio clients)
	if err := client.Start(ctx); err != nil {
		m.updateState(name, StateError, err, nil, 0)
		client.Close()
		return fmt.Errorf("failed to start client %s: %w", name, err)
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
		m.updateState(name, StateError, err, nil, 0)
		client.Close()
		return fmt.Errorf("failed to initialize MCP client %s: %w", name, err)
	}

	// Store the client
	m.clients.Store(name, client)

	// Count tools
	toolsRequest := mcp.ListToolsRequest{}
	result, err := client.ListTools(ctx, toolsRequest)
	if err != nil {
		// Non-fatal: client is initialized but we couldn't list tools
		log.Printf("Warning: failed to list tools from %s: %v", name, err)
		m.updateState(name, StateConnected, nil, client, 0)
	} else {
		m.updateState(name, StateConnected, nil, client, len(result.Tools))
		log.Printf("MCP client %s connected with %d tools", name, len(result.Tools))
	}

	return nil
}

// GetClient retrieves a client by name
func (m *ClientManager) GetClient(name string) (MCPClient, error) {
	value, ok := m.clients.Load(name)
	if !ok {
		return nil, fmt.Errorf("client %s not found", name)
	}

	client, ok := value.(MCPClient)
	if !ok {
		return nil, fmt.Errorf("invalid client type for %s", name)
	}

	// Check if client is in error state
	if info, ok := m.states.Load(name); ok {
		if clientInfo, ok := info.(ClientInfo); ok && clientInfo.State == StateError {
			return nil, fmt.Errorf("client %s is in error state: %v", name, clientInfo.Error)
		}
	}

	return client, nil
}

// GetState returns the state of a specific client
func (m *ClientManager) GetState(name string) (ClientInfo, bool) {
	value, ok := m.states.Load(name)
	if !ok {
		return ClientInfo{}, false
	}
	info, ok := value.(ClientInfo)
	return info, ok
}

// GetAllStates returns the states of all clients
func (m *ClientManager) GetAllStates() map[string]ClientInfo {
	states := make(map[string]ClientInfo)
	m.states.Range(func(key, value interface{}) bool {
		if name, ok := key.(string); ok {
			if info, ok := value.(ClientInfo); ok {
				states[name] = info
			}
		}
		return true
	})
	return states
}

// CloseAll closes all managed clients
func (m *ClientManager) CloseAll() {
	m.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(MCPClient); ok {
			if err := client.Close(); err != nil {
				log.Printf("Error closing client %v: %v", key, err)
			}
		}
		return true
	})
	m.clients = sync.Map{}
	m.states = sync.Map{}
}

// updateState updates the state of a client
func (m *ClientManager) updateState(name string, state ClientState, err error, client MCPClient, toolCount int) {
	info := ClientInfo{
		Name:      name,
		State:     state,
		Error:     err,
		Client:    client,
		ToolCount: toolCount,
	}
	if state == StateConnected {
		info.ConnectedAt = time.Now()
	}
	m.states.Store(name, info)
}

// GetTools retrieves tools from a specific MCP server using the manager
func (m *ClientManager) GetTools(ctx context.Context, name string) ([]mcp.Tool, error) {
	client, err := m.GetClient(name)
	if err != nil {
		return nil, err
	}

	toolsRequest := mcp.ListToolsRequest{}
	result, err := client.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from %s: %w", name, err)
	}

	return result.Tools, nil
}