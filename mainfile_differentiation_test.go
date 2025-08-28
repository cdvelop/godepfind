package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

// Test that main.go files in different app folders are correctly attributed
// to handlers that identify their target by app/package name (e.g. "appAserver").
// This mirrors the table-driven style used in TestValidateInputForProcessing.
func TestMainFileDifferentiationAcrossApps(t *testing.T) {
	finder := New("testproject")

	serverMain := "appAserver/main.go"
	cmdMain := "appBcmd/main.go"
	wasmMain := "appCwasm/main.go"

	tests := []struct {
		name                      string
		mainInputFileRelativePath string
		fileName                  string
		filePath                  string
		expectOwner               bool
	}{
		{"appA main owned by serverHandler", serverMain, "main.go", filepath.Join("appAserver", "main.go"), true},
		{"appA main not owned by cmdHandler", cmdMain, "main.go", filepath.Join("appAserver", "main.go"), false},
		{"appB main owned by cmdHandler", cmdMain, "main.go", filepath.Join("appBcmd", "main.go"), true},
		{"appB main not owned by wasmHandler", wasmMain, "main.go", filepath.Join("appBcmd", "main.go"), false},
		{"appC main owned by wasmHandler", wasmMain, "main.go", filepath.Join("appCwasm", "main.go"), true},
		{"module1 not owned by wasmHandler", wasmMain, "module1.go", filepath.Join("modules", "module1", "module1.go"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if file missing in this environment
			if _, err := os.Stat(tt.filePath); err != nil {
				t.Skipf("Skipping test: cannot access %s: %v", tt.filePath, err)
				return
			}

			// Ensure cache and collect diagnostics
			if err := finder.ensureCacheInitialized(); err != nil {
				t.Fatalf("Failed to initialize cache: %v", err)
			}
			resolvedPkg, _ := finder.findPackageContainingFileByPath(tt.filePath)
			mainsForFile, _ := finder.GoFileComesFromMain(tt.fileName)
			t.Logf("Diagnostics: resolvedPkg=%q, fileToPackages[\"%s\"]=%v, mainPackages=%v, mainsForFile=%v", resolvedPkg, tt.fileName, finder.fileToPackages[tt.fileName], finder.mainPackages, mainsForFile)

			isMine, err := finder.ThisFileIsMine(tt.mainInputFileRelativePath, tt.filePath, "write")
			if err != nil {
				t.Fatalf("ThisFileIsMine returned unexpected error: %v", err)
			}
			if isMine != tt.expectOwner {
				// Gather diagnostics
				pkg, _ := finder.findPackageContainingFileByPath(tt.filePath)
				var matched []string
				for _, mp := range finder.mainPackages {
					if finder.cachedMainImportsPackage(mp, pkg) && finder.matchesHandlerFile(mp, tt.mainInputFileRelativePath) {
						matched = append(matched, mp)
					}
				}
				t.Errorf("Expected ownership=%v, got %v; resolved pkg=%q; matching mains=%v", tt.expectOwner, isMine, pkg, matched)
			}
		})
	}
}
