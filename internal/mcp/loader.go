package mcp

import (
	"context"
	"log"
	"sync"

	"github.com/spf13/viper"
	"github.com/trknhr/agenticode/internal/agent"
	"github.com/trknhr/agenticode/internal/tools"
)

// LoadMCPTools loads all configured MCP tools with a client manager
func LoadMCPTools(ctx context.Context, approver agent.ToolApprover, v *viper.Viper) (*ClientManager, []tools.Tool) {
	var mcpConfigs map[string]MCPConfig
	
	// Check if MCP configuration exists in main config
	if v.IsSet("mcp") {
		if err := v.UnmarshalKey("mcp", &mcpConfigs); err != nil {
			log.Printf("Failed to load MCP configuration: %v", err)
			return nil, nil
		}
	} else if v.IsSet("mcp_config_file") {
		// Load from separate file
		mcpConfigFile := v.GetString("mcp_config_file")
		mcpViper := viper.New()
		mcpViper.SetConfigFile(mcpConfigFile)
		
		if err := mcpViper.ReadInConfig(); err != nil {
			log.Printf("Failed to read MCP config file %s: %v", mcpConfigFile, err)
			return nil, nil
		}
		
		// Try to unmarshal from "servers" key first (for dedicated MCP config file)
		if mcpViper.IsSet("servers") {
			if err := mcpViper.UnmarshalKey("servers", &mcpConfigs); err != nil {
				log.Printf("Failed to unmarshal MCP servers: %v", err)
				return nil, nil
			}
		} else {
			// Fallback to root level
			if err := mcpViper.Unmarshal(&mcpConfigs); err != nil {
				log.Printf("Failed to unmarshal MCP config: %v", err)
				return nil, nil
			}
		}
	} else {
		// No MCP configuration found
		return nil, nil
	}

	if len(mcpConfigs) == 0 {
		return nil, nil
	}

	log.Printf("Loading MCP tools from %d servers", len(mcpConfigs))
	
	// Create client manager
	manager := NewClientManager()
	
	// Initialize clients and load tools concurrently
	var wg sync.WaitGroup
	toolsChan := make(chan tools.Tool, 100)
	
	for name, config := range mcpConfigs {
		if config.Disabled {
			log.Printf("Skipping disabled MCP server: %s", name)
			continue
		}
		
		// Validate configuration
		if err := config.Validate(); err != nil {
			log.Printf("Invalid MCP configuration for %s: %v", name, err)
			continue
		}
		
		wg.Add(1)
		go func(serverName string, serverConfig MCPConfig) {
			defer wg.Done()
			
			log.Printf("Initializing MCP server: %s", serverName)
			
			// Initialize client in manager
			if err := manager.InitializeClient(ctx, serverName, serverConfig); err != nil {
				log.Printf("Failed to initialize client %s: %v", serverName, err)
				return
			}
			
			// Get tools from the manager
			mcpTools, err := manager.GetTools(ctx, serverName)
			if err != nil {
				log.Printf("Failed to get tools from %s: %v", serverName, err)
				return
			}
			
			// Create tool adapters
			for _, mcpTool := range mcpTools {
				toolAdapter := NewMCPToolWithManager(serverName, mcpTool, serverConfig, approver, manager)
				toolsChan <- toolAdapter
			}
		}(name, config)
	}
	
	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(toolsChan)
	}()
	
	// Collect all tools
	var allTools []tools.Tool
	for tool := range toolsChan {
		allTools = append(allTools, tool)
	}
	
	log.Printf("Loaded %d MCP tools total", len(allTools))
	return manager, allTools
}

// LoadMCPToolsWithDefaults loads MCP tools and combines them with default tools
func LoadMCPToolsWithDefaults(ctx context.Context, approver agent.ToolApprover, v *viper.Viper, defaultTools []tools.Tool) []tools.Tool {
	// Start with default tools
	allTools := append([]tools.Tool{}, defaultTools...)
	
	// Add MCP tools
	_, mcpTools := LoadMCPTools(ctx, approver, v)
	if len(mcpTools) > 0 {
		log.Printf("Adding %d MCP tools to %d default tools", len(mcpTools), len(defaultTools))
		allTools = append(allTools, mcpTools...)
	}
	
	// For backwards compatibility, just return tools without manager
	// TODO: Update callers to handle manager
	return allTools
}