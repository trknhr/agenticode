#!/bin/bash

# Test script to demonstrate the approval system

echo "Testing AgentiCode with approval system..."
echo

# Create a test input that will trigger different risk levels
cat << 'EOF' | ./agenticode
List the files in the current directory
EOF

echo
echo "Test completed!"