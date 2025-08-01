#!/bin/bash

echo "========================================"
echo "SUB-AGENT EXECUTION DEMONSTRATION"
echo "========================================"
echo
echo "This test will demonstrate that sub-agents are actually running"
echo "by showing detailed logs of their execution."
echo
echo "Watch for:"
echo "  - [SA-XXXX] prefixed logs showing sub-agent activity"
echo "  - Tool calls made by sub-agents"
echo "  - Execution summaries"
echo
echo "----------------------------------------"
echo

# Simple test that will clearly show sub-agent execution
./agenticode -p 'Use the agent tool with the following parameters:
- description: "Count Go files"
- prompt: "Count the number of .go files in internal/tools directory. First list the directory, then report the count."
- agent_type: "searcher"

After the sub-agent completes, summarize what it found.' \
  --max-turns 5 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml 2>&1 

echo
echo "========================================"
echo "END OF DEMONSTRATION"
echo "========================================"
echo
echo "If you saw logs with [SA-XXXX] prefixes, those were from the sub-agent!"
echo "The sub-agent executed independently and returned results to the parent."