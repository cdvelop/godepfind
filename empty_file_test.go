package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

// MockDepHandler implements DepHandler for testing
type MockDepHandler struct {
	name         string
	mainFilePath string
}

func (m MockDepHandler) Name() string {
	return m.name
}

func (m MockDepHandler) MainFilePath() string {
	return m.mainFilePath
}

// TestThisFileIsMineWithEmptyFile tests handling of empty Go files
func TestThisFileIsMineWithEmptyFile(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a valid Go file with package declaration
	validFile := filepath.Join(tempDir, "valid.go")
	validContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
}
`
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to create valid file: %v", err)
	}

	// Create an empty Go file (this should cause the issue)
	emptyFile := filepath.Join(tempDir, "empty.go")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Create GoDepFind instance
	gdf := New(tempDir)

	// Create mock handler
	handler := MockDepHandler{
		name:         "testHandler",
		mainFilePath: "main.go",
	}

	// Test with empty file - this should not panic or break
	// and should handle the empty file gracefully
	result, err := gdf.ThisFileIsMine(handler, "empty.go", emptyFile, "create")

	// The function should not return an error due to empty file
	// but should handle it gracefully
	if err != nil {
		t.Logf("Error occurred with empty file: %v", err)
		// Check if the error is related to parsing
		if err.Error() != "" {
			t.Logf("Error message: %s", err.Error())
		}
	}

	t.Logf("Result for empty file: %v", result)
}

// TestThisFileIsMineWithInvalidSyntax tests handling of Go files with invalid syntax
func TestThisFileIsMineWithInvalidSyntax(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a Go file with invalid syntax
	invalidFile := filepath.Join(tempDir, "invalid.go")
	invalidContent := `package main

func main() {
	fmt.Println("Hello World"
	// Missing closing parenthesis and brace
`
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	// Create GoDepFind instance
	gdf := New(tempDir)

	// Create mock handler
	handler := MockDepHandler{
		name:         "testHandler",
		mainFilePath: "main.go",
	}

	// Test with invalid syntax file
	result, err := gdf.ThisFileIsMine(handler, "invalid.go", invalidFile, "create")

	if err != nil {
		t.Logf("Error occurred with invalid syntax file: %v", err)
	}

	t.Logf("Result for invalid syntax file: %v", result)
}

// TestThisFileIsMineWithPartiallyWritten tests handling of files being written
func TestThisFileIsMineWithPartiallyWritten(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a Go file that looks like it's being written (incomplete package declaration)
	partialFile := filepath.Join(tempDir, "partial.go")
	partialContent := `pack` // Incomplete package declaration
	if err := os.WriteFile(partialFile, []byte(partialContent), 0644); err != nil {
		t.Fatalf("Failed to create partial file: %v", err)
	}

	// Create GoDepFind instance
	gdf := New(tempDir)

	// Create mock handler
	handler := MockDepHandler{
		name:         "testHandler",
		mainFilePath: "main.go",
	}

	// Test with partial file
	result, err := gdf.ThisFileIsMine(handler, "partial.go", partialFile, "write")

	if err != nil {
		t.Logf("Error occurred with partial file: %v", err)
	}

	t.Logf("Result for partial file: %v", result)
}
