# Multi Edit Tool

The `multi_edit` tool allows you to make multiple edits to a single file in one atomic operation. It's built on top of the Edit tool and is ideal when you need to make several changes to different parts of the same file.

## Features

- **Atomic Operations**: All edits are applied in sequence. If any edit fails, none are applied.
- **Sequential Processing**: Each edit operates on the result of the previous edit.
- **File Creation**: Can create new files by using an empty `old_string` in the first edit.
- **Flexible Replacements**: Supports both single and multiple replacements with `replace_all` option.

## Parameters

```json
{
  "file_path": "string (required) - The absolute path to the file to modify",
  "edits": [
    {
      "old_string": "string (required) - The text to replace",
      "new_string": "string (required) - The text to replace it with",
      "replace_all": "boolean (optional, default: false) - Replace all occurrences"
    }
  ]
}
```

## Usage Examples

### 1. Multiple Replacements in an Existing File

```json
{
  "file_path": "/path/to/file.txt",
  "edits": [
    {
      "old_string": "TODO",
      "new_string": "DONE",
      "replace_all": true
    },
    {
      "old_string": "function oldName",
      "new_string": "function newName"
    },
    {
      "old_string": "oldVariable",
      "new_string": "newVariable",
      "replace_all": true
    }
  ]
}
```

### 2. Creating a New File

```json
{
  "file_path": "/path/to/new_file.py",
  "edits": [
    {
      "old_string": "",
      "new_string": "#!/usr/bin/env python3\n\ndef main():\n    print('Hello, World!')\n\nif __name__ == '__main__':\n    main()"
    },
    {
      "old_string": "Hello, World!",
      "new_string": "Hello, Multi-Edit!"
    }
  ]
}
```

### 3. Refactoring Code

```json
{
  "file_path": "/src/utils.js",
  "edits": [
    {
      "old_string": "const",
      "new_string": "let",
      "replace_all": true
    },
    {
      "old_string": "getData",
      "new_string": "fetchData",
      "replace_all": true
    },
    {
      "old_string": "// Old comment",
      "new_string": "// Updated comment explaining the new implementation"
    }
  ]
}
```

## Important Notes

1. **Order Matters**: Edits are applied sequentially, so earlier edits can affect later ones.
2. **Exact Matching**: The `old_string` must match exactly, including whitespace and indentation.
3. **Validation**: The tool validates that:
   - `old_string` and `new_string` are different
   - `old_string` exists in the file (unless creating a new file)
   - `old_string` is unique when `replace_all` is false

## Error Cases

The tool will fail if:
- The file path is invalid or the file cannot be read (unless creating a new file)
- Any `old_string` is not found in the file
- Any `old_string` appears multiple times but `replace_all` is false
- Any `old_string` and `new_string` are identical
- The edits array is empty or malformed

## Best Practices

1. **Plan Your Edits**: Think through the sequence of edits to avoid conflicts.
2. **Use Context**: When replacing common strings, include surrounding context to ensure uniqueness.
3. **Test First**: For complex edits, test on a copy of the file first.
4. **Combine Related Changes**: Group related edits together in a single multi_edit operation.