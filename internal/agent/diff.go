package agent

import (
	"fmt"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffGenerator generates diffs for file changes
type DiffGenerator struct {
	dmp *diffmatchpatch.DiffMatchPatch
}

// NewDiffGenerator creates a new diff generator
func NewDiffGenerator() *DiffGenerator {
	return &DiffGenerator{
		dmp: diffmatchpatch.New(),
	}
}

// GenerateUnifiedDiff generates a unified diff between two strings
func (d *DiffGenerator) GenerateUnifiedDiff(original, new, fileName string) string {
	diffs := d.dmp.DiffMain(original, new, false)
	d.dmp.DiffCleanupSemantic(diffs)
	
	if len(diffs) == 0 || (len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual) {
		return "No changes"
	}
	
	var result strings.Builder
	
	// Add file header
	result.WriteString(fmt.Sprintf("--- %s\n", fileName))
	result.WriteString(fmt.Sprintf("+++ %s\n", fileName))
	
	// Simple unified diff implementation
	patches := d.dmp.PatchMake(original, diffs)
	
	for _, patch := range patches {
		result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 
			patch.Start1+1, patch.Length1,
			patch.Start2+1, patch.Length2))
		
		// Extract text properly
		patchText := patch.String()
		// Remove the header line from patch text
		lines := strings.Split(patchText, "\n")
		for i, line := range lines {
			if i == 0 && strings.HasPrefix(line, "@") {
				continue
			}
			if line == "" && i == len(lines)-1 {
				continue
			}
			// Unescape the line
			line = strings.ReplaceAll(line, "%0A", "\n")
			result.WriteString(line + "\n")
		}
	}
	
	return result.String()
}

// GenerateColoredDiff generates a colored diff for terminal display
func (d *DiffGenerator) GenerateColoredDiff(original, new, fileName string) string {
	diffs := d.dmp.DiffMain(original, new, false)
	d.dmp.DiffCleanupSemantic(diffs)
	
	var result strings.Builder
	var addedLines, removedLines int
	
	// First pass: count changes
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			addedLines += strings.Count(diff.Text, "\n")
			if !strings.HasSuffix(diff.Text, "\n") && diff.Text != "" {
				addedLines++
			}
		case diffmatchpatch.DiffDelete:
			removedLines += strings.Count(diff.Text, "\n")
			if !strings.HasSuffix(diff.Text, "\n") && diff.Text != "" {
				removedLines++
			}
		}
	}
	
	// Add summary
	result.WriteString(fmt.Sprintf("Changes: %s+%d lines%s, %s-%d lines%s\n\n",
		TermColors.Green, addedLines, TermColors.Reset,
		TermColors.Red, removedLines, TermColors.Reset))
	
	// Generate line-by-line diff with context
	patches := d.dmp.PatchMake(original, diffs)
	
	for i, patch := range patches {
		if i > 0 {
			result.WriteString("\n")
		}
		
		result.WriteString(fmt.Sprintf("%s@@ -%d,%d +%d,%d @@%s\n",
			TermColors.Blue,
			patch.Start1+1, patch.Length1,
			patch.Start2+1, patch.Length2,
			TermColors.Reset))
		
		// Process the patch text
		patchText := patch.String()
		lines := strings.Split(patchText, "\n")
		for i, line := range lines {
			if i == 0 && strings.HasPrefix(line, "@") {
				continue // Skip header
			}
			if line == "" && i == len(lines)-1 {
				continue
			}
			
			// Unescape the line
			line = strings.ReplaceAll(line, "%0A", "\n")
			
			switch {
			case strings.HasPrefix(line, "+"):
				result.WriteString(fmt.Sprintf("%s%s%s\n", TermColors.Green, line, TermColors.Reset))
			case strings.HasPrefix(line, "-"):
				result.WriteString(fmt.Sprintf("%s%s%s\n", TermColors.Red, line, TermColors.Reset))
			default:
				result.WriteString(line + "\n")
			}
		}
	}
	
	return result.String()
}

// GenerateInlineDiff generates a simple inline diff showing changes
func (d *DiffGenerator) GenerateInlineDiff(original, new string) string {
	if original == new {
		return "No changes"
	}
	
	diffs := d.dmp.DiffMain(original, new, false)
	d.dmp.DiffCleanupSemantic(diffs)
	
	var result strings.Builder
	
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			result.WriteString(fmt.Sprintf("%s+%s%s", TermColors.Green, diff.Text, TermColors.Reset))
		case diffmatchpatch.DiffDelete:
			result.WriteString(fmt.Sprintf("%s-%s%s", TermColors.Red, diff.Text, TermColors.Reset))
		case diffmatchpatch.DiffEqual:
			// Show some context
			text := diff.Text
			if len(text) > 40 {
				result.WriteString(text[:20] + "..." + text[len(text)-20:])
			} else {
				result.WriteString(text)
			}
		}
	}
	
	return result.String()
}