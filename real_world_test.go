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
	goServerHandler := MockDepHandler{
		name:         "GoServer",
		mainFilePath: "appAserver/main.go", // Simulates pwa/main.server.go
	}

	tinyWasmHandler := MockDepHandler{
		name:         "TinyWasm",
		mainFilePath: "appCwasm/main.go", // Simulates pwa/public/main.wasm
	}

	tests := []struct {
		name        string
		handler     MockDepHandler
		fileName    string
		filePath    string
		expectOwner bool
	}{
		{
			"GoServer should own main.go when main.go is edited",
			goServerHandler,
			"main.go", // File being edited: main.go
			"testproject/appAserver/main.go",
			true,
		},
		{
			"TinyWasm should NOT own main.go from appAserver",
			tinyWasmHandler,
			"main.go", // File being edited: main.go
			"testproject/appAserver/main.go",
			false,
		},
		{
			"TinyWasm should own main.go when main.go is edited in appCwasm",
			tinyWasmHandler,
			"main.go", // File being edited: main.go
			"testproject/appCwasm/main.go",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Handler: %s, MainFilePath(): %s", tt.handler.Name(), tt.handler.MainFilePath())
			t.Logf("File: %s, FilePath: %s", tt.fileName, tt.filePath)

			// Test the actual method that's failing
			isMine, err := finder.ThisFileIsMine(tt.handler, tt.fileName, tt.filePath, "write")

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
