package godepfind

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugMatchesHandlerFile(t *testing.T) {
	finder := New("testproject")

	// Manual replication of ThisFileIsMine logic
	mainInputFileRelativePath := "appAserver/main.go"
	fileAbsPath := "appBcmd/main.go"
	event := "write"

	// Step 1: Path resolution (from ThisFileIsMine)
	if fileAbsPath == "" {
		t.Fatal("fileAbsPath cannot be empty")
	}

	// If the caller provided a relative path, resolve it against rootDir first
	if !filepath.IsAbs(fileAbsPath) {
		fileAbsPath = filepath.Join(finder.rootDir, fileAbsPath)
	}
	fmt.Printf("fileAbsPath after joining: %q\n", fileAbsPath)

	fileName := filepath.Base(fileAbsPath)

	// Step 2: Initialize and update cache
	err := finder.ensureCacheInitialized()
	if err != nil {
		t.Fatalf("Cache initialization failed: %v", err)
	}

	err = finder.updateCacheForFileWithContext(fileAbsPath, event, mainInputFileRelativePath)
	if err != nil {
		t.Fatalf("Cache update failed: %v", err)
	}

	// Step 3: Direct file comparison
	handlerFile := mainInputFileRelativePath
	handlerFileName := filepath.Base(handlerFile)

	if fileName == handlerFileName {
		// Get the relative path from the project root
		relativeFilePath := strings.TrimPrefix(fileAbsPath, finder.rootDir+"/")
		fmt.Printf("relativeFilePath: %q, handlerFile: %q\n", relativeFilePath, handlerFile)

		if relativeFilePath == handlerFile {
			fmt.Printf("DIRECT MATCH: returning true\n")
			t.Fatalf("Direct match should not happen here!")
		}
	}

	fmt.Printf("Direct file comparison: no match\n")

	// Step 4: Package-based resolution
	var targetPkg string

	// Try exact path lookup in filePathToPackage
	relPath := strings.TrimPrefix(fileAbsPath, "/home/cesar/Dev/Pkg/Mine/godepfind/")
	fmt.Printf("Looking up relPath: %q\n", relPath)

	if pkg, exists := finder.filePathToPackage[relPath]; exists {
		targetPkg = pkg
		fmt.Printf("Found targetPkg via exact path: %q\n", targetPkg)
	} else {
		// Fallback to fileToPackages
		packages := finder.fileToPackages[fileName]
		if len(packages) > 0 {
			targetPkg = packages[0]
			fmt.Printf("Found targetPkg via fileToPackages[0]: %q\n", targetPkg)
		}
	}

	// Step 5: Check if this is a main package that matches the handler
	if finder.isMainPackage(targetPkg) && finder.matchesHandlerFile(targetPkg, handlerFile) {
		fmt.Printf("MAIN PACKAGE MATCH: isMainPackage(%q)=%v && matchesHandlerFile(%q, %q)=%v\n",
			targetPkg, finder.isMainPackage(targetPkg), targetPkg, handlerFile, finder.matchesHandlerFile(targetPkg, handlerFile))
		t.Fatalf("This should not match!")
	}

	// Step 6: Check if any main package imports this target package and matches the handler
	for _, mainPath := range finder.mainPackages {
		imports := finder.cachedMainImportsPackage(mainPath, targetPkg)
		matches := finder.matchesHandlerFile(mainPath, handlerFile)
		fmt.Printf("Loop: cachedMainImportsPackage(%q, %q)=%v && matchesHandlerFile(%q, %q)=%v\n",
			mainPath, targetPkg, imports, mainPath, handlerFile, matches)

		if imports && matches {
			fmt.Printf("IMPORT MATCH: returning true\n")
			t.Fatalf("This should not match!")
		}
	}

	fmt.Printf("All checks passed - should return false\n")
}
