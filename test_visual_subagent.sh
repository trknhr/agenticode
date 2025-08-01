#!/bin/bash

echo "🎯 VISUAL SUB-AGENT DEMONSTRATION"
echo "================================="
echo

# Function to highlight sub-agent logs
highlight_logs() {
    while IFS= read -r line; do
        if [[ $line == *"[SA-"* ]]; then
            # Sub-agent logs in blue
            echo -e "\033[34m$line\033[0m"
        elif [[ $line == *"Starting turn"* ]] && [[ $line != *"[SA-"* ]]; then
            # Parent agent turns in green
            echo -e "\033[32m[PARENT] $line\033[0m"
        elif [[ $line == *"Tool call"* ]] && [[ $line != *"[SA-"* ]]; then
            # Parent tool calls in yellow
            echo -e "\033[33m[PARENT] $line\033[0m"
        else
            echo "$line"
        fi
    done
}

echo "Running agenticode with sub-agent..."
echo "Parent logs = GREEN/YELLOW, Sub-agent logs = BLUE"
echo "-------------------------------------------------"
echo

./agenticode -p 'Use the agent tool to analyze the agent.go file with these parameters:
- description: "Analyze agent.go"
- prompt: "Read the first 50 lines of internal/tools/agent.go and tell me what functions are defined there."
- agent_type: "analyzer"' \
  --max-turns 5 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml 2>&1 | highlight_logs

echo
echo "================================="
echo "Legend:"
echo -e "\033[32m[PARENT]\033[0m = Parent agent activity"
echo -e "\033[34m[SA-XXXX]\033[0m = Sub-agent activity"
echo "================================="