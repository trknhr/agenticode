#!/bin/bash

echo "========================================="
echo "AGENTICODE HOOKS DEMONSTRATION"
echo "========================================="
echo
echo "This test demonstrates the hooks system in AgentiCode."
echo "Make sure you have configured hooks in your ~/.agenticode.yaml"
echo
echo "Testing various hook scenarios..."
echo

# Test 1: UserPromptSubmit hook (adds context)
echo "1. Testing UserPromptSubmit hook (should add context to prompt):"
echo "----------------------------------------"
./agenticode -p "Show me the current time from the context" \
  --max-turns 1 \
  --permission-mode bypassPermissions

echo
echo "2. Testing PreToolUse hook for shell commands:"
echo "----------------------------------------"
# This should trigger validation
./agenticode -p "Run this command: grep -r 'test' ." \
  --max-turns 2 \
  --permission-mode bypassPermissions

echo
echo "3. Testing file operation hooks:"
echo "----------------------------------------"
./agenticode -p "Create a test file called hook_test.go with a simple hello world program" \
  --max-turns 3 \
  --permission-mode bypassPermissions

echo
echo "4. Testing blocked command (if validation hook is active):"
echo "----------------------------------------"
./agenticode -p "Run this command: sudo rm -rf /tmp/test" \
  --max-turns 2 \
  --permission-mode bypassPermissions

echo
echo "========================================="
echo "HOOK LOGS"
echo "========================================="
echo
echo "Tool usage log:"
if [[ -f ~/.agenticode/tools.log ]]; then
    tail -n 10 ~/.agenticode/tools.log
else
    echo "(No tool log found)"
fi

echo
echo "Operations log:"
if [[ -f ~/.agenticode/operations.log ]]; then
    tail -n 10 ~/.agenticode/operations.log
else
    echo "(No operations log found)"
fi

echo
echo "========================================="
echo "END OF DEMONSTRATION"
echo "========================================="