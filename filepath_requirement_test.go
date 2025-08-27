package godepfind_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdvelop/godepfind"
)

// MockHandler for testing
type MockHandler struct {
	mainFilePath string
}

func (m *MockHandler) MainFilePath() string {
	return m.mainFilePath
}

func TestFilePathRequirement(t *testing.T) {
	// Create temporary directory structure
	tmp := t.TempDir()

	// Initialize a Go module in the temp directory
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module testproject\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create multiple directories with the same filename
	dir1 := filepath.Join(tmp, "app1")
	dir2 := filepath.Join(tmp, "app2")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Create main.go files in both directories
	mainContent := `package main

import "fmt"

func main() {
    fmt.Println("Hello World")
}
`

	file1 := filepath.Join(dir1, "main.go")
	file2 := filepath.Join(dir2, "main.go")

	if err := os.WriteFile(file1, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create godepfind instance
	depFinder := godepfind.New(tmp)

	// Create mock handler that expects app1/main.go
	handler := &MockHandler{
		mainFilePath: "app1/main.go",
	}

	t.Run("empty_filePath_should_be_rejected", func(t *testing.T) {
		// This should fail because filePath is empty
		// Without filePath, godepfind can't distinguish between app1/main.go and app2/main.go
		isMine, err := depFinder.ThisFileIsMine(handler, "", "write")

		// Should return error indicating filePath is required
		if err == nil {
			t.Fatal("Expected error when filePath is empty")
		}
		if !strings.Contains(err.Error(), "filePath cannot be empty") {
			t.Fatalf("Expected error to contain 'filePath cannot be empty', got: %v", err)
		}
		if isMine {
			t.Fatal("Expected isMine to be false when filePath is empty")
		}
	})

	t.Run("correct_filePath_should_work", func(t *testing.T) {
		// This should work because we provide the correct filePath
		isMine, err := depFinder.ThisFileIsMine(handler, "app1/main.go", "write")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !isMine {
			t.Fatal("Expected isMine to be true for correct filePath")
		}
	})

	t.Run("wrong_filePath_should_not_match", func(t *testing.T) {
		// This should not match because it's the wrong file
		isMine, err := depFinder.ThisFileIsMine(handler, "app2/main.go", "write")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if isMine {
			t.Fatal("Expected isMine to be false for wrong filePath")
		}
	})
}
