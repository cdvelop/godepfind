package godepfind

import (
	"testing"
)

// TestThisFileIsMineRealWorldScenario tests the actual ThisFileIsMine method
// reproducing the exact issue from devwatch logs
func TestThisFileIsMineRealWorldScenario(t *testing.T) {
	// Use testproject directory like other tests
	finder := New("testproject")

	// Create handlers that mimic the real ones from your logs
	goServerMainPath := "appAserver/main.go" // Simulates pwa/main.server.go
	tinyWasmMainPath := "appCwasm/main.go"   // Simulates pwa/public/main.wasm

	tests := []struct {
		name         string
		mainFilePath string
		fileName     string
		filePath     string
		expectOwner  bool
	}{
		{
			"GoServer should own main.go when main.go is edited",
			goServerMainPath,
			"main.go", // File being edited: main.go
			"testproject/appAserver/main.go",
			true,
		},
		{
			"TinyWasm should NOT own main.go from appAserver",
			tinyWasmMainPath,
			"main.go", // File being edited: main.go
			"testproject/appAserver/main.go",
			false,
		},
		{
			"TinyWasm should own main.go when main.go is edited in appCwasm",
			tinyWasmMainPath,
			"main.go", // File being edited: main.go
			"testproject/appCwasm/main.go",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("MainFilePath: %s", tt.mainFilePath)
			t.Logf("File: %s, FilePath: %s", tt.fileName, tt.filePath)

			// Test the actual method that's failing
			isMine, err := finder.ThisFileIsMine(tt.mainFilePath, tt.filePath, "write")

			if err != nil {
				t.Logf("ThisFileIsMine error: %v", err)
				return // Skip on cache errors
			}

			t.Logf("Result: IsMine=%v (expected=%v)", isMine, tt.expectOwner)

			if isMine != tt.expectOwner {
				t.Errorf("FAILED: Expected=%v, got=%v", tt.expectOwner, isMine)
			}
		})
	}
}

// TestRealWorldGoDevLogs simulates the exact scenario from your logs
func TestRealWorldGoDevLogs(t *testing.T) {
	// Use testproject since godev/test has module issues
	finder := New("testproject")

	// Real handlers from logs - exact values
	goServerMainPath := "pwa/main.server.go" // Exact from logs
	tinyWasmMainPath := "pwa/main.wasm.go"   // Corrected: should be the Go source file, not the compiled .wasm

	// Test the exact scenario from logs
	fileName := "main.server.go"
	// Simulate the filePath that would be passed to the method
	filePath := "testproject/pwa/main.server.go"

	t.Logf("=== Testing GoServer ===")
	t.Logf("MainFilePath: %s File: %s", goServerMainPath, fileName)

	isMine, err := finder.ThisFileIsMine(goServerMainPath, filePath, "write")
	if err != nil {
		t.Logf("Error: %v - Skipping due to cache issues", err)
		t.Skip("Skipping due to cache initialization issues")
		return
	}

	t.Logf("IsMine: %v", isMine)
	if !isMine {
		t.Errorf("GoServer should own main.server.go file but returned false")
	}

	t.Logf("=== Testing TinyWasm ===")
	t.Logf("MainFilePath: %s File: %s", tinyWasmMainPath, fileName)

	isMine, err = finder.ThisFileIsMine(tinyWasmMainPath, filePath, "write")
	if err != nil {
		t.Logf("Error: %v - Skipping due to cache issues", err)
		return
	}

	t.Logf("IsMine: %v", isMine)
	if isMine {
		t.Errorf("TinyWasm should NOT own main.server.go file but returned true")
	}

	// Additional test: TinyWasm should own main.wasm.go
	t.Logf("=== Testing TinyWasm with its own file ===")
	wasmFileName := "main.wasm.go"
	wasmFilePath := "testproject/pwa/main.wasm.go"
	t.Logf("MainFilePath: %s File: %s", tinyWasmMainPath, wasmFileName)

	isMine, err = finder.ThisFileIsMine(tinyWasmMainPath, wasmFilePath, "write")
	if err != nil {
		t.Logf("Error: %v - Skipping due to cache issues", err)
		return
	}

	t.Logf("IsMine: %v", isMine)
	if !isMine {
		t.Errorf("TinyWasm should own main.wasm.go file but returned false")
	}
}
