#!/bin/bash

echo "Testing AgentiCode Debug Mode"
echo "============================="
echo ""
echo "This will run agenticode in debug mode, pausing before each LLM call."
echo "You can use this to see the conversation history and control execution."
echo ""
echo "Running: agenticode --debug"
echo ""

# Run agenticode in debug mode
go run main.go --debug