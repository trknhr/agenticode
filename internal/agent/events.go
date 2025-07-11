package agent

import "github.com/sashabaranov/go-openai"

// EventType represents different types of events during agent execution
type EventType int

const (
	EventTypeContent EventType = iota
	EventTypeToolCallRequest
	EventTypeToolCallResponse
	EventTypeToolCallConfirmation
	EventTypeUserCancelled
	EventTypeError
	EventTypeUsageMetadata
	EventTypeThought
	EventTypeTurnComplete
)

// Event is the base interface for all events
type Event interface {
	Type() EventType
}

// ContentEvent represents text content from the LLM
type ContentEvent struct {
	Content string
}

func (e ContentEvent) Type() EventType { return EventTypeContent }

// ToolCallRequestEvent represents a request to execute a tool
type ToolCallRequestEvent struct {
	CallID           string
	Name             string
	Args             map[string]interface{}
	IsClientInitiated bool
}

func (e ToolCallRequestEvent) Type() EventType { return EventTypeToolCallRequest }

// ToolCallResponseEvent represents the result of a tool execution
type ToolCallResponseEvent struct {
	CallID        string
	Result        interface{}
	ReturnDisplay string
	Error         error
}

func (e ToolCallResponseEvent) Type() EventType { return EventTypeToolCallResponse }

// ToolCallConfirmationEvent represents a request for user confirmation
type ToolCallConfirmationEvent struct {
	Request ToolCallRequestEvent
	Details ToolCallConfirmationDetails
}

func (e ToolCallConfirmationEvent) Type() EventType { return EventTypeToolCallConfirmation }

// UserCancelledEvent indicates the user cancelled the operation
type UserCancelledEvent struct{}

func (e UserCancelledEvent) Type() EventType { return EventTypeUserCancelled }

// ErrorEvent represents an error during execution
type ErrorEvent struct {
	Error   error
	Message string
	Status  int
}

func (e ErrorEvent) Type() EventType { return EventTypeError }

// UsageMetadataEvent contains token usage information
type UsageMetadataEvent struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	DurationMs       int64
}

func (e UsageMetadataEvent) Type() EventType { return EventTypeUsageMetadata }

// ThoughtEvent represents internal reasoning from the LLM
type ThoughtEvent struct {
	Subject     string
	Description string
}

func (e ThoughtEvent) Type() EventType { return EventTypeThought }

// TurnCompleteEvent signals that a turn has completed all processing
type TurnCompleteEvent struct {
	Conversation []openai.ChatCompletionMessage
}

func (e TurnCompleteEvent) Type() EventType { return EventTypeTurnComplete }

// ToolCallConfirmationDetails contains details for confirmation
type ToolCallConfirmationDetails struct {
	ToolName    string
	Risk        RiskLevel
	Description string
	Arguments   map[string]interface{}
}

// EventHandler processes events emitted by the Turn
type EventHandler interface {
	HandleEvent(event Event) error
}

// EventStream allows yielding events during execution
type EventStream struct {
	events chan Event
	errors chan error
}

// NewEventStream creates a new event stream
func NewEventStream() *EventStream {
	return &EventStream{
		events: make(chan Event, 100),
		errors: make(chan error, 1),
	}
}

// Emit sends an event to the stream
func (s *EventStream) Emit(event Event) {
	select {
	case s.events <- event:
	default:
		// Buffer full, drop event (could log this)
	}
}

// Events returns the event channel for reading
func (s *EventStream) Events() <-chan Event {
	return s.events
}

// Close closes the event stream
func (s *EventStream) Close() {
	close(s.events)
}

// Error reports an error and closes the stream
func (s *EventStream) Error(err error) {
	select {
	case s.errors <- err:
	default:
	}
	s.Close()
}