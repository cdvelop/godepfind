package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

// Test that when fileName equals handler.MainFilePath (specific identifier in a main package)
// ThisFileIsMine should return true for the handler that owns that specific main package.
// This follows the new path-based disambiguation approach per CACHE_REFACTOR_PLAN.md
func TestMainFileNameEqualsHandlerMainFilePath(t *testing.T) {
	finder := New("testproject")

	// Use specific handler that targets appAserver package
	handler := &MockHandler{
		name:         "serverHandler",
		mainFilePath: "appAserver", // Specific identifier, not generic "main.go"
	}

	// Use the main.go from testproject/appAserver
	filePath := filepath.Join("testproject", "appAserver", "main.go")

	// If the file doesn't exist in the test environment, skip the assertion
	if _, err := os.Stat(filePath); err != nil {
		t.Skipf("Skipping test: cannot access %s: %v", filePath, err)
		return
	}

	isMine, err := finder.ThisFileIsMine(handler, "main.go", filePath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine returned unexpected error: %v", err)
	}

	// Expect true because appAserver is a main package and handler specifically targets "appAserver"
	if !isMine {
		t.Errorf("Expected ThisFileIsMine to return true when handler specifically targets this package, got false")
	}
}
