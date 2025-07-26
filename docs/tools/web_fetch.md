# Web Fetch Tool

The `web_fetch` tool fetches content from web URLs, converts HTML to markdown, and processes it using AI to extract specific information based on your prompt.

## Features

- **URL Fetching**: Retrieves content from any HTTPS URL (HTTP URLs are automatically upgraded)
- **HTML to Markdown**: Converts HTML content to clean markdown for better readability
- **AI Processing**: Uses a language model to analyze content based on your prompt
- **Caching**: 15-minute cache for faster repeated requests to the same URL
- **Size Limits**: Automatically truncates very large content (>100KB)
- **Security**: Validates URLs and uses HTTPS by default

## Parameters

```json
{
  "url": "string (required) - The URL to fetch content from",
  "prompt": "string (required) - The prompt to run on the fetched content"
}
```

## Usage Examples

### 1. Extract Information from a Web Page

```json
{
  "url": "https://example.com/article",
  "prompt": "What are the main points discussed in this article?"
}
```

### 2. Analyze Documentation

```json
{
  "url": "https://docs.example.com/api-reference",
  "prompt": "List all the available endpoints and their HTTP methods"
}
```

### 3. Check for Specific Content

```json
{
  "url": "https://status.example.com",
  "prompt": "Are there any ongoing incidents or outages reported?"
}
```

### 4. Summarize News Articles

```json
{
  "url": "https://news.example.com/tech/latest-release",
  "prompt": "Provide a brief summary of this news article in 3 bullet points"
}
```

## Important Notes

1. **HTTPS Only**: The tool automatically upgrades HTTP URLs to HTTPS for security (except for localhost/127.0.0.1)
2. **Caching**: Content is cached for 15 minutes to reduce load on target servers
3. **Size Limits**: Very large pages (>100KB) will be truncated
4. **HTML Conversion**: Complex layouts may not convert perfectly to markdown
5. **AI Processing**: The tool requires an LLM client to be configured

## Error Handling

The tool will fail if:
- The URL is invalid or empty
- The server returns a non-200 status code
- The content cannot be fetched (network issues, timeouts)
- No LLM client is configured
- The prompt is missing

## Best Practices

1. **Specific Prompts**: Write clear, specific prompts for better results
2. **Check Cache**: The tool will indicate when cached content is used
3. **Respect Rate Limits**: Don't overwhelm servers with rapid requests
4. **URL Validation**: Ensure URLs are properly formatted before use

## Security Considerations

- URLs are validated to prevent malformed requests
- HTTP is upgraded to HTTPS automatically (except for local testing)
- Content size is limited to prevent memory issues
- The tool is read-only and doesn't modify any resources

## Integration Note

This tool requires an LLM client to be configured in the agent. If you see "LLM client not configured" errors, ensure the agent is properly initialized with an LLM client.