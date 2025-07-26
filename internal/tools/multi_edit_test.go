package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiEditTool(t *testing.T) {
	tool := NewMultiEditTool()

	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "multi_edit_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("multiple edits on existing file", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test1.txt")
		content := `Hello World
This is a test file
We will replace multiple things
Hello again`
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Apply multiple edits
		args := map[string]interface{}{
			"file_path": testFile,
			"edits": []interface{}{
				map[string]interface{}{
					"old_string": "Hello",
					"new_string": "Hi",
					"replace_all": true,
				},
				map[string]interface{}{
					"old_string": "test file",
					"new_string": "sample document",
				},
				map[string]interface{}{
					"old_string": "We will replace",
					"new_string": "We have replaced",
				},
			},
		}

		result, err := tool.Execute(args)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Read the modified file
		modified, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}

		expected := `Hi World
This is a sample document
We have replaced multiple things
Hi again`
		if string(modified) != expected {
			t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(modified))
		}

		// Check the result
		if result.Error != nil {
			t.Errorf("Expected no error in result, got: %v", result.Error)
		}
	})

	t.Run("create new file with multi_edit", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new_file.txt")

		args := map[string]interface{}{
			"file_path": testFile,
			"edits": []interface{}{
				map[string]interface{}{
					"old_string": "",
					"new_string": "Initial content\nLine 2",
				},
				map[string]interface{}{
					"old_string": "Line 2",
					"new_string": "Modified Line 2",
				},
			},
		}

		result, err := tool.Execute(args)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Read the created file
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}

		expected := "Initial content\nModified Line 2"
		if string(content) != expected {
			t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(content))
		}

		if result.Error != nil {
			t.Errorf("Expected no error in result, got: %v", result.Error)
		}
	})

	t.Run("error on non-unique string without replace_all", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test3.txt")
		content := "duplicate duplicate"
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		args := map[string]interface{}{
			"file_path": testFile,
			"edits": []interface{}{
				map[string]interface{}{
					"old_string": "duplicate",
					"new_string": "unique",
					"replace_all": false,
				},
			},
		}

		_, err = tool.Execute(args)
		if err == nil {
			t.Error("Expected error for non-unique string without replace_all")
		}
	})

	t.Run("error on identical old and new strings", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test4.txt")
		content := "some content"
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		args := map[string]interface{}{
			"file_path": testFile,
			"edits": []interface{}{
				map[string]interface{}{
					"old_string": "content",
					"new_string": "content",
				},
			},
		}

		_, err = tool.Execute(args)
		if err == nil {
			t.Error("Expected error for identical old and new strings")
		}
	})

	t.Run("error on string not found", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test5.txt")
		content := "some content"
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		args := map[string]interface{}{
			"file_path": testFile,
			"edits": []interface{}{
				map[string]interface{}{
					"old_string": "missing",
					"new_string": "replacement",
				},
			},
		}

		_, err = tool.Execute(args)
		if err == nil {
			t.Error("Expected error for string not found")
		}
	})
}