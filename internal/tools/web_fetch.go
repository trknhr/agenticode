package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

const (
	cacheTimeout = 15 * time.Minute
	maxContentSize = 100 * 1024 // 100KB limit for content
	userAgent = "AgentiCode/1.0"
)

// LLMProcessor interface to avoid circular dependency
type LLMProcessor interface {
	ProcessContent(ctx context.Context, content, prompt string) (string, error)
}

type WebFetchTool struct {
	cache      map[string]cacheEntry
	cacheMutex sync.RWMutex
	llmClient  LLMProcessor
}

type cacheEntry struct {
	content   string
	timestamp time.Time
}

func NewWebFetchTool(llmClient interface{}) *WebFetchTool {
	tool := &WebFetchTool{
		cache:     make(map[string]cacheEntry),
	}
	
	// Type assert the llmClient
	if client, ok := llmClient.(LLMProcessor); ok {
		tool.llmClient = client
	}
	
	// Start cache cleanup goroutine
	go tool.cleanupCache()
	
	return tool
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetch and analyze web content using AI"
}

func (t *WebFetchTool) ReadOnly() bool {
	return true
}

func (t *WebFetchTool) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to fetch content from",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "The prompt to run on the fetched content",
			},
		},
		"required": []string{"url", "prompt"},
	}
}

func (t *WebFetchTool) Execute(args map[string]interface{}) (*ToolResult, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required and must be a string")
	}

	prompt, ok := args["prompt"].(string)
	if !ok {
		return nil, fmt.Errorf("prompt is required and must be a string")
	}

	// Validate and clean URL
	cleanedURL, err := t.validateURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Check cache first
	content, cached := t.getFromCache(cleanedURL)
	if !cached {
		// Fetch content
		content, err = t.fetchContent(cleanedURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch content: %w", err)
		}
		
		// Cache the content
		t.addToCache(cleanedURL, content)
	}

	// Process with LLM
	result, err := t.processWithLLM(content, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to process content: %w", err)
	}

	// Prepare return display
	displayMsg := fmt.Sprintf("🌐 **Fetched and analyzed**: `%s`\n\n", cleanedURL)
	if cached {
		displayMsg += "*(Using cached content)*\n\n"
	}
	displayMsg += "**Analysis:**\n" + result

	return &ToolResult{
		LLMContent:    fmt.Sprintf("Web content analysis for %s:\n%s", cleanedURL, result),
		ReturnDisplay: displayMsg,
		Error:         nil,
	}, nil
}

func (t *WebFetchTool) validateURL(urlStr string) (string, error) {
	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// If no scheme, try to parse as //host format
	if u.Scheme == "" && u.Host == "" {
		// Try parsing with // prefix
		u, err = url.Parse("//" + urlStr)
		if err != nil {
			return "", err
		}
		u.Scheme = "https"
	}

	// Ensure scheme is present
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	// Allow HTTP for local/test servers (127.0.0.1, localhost)
	if u.Scheme == "http" && !strings.Contains(u.Host, "127.0.0.1") && !strings.Contains(u.Host, "localhost") {
		u.Scheme = "https"
	}

	// Validate scheme
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("only HTTP/HTTPS URLs are supported")
	}

	// Ensure host is present
	if u.Host == "" {
		return "", fmt.Errorf("URL must have a valid host")
	}

	return u.String(), nil
}

func (t *WebFetchTool) fetchContent(url string) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, maxContentSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	// Convert to string
	htmlContent := string(body)

	// Convert HTML to markdown
	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		// If conversion fails, try to extract text content
		markdown = t.extractTextContent(htmlContent)
	}

	// Trim and clean up
	markdown = strings.TrimSpace(markdown)
	
	// If content is still too large, summarize it
	if len(markdown) > maxContentSize {
		markdown = markdown[:maxContentSize] + "\n\n[Content truncated due to size limits]"
	}

	return markdown, nil
}

func (t *WebFetchTool) extractTextContent(html string) string {
	// Simple text extraction as fallback
	// Remove script and style tags
	html = removeHTMLTags(html, "script")
	html = removeHTMLTags(html, "style")
	
	// Remove all HTML tags
	html = stripHTMLTags(html)
	
	// Clean up whitespace
	lines := strings.Split(html, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	
	return strings.Join(cleanLines, "\n")
}

func removeHTMLTags(html, tag string) string {
	start := fmt.Sprintf("<%s", tag)
	end := fmt.Sprintf("</%s>", tag)
	
	for {
		startIdx := strings.Index(strings.ToLower(html), start)
		if startIdx == -1 {
			break
		}
		
		endIdx := strings.Index(strings.ToLower(html[startIdx:]), end)
		if endIdx == -1 {
			break
		}
		
		endIdx += startIdx + len(end)
		html = html[:startIdx] + html[endIdx:]
	}
	
	return html
}

func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
			result.WriteRune(' ')
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

func (t *WebFetchTool) processWithLLM(content, prompt string) (string, error) {
	if t.llmClient == nil {
		return "", fmt.Errorf("LLM client not configured")
	}

	// Call LLM processor
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return t.llmClient.ProcessContent(ctx, content, prompt)
}

func (t *WebFetchTool) getFromCache(url string) (string, bool) {
	t.cacheMutex.RLock()
	defer t.cacheMutex.RUnlock()

	entry, exists := t.cache[url]
	if !exists {
		return "", false
	}

	// Check if cache is still valid
	if time.Since(entry.timestamp) > cacheTimeout {
		return "", false
	}

	return entry.content, true
}

func (t *WebFetchTool) addToCache(url, content string) {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()

	t.cache[url] = cacheEntry{
		content:   content,
		timestamp: time.Now(),
	}
}

func (t *WebFetchTool) cleanupCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.cacheMutex.Lock()
		now := time.Now()
		for url, entry := range t.cache {
			if now.Sub(entry.timestamp) > cacheTimeout {
				delete(t.cache, url)
			}
		}
		t.cacheMutex.Unlock()
	}
}