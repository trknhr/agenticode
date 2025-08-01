#!/bin/bash

echo "Testing Multi-Agent System with Different Agent Types"
echo "====================================================="
echo

# Test 1: Searcher agent
echo "Test 1: Using a searcher agent to find Go files"
echo "------------------------------------------------"
./agenticode -p 'Use the agent tool to search for Go files. Use:
{
  "tool": "agent",
  "args": {
    "description": "Find Go files",
    "prompt": "Search for all .go files in the internal/tools directory and list their names",
    "agent_type": "searcher"
  }
}' \
  --max-turns 5 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml

echo
echo "Test 2: Using an analyzer agent to analyze code structure"
echo "---------------------------------------------------------"
./agenticode -p 'Use the agent tool to analyze the agent.go file structure. Use:
{
  "tool": "agent", 
  "args": {
    "description": "Analyze agent.go",
    "prompt": "Analyze the structure and functions in internal/tools/agent.go file",
    "agent_type": "analyzer"
  }
}' \
  --max-turns 5 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml

echo
echo "Test 3: Multiple agents working together"
echo "----------------------------------------"
./agenticode -p 'Use multiple agent tools to:
1. First, use a searcher agent to find all test files
2. Then, use an analyzer agent to analyze one of them

Make two agent tool calls.' \
  --max-turns 10 \
  --permission-mode bypassPermissions \
  --config ~/.agenticode.yaml