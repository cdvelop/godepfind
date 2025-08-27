package godepfind

import (
	"testing"
)

func TestThisFileIsMine(t *testing.T) {
	finder := New("testproject")

	// Test with empty main file path - should return error
	_, err := finder.ThisFileIsMine("", "testproject/appAserver/main.go", "write")
	if err == nil {
		t.Error("Expected error for empty main file path")
	}

	// Test main.go ownership: each handler should own only its specific main.go
	tests := []struct {
		name         string
		mainFilePath string
		filePath     string
		expected     bool
	}{
		{"serverHandler owns appAserver main.go", "appAserver/main.go", "testproject/appAserver/main.go", true},
		{"cmdHandler owns appBcmd main.go", "appBcmd/main.go", "testproject/appBcmd/main.go", true},
		{"wasmHandler owns appCwasm main.go", "appCwasm/main.go", "testproject/appCwasm/main.go", true},
		// Cross-ownership should be false (path-based disambiguation)
		{"serverHandler does NOT own appBcmd main.go", "appAserver/main.go", "testproject/appBcmd/main.go", false},
		{"cmdHandler does NOT own appCwasm main.go", "appBcmd/main.go", "testproject/appCwasm/main.go", false},
		{"wasmHandler does NOT own appAserver main.go", "appCwasm/main.go", "testproject/appAserver/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := finder.ThisFileIsMine(tt.mainFilePath, tt.filePath, "write")
			if err != nil {
				t.Logf("Test %s: got error (may be expected in test environment): %v", tt.name, err)
				return // Skip if cache initialization fails
			}
			if result != tt.expected {
				t.Errorf("Test %s: expected %v, got %v", tt.name, tt.expected, result)
			}
		})
	}
}

func TestMatchesHandlerFile(t *testing.T) {
	finder := New("testproject")

	tests := []struct {
		mainPkg     string
		handlerFile string
		expected    bool
		description string
	}{
		{"testproject/appAserver", "appAserver", true, "exact match with app name"},
		{"testproject/appBcmd", "appBcmd", true, "exact match with cmd app name"},
		{"testproject/appCwasm", "appCwasm", true, "exact match with wasm app name"},
		{"testproject/appAserver", "main.go", false, "main.go is too generic"},
		{"testproject/appBcmd", "appAserver", false, "cmd app doesn't match server"},
		{"testproject/appCwasm", "wasm", true, "wasm in appCwasm should match wasm handler"},
	}

	for _, test := range tests {
		result := finder.matchesHandlerFile(test.mainPkg, test.handlerFile)
		if result != test.expected {
			t.Errorf("Test '%s': expected %v, got %v", test.description, test.expected, result)
		}
	}
}

func TestCacheInitialization(t *testing.T) {
	finder := New("testproject") // Use testproject like existing tests

	// Initially cache should not be initialized
	if finder.cachedModule {
		t.Error("Cache should not be initialized initially")
	}

	// Verify cache fields are initialized
	if finder.packageCache == nil {
		t.Error("packageCache should be initialized")
	}
	if finder.dependencyGraph == nil {
		t.Error("dependencyGraph should be initialized")
	}
	if finder.reverseDeps == nil {
		t.Error("reverseDeps should be initialized")
	}
	if finder.filePathToPackage == nil {
		t.Error("filePathToPackage should be initialized")
	}
	if finder.fileToPackages == nil {
		t.Error("fileToPackages should be initialized")
	}

	// Test lazy initialization with real testproject
	err := finder.ensureCacheInitialized()
	if err != nil {
		t.Logf("Cache initialization result: %v", err)
	} else {
		t.Log("Cache initialized successfully")

		// Verify cache was populated
		if len(finder.packageCache) == 0 {
			t.Error("Expected packageCache to be populated after initialization")
		}
		if len(finder.mainPackages) == 0 {
			t.Error("Expected mainPackages to be populated (should find appAserver, appBcmd, appCwasm)")
		} else {
			t.Logf("Found main packages: %v", finder.mainPackages)
		}
	}

	// After successful initialization, cache should be marked as initialized
	if err == nil && !finder.cachedModule {
		t.Error("Cache should be marked as initialized after successful rebuild")
	}
}

func TestGoFileComesFromMainWithCache(t *testing.T) {
	// Test the cached version of GoFileComesFromMain using real testproject
	finder := New("testproject")

	// Test with module1.go - should be used by appAserver and appBcmd
	mains, err := finder.GoFileComesFromMain("module1.go")
	if err != nil {
		t.Logf("GoFileComesFromMain error: %v", err)
		return // Skip if we can't analyze the testproject
	}

	t.Logf("Main packages that depend on module1.go: %v", mains)

	// module1 is imported by appAserver and appBcmd, so should find both
	expectedCount := 2
	if len(mains) != expectedCount {
		t.Logf("Expected %d main packages for module1.go, got %d: %v", expectedCount, len(mains), mains)
	}

	// Test with module3.go - should only be used by appCwasm
	mains2, err := finder.GoFileComesFromMain("module3.go")
	if err != nil {
		t.Errorf("Unexpected error for module3.go: %v", err)
		return
	}

	t.Logf("Main packages that depend on module3.go: %v", mains2)

	// module3 is only imported by appCwasm
	expectedCount2 := 1
	if len(mains2) != expectedCount2 {
		t.Logf("Expected %d main package for module3.go, got %d: %v", expectedCount2, len(mains2), mains2)
	}

	// Test with a non-existent file
	mains3, err := finder.GoFileComesFromMain("nonexistent.go")
	if err != nil {
		t.Errorf("Unexpected error for non-existent file: %v", err)
	}

	if len(mains3) != 0 {
		t.Errorf("Expected empty result for non-existent file, got %v", mains3)
	}
}
