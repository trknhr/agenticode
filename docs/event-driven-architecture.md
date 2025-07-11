# Event-Driven Architecture

This document describes the refactored event-driven architecture for AgentiCode, inspired by Google's Gemini implementation.

## Overview

The new architecture separates concerns by using events to communicate between the LLM interaction layer and the UI/approval layer. This provides better modularity, testability, and flexibility.

## Core Components

### 1. Turn Class (`turn.go`)
Manages a single interaction turn with the LLM:
- Calls the LLM and receives responses
- Emits events for different response types
- Does NOT handle approval logic directly

```go
turn := NewTurn(llmClient, tools, conversation)
events := turn.Run(ctx)

for event := range events {
    // Handle each event
}
```

### 2. Event System (`events.go`)
Defines event types and structures:
- `ContentEvent` - Text responses from LLM
- `ToolCallRequestEvent` - Tool execution requests
- `ToolCallConfirmationEvent` - Approval requests
- `ToolCallResponseEvent` - Tool execution results
- `ErrorEvent` - Error notifications
- `UserCancelledEvent` - User cancellation

### 3. Event Handlers (`handlers.go`)
Process events emitted by Turn:
- `TurnHandler` - Coordinates event handling
- Manages tool approval flow
- Executes approved tools
- Updates conversation state

### 4. AgentV2 (`agent_v2.go`)
The refactored agent that uses the event-driven architecture:
- Creates turns for each interaction
- Delegates event handling to TurnHandler
- Maintains execution state and results

## Benefits

### 1. Separation of Concerns
- LLM interaction (Turn) is separate from UI (Handlers)
- Approval logic is cleanly separated
- Easy to test each component independently

### 2. Flexibility
- Can easily add new event types
- Different UI implementations (CLI, Web, etc.) can consume same events
- Can record/replay events for debugging

### 3. Better Error Handling
- Errors are events, not exceptions
- Can gracefully handle partial failures
- Better user feedback through error events

### 4. Extensibility
- Easy to add new handlers
- Can intercept and modify event flow
- Plugin architecture possible

## Migration Path

To use the new architecture:

1. Replace `Agent` with `AgentV2` in your code
2. The API remains compatible:
   ```go
   // Old
   agent := agent.New(client, agent.WithMaxSteps(10))
   
   // New (same API)
   agent := agent.NewAgentV2(client, agent.WithMaxSteps(10))
   ```

3. Event handling is automatic in interactive mode
4. For custom implementations, you can:
   - Create custom event handlers
   - Subscribe to specific event types
   - Implement custom approval flows

## Example: Custom Event Handler

```go
type CustomHandler struct {
    // your fields
}

func (h *CustomHandler) HandleEvent(event agent.Event) error {
    switch e := event.(type) {
    case agent.ToolCallRequestEvent:
        // Custom handling for tool requests
        fmt.Printf("Tool requested: %s\n", e.Name)
    case agent.ContentEvent:
        // Custom content display
        fmt.Printf("AI: %s\n", e.Content)
    }
    return nil
}
```

## Architecture Diagram

```
┌─────────────┐     ┌──────────┐     ┌──────────────┐
│     LLM     │────▶│   Turn   │────▶│   Events     │
└─────────────┘     └──────────┘     └──────────────┘
                                             │
                                             ▼
┌─────────────┐     ┌──────────┐     ┌──────────────┐
│   Approver  │◀────│ Handler  │◀────│  EventStream │
└─────────────┘     └──────────┘     └──────────────┘
                           │
                           ▼
                    ┌──────────┐
                    │  Tools   │
                    └──────────┘
```

## Future Enhancements

1. **Event Recording/Replay**
   - Record all events for debugging
   - Replay events for testing

2. **Event Filtering**
   - Subscribe to specific event types
   - Filter events based on criteria

3. **Async Event Processing**
   - Process events concurrently
   - Better performance for multiple tools

4. **Event Middleware**
   - Intercept and modify events
   - Add logging, metrics, etc.

5. **WebSocket Support**
   - Stream events to web clients
   - Real-time UI updates