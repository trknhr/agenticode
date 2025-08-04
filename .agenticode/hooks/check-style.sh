#!/bin/bash
# Example hook: Check code style after file modifications
# Place in .agenticode/hooks/check-style.sh

# Read input from stdin
input=$(cat)

# Extract tool name and file path
tool_name=$(echo "$input" | jq -r '.tool_name')
file_path=$(echo "$input" | jq -r '.tool_input.path // .tool_input.file_path // empty')

# Only process file modifications
if [[ "$tool_name" != "write_file" && "$tool_name" != "edit" ]]; then
    exit 0
fi

# Only check Go files
if [[ ! "$file_path" =~ \.go$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$file_path" ]]; then
    exit 0
fi

# Run gofmt check
if ! gofmt -l "$file_path" | grep -q .; then
    echo "✅ Go formatting looks good for $file_path"
    exit 0
fi

# File needs formatting
echo "File $file_path needs formatting. Run: gofmt -w $file_path" >&2
exit 2  # Block and provide feedback to agent