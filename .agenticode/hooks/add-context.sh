#!/bin/bash
# Example hook: Add context to user prompts
# Place in .agenticode/hooks/add-context.sh
# Configure for UserPromptSubmit event

# Add useful context
echo "=== Additional Context ==="
echo "Current time: $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo "Working directory: $(pwd)"
echo "Git branch: $(git branch --show-current 2>/dev/null || echo 'not in git repo')"
echo "Go version: $(go version 2>/dev/null | cut -d' ' -f3 || echo 'Go not found')"

# Check for any uncommitted changes
if git diff --quiet 2>/dev/null; then
    echo "Git status: No uncommitted changes"
else
    echo "Git status: Uncommitted changes present"
fi

exit 0