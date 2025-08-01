#!/bin/bash

echo "========================================"
echo "SIMPLE AGENT TOOL TEST"
echo "========================================"
echo

echo "Testing with a very simple prompt that should trigger agent tool..."
echo

# Run without grep filtering to see full output
./agenticode -p 'I need you to use the "agent" tool. Set the description to "Test task", the prompt to "Say hello from the sub-agent", and agent_type to "general-purpose". This will test if the agent tool is working.' \
  --max-turns 3 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml

echo
echo "========================================"
echo "If you see [SA-XXXX] logs above, the agent tool is working!"
echo "========================================"