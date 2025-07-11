# Changelog

## [Unreleased] - Event-Driven Architecture Refactor

### Added
- **Event-driven architecture** inspired by Google's Gemini implementation
  - `Turn` class manages LLM interactions and emits events
  - Event system with types: Content, ToolCallRequest, ToolCallConfirmation, etc.
  - `TurnHandler` processes events and manages tool execution
  - `AgentV2` implements the new architecture with API compatibility

- **Tool approval system** with risk assessment
  - Interactive approval UI for tool calls
  - Auto-approval for safe (read-only) operations
  - Risk levels: Low (🟢), Medium (🟡), High (🔴)
  - Options: approve all, reject all, or select individual tools
  - Configurable auto-approval lists

- **Documentation**
  - `docs/approval-system.md` - Comprehensive approval system guide
  - `docs/event-driven-architecture.md` - Architecture documentation
  - Migration guide for using AgentV2

### Changed
- Updated `cmd/root.go` to use `AgentV2` with event-driven architecture
- Updated `cmd/code.go` to use `AgentV2` with approval system
- Modified README.md to document the approval system
- All tool executions now go through the approval system (except auto-approved ones)

### Benefits
- **Better separation of concerns**: UI logic separated from LLM interaction
- **Improved testability**: Each component can be tested independently  
- **Enhanced security**: User has full control over tool execution
- **Greater flexibility**: Easy to add new UI implementations or event handlers
- **Cleaner architecture**: Event-driven design is more maintainable

### Migration
To use the new system, replace `agent.New()` with `agent.NewAgentV2()`. The API remains compatible, so existing code will work with minimal changes.