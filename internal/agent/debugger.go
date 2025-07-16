// debugger.go
package agent

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type Debugger interface {
	ShouldContinue(messages []openai.ChatCompletionMessage) bool
}

// InteractiveDebugger prompts user before LLM calls
type InteractiveDebugger struct {
	reader *bufio.Reader
}

func NewInteractiveDebugger() *InteractiveDebugger {
	return &InteractiveDebugger{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (d *InteractiveDebugger) ShouldContinue(messages []openai.ChatCompletionMessage) bool {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🛑 DEBUG: About to call LLM")
	fmt.Println(strings.Repeat("=", 80))

	// Show conversation summary
	fmt.Printf("\nConversation history (%d messages):\n", len(messages))
	for i, msg := range messages {
		preview := msg.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("[%d] %s: %s\n", i, msg.Role, preview)

		if len(msg.ToolCalls) > 0 {
			fmt.Printf("    Tool calls: ")
			for _, tc := range msg.ToolCalls {
				fmt.Printf("%s ", tc.Function.Name)
			}
			fmt.Println()
		}
	}

	// Show last message in full
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		fmt.Printf("\nLast message (role: %s):\n", lastMsg.Role)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println(lastMsg.Content)
		fmt.Println(strings.Repeat("-", 40))
	}

	fmt.Print("\nContinue with LLM call? (y/n/q): ")

	input, err := d.reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v", err)
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "y", "yes", "":
		return true
	case "q", "quit":
		fmt.Println("Exiting...")
		os.Exit(0)
	default:
		return false
	}
	return true
}

// NoOpDebugger always continues (for production)
type NoOpDebugger struct{}

func (d *NoOpDebugger) ShouldContinue(messages []openai.ChatCompletionMessage) bool {
	return true
}
