package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoFileValidator_IsValidGoFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		filename    string
		expected    bool
		expectError bool
	}{
		{
			name:     "valid go file",
			content:  "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
			filename: "valid.go",
			expected: true,
		},
		{
			name:     "empty file",
			content:  "",
			filename: "empty.go",
			expected: false,
		},
		{
			name:     "only package declaration",
			content:  "package main",
			filename: "minimal.go",
			expected: true,
		},
		{
			name:     "invalid syntax - missing closing brace",
			content:  "package main\n\nfunc main() {\n\tprintln(\"Hello\")",
			filename: "invalid.go",
			expected: false,
		},
		{
			name:     "incomplete package declaration",
			content:  "pack",
			filename: "partial.go",
			expected: false,
		},
		{
			name:     "non-go file",
			content:  "some content",
			filename: "file.txt",
			expected: false,
		},
		{
			name:     "go file with comments only",
			content:  "// This is a comment\n/* Another comment */",
			filename: "comments.go",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, tt.filename)

			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			validator := NewGoFileValidator()
			result, err := validator.IsValidGoFile(filePath)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGoFileValidator_HasMinimumGoContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "has package declaration",
			content:  "package main",
			expected: true,
		},
		{
			name:     "package with comments before",
			content:  "// Comment\npackage main",
			expected: true,
		},
		{
			name:     "empty file",
			content:  "",
			expected: false,
		},
		{
			name:     "only comments",
			content:  "// Just comments\n/* More comments */",
			expected: false,
		},
		{
			name:     "incomplete package",
			content:  "pack",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "test.go")

			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			validator := NewGoFileValidator()
			result, err := validator.HasMinimumGoContent(filePath)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGoFileValidator_IsFileBeingWritten(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "complete valid file",
			content:  "package main\n\nfunc main() {}",
			expected: false,
		},
		{
			name:     "empty file",
			content:  "",
			expected: false,
		},
		{
			name:     "partial content - looks like being written",
			content:  "pack",
			expected: true,
		},
		{
			name:     "invalid syntax but has package",
			content:  "package main\n\nfunc main() {",
			expected: false, // Has package declaration, so not considered "being written"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "test.go")

			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			validator := NewGoFileValidator()
			result, err := validator.IsFileBeingWritten(filePath)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for content: %q", tt.expected, result, tt.content)
			}
		})
	}
}
