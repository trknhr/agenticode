package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InteractiveApprover implements approval through CLI interaction
type InteractiveApprover struct {
	scanner      *bufio.Scanner
	autoApprove  map[string]bool // Tool names that are auto-approved
	autoReject   map[string]bool // Tool names that are auto-rejected
	defaultAllow bool            // Default action when timeout
}

// NewInteractiveApprover creates a new interactive approver
func NewInteractiveApprover() *InteractiveApprover {
	return &InteractiveApprover{
		scanner:     bufio.NewScanner(os.Stdin),
		autoApprove: make(map[string]bool),
		autoReject:  make(map[string]bool),
	}
}

// SetAutoApprove configures tools that should be automatically approved
func (ia *InteractiveApprover) SetAutoApprove(toolNames []string) {
	for _, name := range toolNames {
		ia.autoApprove[name] = true
	}
}

// SetAutoReject configures tools that should be automatically rejected
func (ia *InteractiveApprover) SetAutoReject(toolNames []string) {
	for _, name := range toolNames {
		ia.autoReject[name] = true
	}
}

// RequestApproval prompts the user for approval
func (ia *InteractiveApprover) RequestApproval(ctx context.Context, request ApprovalRequest) (ApprovalResponse, error) {
	response := ApprovalResponse{
		RequestID:   request.RequestID,
		ApprovedIDs: []string{},
		RejectedIDs: []string{},
	}

	// Check for auto-approval/rejection
	allAutoApproved := true
	for _, call := range request.ToolCalls {
		toolName := call.ToolCall.Function.Name
		if ia.autoReject[toolName] {
			response.RejectedIDs = append(response.RejectedIDs, call.ID)
			response.Reason = fmt.Sprintf("Tool '%s' is configured for auto-rejection", toolName)
			continue
		}
		if !ia.autoApprove[toolName] {
			allAutoApproved = false
		}
	}

	// If all tools are auto-approved, approve them all
	if allAutoApproved && len(response.RejectedIDs) == 0 {
		for _, call := range request.ToolCalls {
			response.ApprovedIDs = append(response.ApprovedIDs, call.ID)
		}
		response.Approved = true
		fmt.Println("✅ Auto-approved read-only operations")
		return response, nil
	}

	// Display pending tool calls
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🔧 TOOL APPROVAL REQUEST")
	fmt.Println(strings.Repeat("─", 60))

	for i, call := range request.ToolCalls {
		if ia.autoReject[call.ToolCall.Function.Name] {
			continue // Skip rejected tools
		}

		toolName := call.ToolCall.Function.Name
		risk := request.Risks[call.ID]

		fmt.Printf("\n%d. %s %s - %s\n", i+1, GetRiskIcon(risk), toolName, GetRiskDescription(risk))

		// Check if we have confirmation details for file operations
		if request.ConfirmationDetails != nil {
			if fileDetails, ok := request.ConfirmationDetails.(*ToolFileConfirmationDetails); ok {
				fmt.Printf("   %s\n", fileDetails.Title())

				// Show file diff preview for modifications
				if !fileDetails.IsNewFile && fileDetails.FileDiff != "" {
					fmt.Println("   Preview of changes:")
					// Show first few lines of the diff
					diffLines := strings.Split(fileDetails.FileDiff, "\n")
					maxLines := 10
					for j, line := range diffLines {
						if j >= maxLines && j < len(diffLines)-3 {
							if j == maxLines {
								fmt.Printf("   ... (%d more lines) ...\n", len(diffLines)-maxLines-3)
							}
							continue
						}
						if j >= len(diffLines)-3 || j < maxLines {
							fmt.Printf("   %s\n", line)
						}
					}
				} else if fileDetails.IsNewFile {
					// For new files, show first few lines
					contentLines := strings.Split(fileDetails.NewContent, "\n")
					fmt.Printf("   New file content preview (%d lines):\n", len(contentLines))
					for j := 0; j < 5 && j < len(contentLines); j++ {
						fmt.Printf("   %s\n", contentLines[j])
					}
					if len(contentLines) > 5 {
						fmt.Printf("   ... (%d more lines) ...\n", len(contentLines)-5)
					}
				}
			} else {
				// For non-file operations, show arguments as before
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(call.ToolCall.Function.Arguments), &args); err == nil {
					fmt.Println("   Arguments:")
					for key, value := range args {
						// Format the value nicely
						valueStr := fmt.Sprintf("%v", value)
						if len(valueStr) > 100 {
							valueStr = valueStr[:97] + "..."
						}
						fmt.Printf("   - %s: %s\n", key, valueStr)
					}
				}
			}
		} else {
			// Fallback to showing arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(call.ToolCall.Function.Arguments), &args); err == nil {
				fmt.Println("   Arguments:")
				for key, value := range args {
					// Format the value nicely
					valueStr := fmt.Sprintf("%v", value)
					if len(valueStr) > 100 {
						valueStr = valueStr[:97] + "..."
					}
					fmt.Printf("   - %s: %s\n", key, valueStr)
				}
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("Options:")
	fmt.Println("  y/yes    - Approve all")
	fmt.Println("  n/no     - Reject all")
	fmt.Println("  s/select - Choose individual tools")
	fmt.Println("  i/info   - Show more details")
	fmt.Print("\nYour choice [y/n/s/i]: ")

	if !ia.scanner.Scan() {
		return response, fmt.Errorf("failed to read user input")
	}

	input := strings.ToLower(strings.TrimSpace(ia.scanner.Text()))

	switch input {
	case "y", "yes":
		for _, call := range request.ToolCalls {
			if !ia.autoReject[call.ToolCall.Function.Name] {
				response.ApprovedIDs = append(response.ApprovedIDs, call.ID)
			}
		}
		response.Approved = true
		fmt.Println("✅ All tools approved")

	case "n", "no":
		for _, call := range request.ToolCalls {
			response.RejectedIDs = append(response.RejectedIDs, call.ID)
		}
		response.Approved = false
		response.Reason = "User rejected all tool calls"
		fmt.Println("❌ All tools rejected")

	case "s", "select":
		response = ia.selectiveApproval(request)

	case "i", "info":
		ia.showDetailedInfo(request)
		// Recursive call to show the menu again
		return ia.RequestApproval(ctx, request)

	default:
		return response, fmt.Errorf("invalid choice: %s", input)
	}

	return response, nil
}

// selectiveApproval allows the user to choose individual tools
func (ia *InteractiveApprover) selectiveApproval(request ApprovalRequest) ApprovalResponse {
	response := ApprovalResponse{
		RequestID:   request.RequestID,
		ApprovedIDs: []string{},
		RejectedIDs: []string{},
	}

	fmt.Println("\nEnter the numbers of tools to approve (comma-separated), or 'all' for all, 'none' for none:")
	fmt.Print("Your selection: ")

	if !ia.scanner.Scan() {
		fmt.Println("Error reading input")
		return response
	}

	input := strings.ToLower(strings.TrimSpace(ia.scanner.Text()))

	if input == "all" {
		for _, call := range request.ToolCalls {
			if !ia.autoReject[call.ToolCall.Function.Name] {
				response.ApprovedIDs = append(response.ApprovedIDs, call.ID)
			}
		}
		response.Approved = len(response.ApprovedIDs) > 0
	} else if input == "none" {
		for _, call := range request.ToolCalls {
			response.RejectedIDs = append(response.RejectedIDs, call.ID)
		}
		response.Approved = false
	} else {
		// Parse comma-separated numbers
		selections := strings.Split(input, ",")
		selectedIndices := make(map[int]bool)

		for _, s := range selections {
			if num, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				selectedIndices[num-1] = true // Convert to 0-based index
			}
		}

		for i, call := range request.ToolCalls {
			if selectedIndices[i] {
				response.ApprovedIDs = append(response.ApprovedIDs, call.ID)
			} else {
				response.RejectedIDs = append(response.RejectedIDs, call.ID)
			}
		}
		response.Approved = len(response.ApprovedIDs) > 0
	}

	fmt.Printf("✅ Approved %d tools, ❌ Rejected %d tools\n",
		len(response.ApprovedIDs), len(response.RejectedIDs))

	return response
}

// showDetailedInfo displays detailed information about the tool calls
func (ia *InteractiveApprover) showDetailedInfo(request ApprovalRequest) {
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("DETAILED TOOL INFORMATION")
	fmt.Println(strings.Repeat("═", 60))

	for i, call := range request.ToolCalls {
		toolName := call.ToolCall.Function.Name
		risk := request.Risks[call.ID]

		fmt.Printf("\n%d. Tool: %s\n", i+1, toolName)
		fmt.Printf("   Risk Level: %s %s\n", GetRiskIcon(risk), GetRiskDescription(risk))
		fmt.Printf("   Tool Call ID: %s\n", call.ID)
		fmt.Printf("   Created At: %s\n", call.CreatedAt.Format("15:04:05"))

		// Check if we have file confirmation details
		if request.ConfirmationDetails != nil {
			if fileDetails, ok := request.ConfirmationDetails.(*ToolFileConfirmationDetails); ok {
				fmt.Printf("   %s\n", fileDetails.Title())
				fmt.Printf("   File Path: %s\n", fileDetails.FilePath)

				if fileDetails.IsNewFile {
					fmt.Println("\n   Full new file content:")
					fmt.Println(strings.Repeat("-", 50))
					fmt.Println(fileDetails.NewContent)
					fmt.Println(strings.Repeat("-", 50))
				} else {
					fmt.Println("\n   Full diff:")
					fmt.Println(strings.Repeat("-", 50))
					fmt.Println(fileDetails.FileDiff)
					fmt.Println(strings.Repeat("-", 50))
				}
			} else if execDetails, ok := request.ConfirmationDetails.(*ToolExecConfirmationDetails); ok {
				fmt.Printf("   Command: %s\n", execDetails.Command)
				fmt.Printf("   Working Directory: %s\n", execDetails.WorkingDir)
			} else {
				// For other tools, show arguments
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(call.ToolCall.Function.Arguments), &args); err == nil {
					fmt.Println("   Full Arguments:")
					for key, value := range args {
						fmt.Printf("   - %s:\n", key)
						// Pretty print the value
						if valueBytes, err := json.MarshalIndent(value, "     ", "  "); err == nil {
							fmt.Printf("     %s\n", string(valueBytes))
						} else {
							fmt.Printf("     %v\n", value)
						}
					}
				}
			}
		} else {
			// Fallback to arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(call.ToolCall.Function.Arguments), &args); err == nil {
				fmt.Println("   Full Arguments:")
				for key, value := range args {
					fmt.Printf("   - %s:\n", key)
					// Pretty print the value
					if valueBytes, err := json.MarshalIndent(value, "     ", "  "); err == nil {
						fmt.Printf("     %s\n", string(valueBytes))
					} else {
						fmt.Printf("     %v\n", value)
					}
				}
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("═", 60))
}

// NotifyExecution notifies about tool execution results
func (ia *InteractiveApprover) NotifyExecution(toolCallID string, result interface{}, err error) {
	if err != nil {
		fmt.Printf("❌ Tool execution failed (ID: %s): %v\n", toolCallID, err)
	} else {
		// Silent success - execution results will be shown by the tool itself
	}
}
