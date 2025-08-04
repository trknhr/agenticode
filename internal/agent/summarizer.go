package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/trknhr/agenticode/internal/llm"
)

// SummarizationResult contains the results of conversation summarization
type SummarizationResult struct {
	Summary          string
	OriginalTokens   int
	SummaryTokens    int
	TokensSaved      int
	CompressionRatio float64
}

// SummarizeConversation compresses a conversation history into a summary
func SummarizeConversation(ctx context.Context, client llm.Client, conversation []openai.ChatCompletionMessage, useAlternateModel bool, alternateClient llm.Client) (*SummarizationResult, error) {
	// Filter out system and tool messages for token counting
	userAssistantMessages := filterUserAssistantMessages(conversation)
	
	if len(userAssistantMessages) < 2 {
		return nil, fmt.Errorf("conversation too short to summarize (need at least 2 messages)")
	}

	// Estimate original token count (rough estimate: 1 token per 4 characters)
	originalTokens := estimateTokens(userAssistantMessages)

	// Create summarization prompt
	summarizationPrompt := buildSummarizationPrompt()

	// Prepare messages for summarization
	summarizeMessages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: "You are a helpful assistant that creates concise summaries of conversations while preserving all important context, decisions, and next steps.",
		},
	}

	// Add the conversation messages to summarize
	summarizeMessages = append(summarizeMessages, userAssistantMessages...)

	// Add the summarization request
	summarizeMessages = append(summarizeMessages, openai.ChatCompletionMessage{
		Role:    "user",
		Content: summarizationPrompt,
	})

	// Use alternate client if configured and available
	llmClient := client
	if useAlternateModel && alternateClient != nil {
		llmClient = alternateClient
		log.Println("Using alternate model for summarization")
	}

	// Generate the summary
	response, err := llmClient.Generate(ctx, summarizeMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from summarization model")
	}

	summary := strings.TrimSpace(response.Choices[0].Message.Content)
	if summary == "" {
		return nil, fmt.Errorf("empty summary returned")
	}

	// Calculate metrics
	summaryTokens := estimateTokens([]openai.ChatCompletionMessage{{Content: summary}})
	tokensSaved := originalTokens - summaryTokens
	compressionRatio := float64(originalTokens) / float64(summaryTokens)

	// Log summarization metrics
	log.Printf("Summarization complete: %d tokens -> %d tokens (%.1fx compression, saved %d tokens)",
		originalTokens, summaryTokens, compressionRatio, tokensSaved)

	return &SummarizationResult{
		Summary:          summary,
		OriginalTokens:   originalTokens,
		SummaryTokens:    summaryTokens,
		TokensSaved:      tokensSaved,
		CompressionRatio: compressionRatio,
	}, nil
}

// filterUserAssistantMessages removes system and tool messages from conversation
func filterUserAssistantMessages(conversation []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	filtered := make([]openai.ChatCompletionMessage, 0)
	for _, msg := range conversation {
		if msg.Role == "user" || msg.Role == "assistant" {
			// Skip developer messages and system messages
			if msg.Role == "assistant" || (msg.Role == "user" && !strings.Contains(msg.Content, "[SYSTEM]")) {
				filtered = append(filtered, msg)
			}
		}
	}
	return filtered
}

// estimateTokens provides a rough token count estimate
func estimateTokens(messages []openai.ChatCompletionMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		// Add overhead for message structure
		totalChars += 10 // Rough estimate for role and message metadata
	}
	// Rough estimate: 1 token per 4 characters for English text
	return totalChars / 4
}

// buildSummarizationPrompt creates the prompt for summarization
func buildSummarizationPrompt() string {
	return `Please provide a comprehensive but concise summary of our conversation above. 

The summary should:
1. Capture the main objectives and tasks discussed
2. List what has been accomplished so far
3. Note any important decisions or changes made
4. Include relevant file paths and code changes
5. Preserve any pending tasks or next steps
6. Maintain context about the current working state

Format the summary clearly with sections if needed. Focus on information that would be helpful for continuing the conversation. Be concise but don't lose important technical details.`
}

// CreateSummaryMessage creates a formatted summary message for the conversation
func CreateSummaryMessage(summary string, result *SummarizationResult) string {
	return fmt.Sprintf(`[CONVERSATION SUMMARY]

%s

---
📊 Compression Stats: %d → %d tokens (%.1fx compression, saved %d tokens)
---

The conversation history above has been summarized. All previous messages have been compressed into this summary to reduce token usage while maintaining context.`, 
		summary, 
		result.OriginalTokens, 
		result.SummaryTokens,
		result.CompressionRatio,
		result.TokensSaved)
}