#!/bin/bash

# Test the multi-agent system
echo "Testing multi-agent system..."

# Create a test prompt that uses the agent tool
./agenticode -p "Use the agent tool to search for all Go files in the internal/tools directory and list their names. The agent should use description: 'Find Go files' and prompt: 'List all .go files in the internal/tools directory'" \
  --max-turns 5 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml