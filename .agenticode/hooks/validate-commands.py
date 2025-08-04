#!/usr/bin/env python3
"""
Example hook: Validate shell commands before execution
Place in .agenticode/hooks/validate-commands.py
"""

import json
import re
import sys

# Define validation rules
DANGEROUS_PATTERNS = [
    (r"\brm\s+-rf\s+/", "Attempting to delete root directory"),
    (r"\brm\s+-rf\s+~", "Attempting to delete home directory"),
    (r"\bsudo\s+rm", "Using sudo with rm command"),
    (r"\bchmod\s+777", "Setting overly permissive file permissions"),
    (r"curl.*\|\s*sh", "Piping curl output directly to shell"),
    (r"wget.*\|\s*sh", "Piping wget output directly to shell"),
    (r"\beval\s+", "Using eval command"),
    (r">\s*/dev/sd[a-z]", "Writing directly to disk device"),
]

# Patterns that should use better alternatives
IMPROVEMENT_SUGGESTIONS = [
    (r"\bgrep\b(?!.*\|)", "Use 'rg' (ripgrep) instead of 'grep' for better performance"),
    (r"\bfind\s+.*-name", "Use 'rg --files -g pattern' instead of 'find -name'"),
    (r"\bcat\s+.*\|\s*grep", "Use 'rg pattern file' instead of 'cat file | grep'"),
]

def validate_command(command):
    """Validate a shell command for security and best practices"""
    issues = []
    
    # Check for dangerous patterns
    for pattern, message in DANGEROUS_PATTERNS:
        if re.search(pattern, command, re.IGNORECASE):
            return False, f"Security violation: {message}"
    
    # Check for improvement suggestions
    for pattern, suggestion in IMPROVEMENT_SUGGESTIONS:
        if re.search(pattern, command):
            issues.append(suggestion)
    
    return True, issues

def main():
    try:
        # Read input from stdin
        input_data = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        print(f"Error parsing JSON input: {e}", file=sys.stderr)
        sys.exit(1)
    
    # Only process run_shell commands
    if input_data.get("tool_name") != "run_shell":
        sys.exit(0)
    
    command = input_data.get("tool_input", {}).get("command", "")
    if not command:
        sys.exit(0)
    
    # Validate the command
    is_safe, issues = validate_command(command)
    
    if not is_safe:
        # Block execution with reason
        print(issues, file=sys.stderr)
        sys.exit(2)
    
    if issues:
        # Provide suggestions but don't block
        output = {
            "decision": "allow",
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "allow",
                "permissionDecisionReason": "Command allowed with suggestions"
            }
        }
        print(json.dumps(output))
        
        # Log suggestions
        for suggestion in issues:
            print(f"💡 Suggestion: {suggestion}", file=sys.stderr)
    
    sys.exit(0)

if __name__ == "__main__":
    main()