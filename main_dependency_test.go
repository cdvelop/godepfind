package godepfind

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// getFileName returns the filename from a path
// Example: "theme/index.html" -> "index.html"
func getFileName(path string) (string, error) {
	if path == "" {
		return "", errors.New("GetFileName empty path")
	}

	// Check if path ends with a separator
	if len(path) > 0 && (path[len(path)-1] == '/' || path[len(path)-1] == '\\') {
		return "", errors.New("GetFileName invalid path: ends with separator")
	}

	fileName := filepath.Base(path)
	if fileName == "." || fileName == string(filepath.Separator) {
		return "", errors.New("GetFileName invalid path")
	}
	if len(path) > 0 && path[len(path)-1] == filepath.Separator {
		return "", errors.New("GetFileName invalid path")
	}

	return fileName, nil
}

func TestGoFileComesFromMain(t *testing.T) {
	// Create test directory within the project
	testDir := filepath.Join(".", "testproject")

	// Clean up any existing test directory and create fresh one
	os.RemoveAll(testDir)

	// Create test structure
	if err := createTestStructure(testDir); err != nil {
		t.Fatalf("Failed to create test structure: %v", err)
	}

	// Create GoDepFind instance
	finder := New(testDir)

	// Test cases
	testCases := []struct {
		fileName      string
		expectedMains []string
		description   string
	}{
		{
			fileName:      "module1.go",
			expectedMains: []string{"testproject/appAserver", "testproject/appBcmd"}, // module1 is imported by both appA and appB
			description:   "module1.go should be used by appAserver and appBcmd",
		},
		{
			fileName:      "module2.go",
			expectedMains: []string{"testproject/appAserver"}, // module2 is only imported by appA
			description:   "module2.go should be used only by appAserver",
		},
		{
			fileName:      "module3.go",
			expectedMains: []string{"testproject/appCwasm"}, // module3 is only imported by appC
			description:   "module3.go should be used only by appCwasm",
		},
		{
			fileName:      "module4.go",
			expectedMains: []string{}, // module4 is not imported by any main
			description:   "module4.go should not be used by any main",
		},
		{
			fileName:      "nonexistent.go",
			expectedMains: []string{}, // file doesn't exist
			description:   "nonexistent.go should return empty slice",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result, err := finder.GoFileComesFromMain(tc.fileName)
			if err != nil {
				t.Fatalf("GoFileComesFromMain failed: %v", err)
			}

			// Debug: let's see what we found
			t.Logf("Testing %s: found mains: %v", tc.fileName, result)

			// Let's also debug what packages and mains were found
			allPaths, _ := finder.listPackages("./...")
			t.Logf("All packages found: %v", allPaths)

			// Debug: Let's test getPackages directly
			packages, err := finder.getPackages(allPaths)
			if err != nil {
				t.Logf("Error in getPackages: %v", err)
			} else {
				t.Logf("Packages loaded: %d", len(packages))
				for path, pkg := range packages {
					if pkg != nil {
						t.Logf("Package %s: Name=%s, GoFiles=%v", path, pkg.Name, pkg.GoFiles)
					} else {
						t.Logf("Package %s: nil", path)
					}
				}
			}

			mainPaths, _ := finder.findMainPackages()
			t.Logf("Main packages found: %v", mainPaths)

			filePkg, _ := finder.findPackageContainingFile(tc.fileName)
			t.Logf("Package containing %s: %s", tc.fileName, filePkg)

			// Convert result to map for easier comparison
			resultMap := make(map[string]bool)
			for _, main := range result {
				resultMap[main] = true
			}

			// Check expected mains
			for _, expectedMain := range tc.expectedMains {
				if !resultMap[expectedMain] {
					t.Errorf("Expected %s to use %s, but it's not in result: %v", expectedMain, tc.fileName, result)
				}
			}

			// Check for unexpected mains
			if len(result) != len(tc.expectedMains) {
				t.Errorf("Expected %d mains, got %d. Expected: %v, Got: %v",
					len(tc.expectedMains), len(result), tc.expectedMains, result)
			}
		})
	}
}

// createTestStructure creates the test directory structure
func createTestStructure(testDir string) error {
	// Create the test directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		return err
	}

	// Create go.mod in root
	goMod := `module testproject

go 1.21
`
	if err := os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Create appAserver main (imports module1 and module2)
	if err := os.MkdirAll(filepath.Join(testDir, "appAserver"), 0755); err != nil {
		return err
	}
	appAMain := `package main

import (
	"testproject/modules/module1"
	"testproject/modules/module2"
)

func main() {
	module1.Function1()
	module2.Function2()
}
`
	if err := os.WriteFile(filepath.Join(testDir, "appAserver", "main.go"), []byte(appAMain), 0644); err != nil {
		return err
	}

	// Create appBcmd main (imports module1 only)
	if err := os.MkdirAll(filepath.Join(testDir, "appBcmd"), 0755); err != nil {
		return err
	}
	appBMain := `package main

import (
	"testproject/modules/module1"
)

func main() {
	module1.Function1()
}
`
	if err := os.WriteFile(filepath.Join(testDir, "appBcmd", "main.go"), []byte(appBMain), 0644); err != nil {
		return err
	}

	// Create appCwasm main (imports module3 only)
	if err := os.MkdirAll(filepath.Join(testDir, "appCwasm"), 0755); err != nil {
		return err
	}
	appCMain := `package main

import (
	"testproject/modules/module3"
)

func main() {
	module3.Function3()
}
`
	if err := os.WriteFile(filepath.Join(testDir, "appCwasm", "main.go"), []byte(appCMain), 0644); err != nil {
		return err
	}

	// Create modules directory structure
	modulesDir := filepath.Join(testDir, "modules")

	// Create module1 (used by appAserver and appBcmd)
	module1Dir := filepath.Join(modulesDir, "module1")
	if err := os.MkdirAll(module1Dir, 0755); err != nil {
		return err
	}
	module1Code := `package module1

func Function1() {
	// Basic exported function
}
`
	if err := os.WriteFile(filepath.Join(module1Dir, "module1.go"), []byte(module1Code), 0644); err != nil {
		return err
	}

	// Create module2 (used only by appAserver)
	module2Dir := filepath.Join(modulesDir, "module2")
	if err := os.MkdirAll(module2Dir, 0755); err != nil {
		return err
	}
	module2Code := `package module2

func Function2() {
	// Basic exported function
}
`
	if err := os.WriteFile(filepath.Join(module2Dir, "module2.go"), []byte(module2Code), 0644); err != nil {
		return err
	}

	// Create module3 (used only by appCwasm)
	module3Dir := filepath.Join(modulesDir, "module3")
	if err := os.MkdirAll(module3Dir, 0755); err != nil {
		return err
	}
	module3Code := `package module3

func Function3() {
	// Basic exported function
}
`
	if err := os.WriteFile(filepath.Join(module3Dir, "module3.go"), []byte(module3Code), 0644); err != nil {
		return err
	}

	// Create module4 (not used by any main)
	module4Dir := filepath.Join(modulesDir, "module4")
	if err := os.MkdirAll(module4Dir, 0755); err != nil {
		return err
	}
	module4Code := `package module4

func Function4() {
	// Basic exported function - not used by any main
}
`
	if err := os.WriteFile(filepath.Join(module4Dir, "module4.go"), []byte(module4Code), 0644); err != nil {
		return err
	}

	return nil
}

func TestGetFileName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"theme/index.html", "index.html", false},
		{"module3.go", "module3.go", false},
		{"/path/to/file.go", "file.go", false},
		{"", "", true},
		{"path/", "", true},
		{"path\\", "", true},
	}

	for _, tc := range testCases {
		result, err := getFileName(tc.input)
		if tc.hasError {
			if err == nil {
				t.Errorf("Expected error for input %q, but got none", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tc.input, err)
			}
			if result != tc.expected {
				t.Errorf("For input %q, expected %q, got %q", tc.input, tc.expected, result)
			}
		}
	}
}
