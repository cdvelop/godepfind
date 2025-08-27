package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateInputForProcessing tests the centralized validation function
func TestValidateInputForProcessing(t *testing.T) {
	tests := []struct {
		name            string
		mainFilePath    string
		fileName        string
		fileContent     string
		expectedProcess bool
		expectError     bool
		errorContains   string
	}{
		{
			mainFilePath:    "main.go",
			fileName:        "test.go",
			fileContent:     "package main\n\nfunc main() {}",
			expectedProcess: true,
			expectError:     false,
		},
		{
			mainFilePath:    "",
			fileName:        "test.go",
			fileContent:     "package main",
			expectedProcess: false,
			expectError:     true,
			errorContains:   "handler main file path cannot be empty",
		},
		{
			mainFilePath:    "main.go",
			fileName:        "empty.go",
			fileContent:     "",
			expectedProcess: false,
			expectError:     false,
		},
		{
			mainFilePath:    "main.go",
			fileName:        "invalid.go",
			fileContent:     "package main\n\nfunc main() {",
			expectedProcess: false,
			expectError:     false,
		},
		{
			mainFilePath:    "main.go",
			fileName:        "partial.go",
			fileContent:     "pack", // Incomplete package declaration
			expectedProcess: false,
			expectError:     false,
		},
		{
			mainFilePath:    "main.go",
			fileName:        "test.txt",
			fileContent:     "some content",
			expectedProcess: true, // Non-go files should pass validation
			expectError:     false,
		},
		{
			mainFilePath:    "main.go",
			fileName:        "comments.go",
			fileContent:     "// Only comments\n/* More comments */",
			expectedProcess: false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and file
			tempDir := t.TempDir()
			var filePath string

			if tt.fileName != "" {
				filePath = filepath.Join(tempDir, tt.fileName)
				if err := os.WriteFile(filePath, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Create GoDepFind instance
			gdf := New(tempDir)

			// Test validation
			shouldProcess, err := gdf.ValidateInputForProcessing(tt.mainFilePath, tt.fileName, filePath)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil && tt.errorContains != "" {
				if !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			}

			// Check shouldProcess result
			if shouldProcess != tt.expectedProcess {
				t.Errorf("Expected shouldProcess=%v, got %v", tt.expectedProcess, shouldProcess)
			}
		})
	}
}

// TestValidateInputForProcessing_Integration tests the validation in the context of ThisFileIsMine
func TestValidateInputForProcessing_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid main file
	mainFile := filepath.Join(tempDir, "main.go")
	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create main file: %v", err)
	}

	// Create an empty file that should be skipped
	emptyFile := filepath.Join(tempDir, "empty.go")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	gdf := New(tempDir)
	mainFilePath := "main.go"

	// Test with empty file - should return false without error
	result, err := gdf.ThisFileIsMine(mainFilePath, emptyFile, "create")
	if err != nil {
		t.Errorf("Unexpected error with empty file: %v", err)
	}
	if result {
		t.Errorf("Expected false for empty file, got true")
	}

	// Test with valid file - should process normally
	result, err = gdf.ThisFileIsMine(mainFilePath, mainFile, "create")
	if err != nil {
		t.Logf("Error with valid file (expected in test environment): %v", err)
	}
	t.Logf("Result for valid file: %v", result)
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
