package tools

import (
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// TodoState represents the state of a todo item
type TodoState string

const (
	TodoPending    TodoState = "pending"
	TodoInProgress TodoState = "in_progress"
	TodoCompleted  TodoState = "completed"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	State     TodoState `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TodoStore manages todo items in memory
type TodoStore struct {
	mu    sync.Mutex
	items map[string]TodoItem
}

// GlobalTodoStore is the singleton instance for todo storage
var GlobalTodoStore = &TodoStore{
	items: make(map[string]TodoItem),
}

// Upsert creates new todos or updates existing ones
func (s *TodoStore) Upsert(items []TodoItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, item := range items {
		if item.ID == "" {
			// Generate new ID for new items
			item.ID = ulid.Make().String()
			item.CreatedAt = now
		} else {
			// Preserve creation time for existing items
			if existing, exists := s.items[item.ID]; exists {
				item.CreatedAt = existing.CreatedAt
			} else {
				item.CreatedAt = now
			}
		}
		item.UpdatedAt = now
		s.items[item.ID] = item
	}
}

// ReadAll returns all todo items
func (s *TodoStore) ReadAll() []TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	todos := make([]TodoItem, 0, len(s.items))
	for _, item := range s.items {
		todos = append(todos, item)
	}

	// Sort by creation time (oldest first)
	// This ensures consistent ordering
	return todos
}

// Clear removes all todos (useful for testing)
func (s *TodoStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]TodoItem)
}
