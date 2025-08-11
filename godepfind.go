package godepfind

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GoDepFind struct {
	rootDir     string
	testImports bool

	// Cache fields
	cachedModule    bool
	packageCache    map[string]*build.Package
	dependencyGraph map[string][]string // pkg -> dependencies
	reverseDeps     map[string][]string // pkg -> reverse dependencies
	fileToPackage   map[string]string   // filename -> package path
	mainPackages    []string
}

// New creates a new GoDepFind instance with the specified root directory
func New(rootDir string) *GoDepFind {
	if rootDir == "" {
		rootDir = "."
	}
	return &GoDepFind{
		rootDir:         rootDir,
		testImports:     false,
		cachedModule:    false,
		packageCache:    make(map[string]*build.Package),
		dependencyGraph: make(map[string][]string),
		reverseDeps:     make(map[string][]string),
		fileToPackage:   make(map[string]string),
		mainPackages:    []string{},
	}
}

// DepHandler interface for handlers that manage specific main files
// DepHandler interface for handlers that manage specific main files
type DepHandler interface {
	Name() string         // handler name: wasmH, serverHttp, cliApp
	MainFilePath() string // eg: web/main.server.go, web/main.wasm.go
}

// ThisFileIsMine determines if a file belongs to a specific handler using dependency analysis
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, fileName, filePath, event string) (bool, error) {
	if dh == nil {
		return false, fmt.Errorf("handler cannot be nil")
	}

	// Update cache based on file changes when queried
	if err := g.updateCacheForFile(fileName, filePath, event); err != nil {
		return false, fmt.Errorf("cache update failed: %w", err)
	}

	// Use optimized GoFileComesFromMain to find which main packages depend on this file
	mainPackages, err := g.GoFileComesFromMain(fileName)
	if err != nil {
		return false, fmt.Errorf("dependency analysis failed: %w", err)
	}

	// Check if any main package matches handler's managed file
	handlerFile := dh.MainFilePath()
	for _, mainPkg := range mainPackages {
		// Compare main package with handler's managed file
		if g.matchesHandlerFile(mainPkg, handlerFile) {
			return true, nil
		}
	}

	return false, nil
}

// SetTestImports enables or disables inclusion of test imports
func (g *GoDepFind) SetTestImports(enabled bool) {
	g.testImports = enabled
}

// listPackages returns the result of running "go list" with the specified path
func (g *GoDepFind) listPackages(path string) ([]string, error) {
	cmd := exec.Command("go", "list", path)
	cmd.Dir = g.rootDir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// getPackages imports and returns a build.Package for each listed package
func (g *GoDepFind) getPackages(paths []string) (map[string]*build.Package, error) {
	packages := make(map[string]*build.Package)
	for _, path := range paths {
		var pkg *build.Package
		var err error

		// For module paths like "testproject/appAserver", we need to convert them to relative directory paths
		// First, try to determine if this is a local module path
		if strings.Contains(path, "/") {
			// Extract the relative path from the module path
			// For "testproject/appAserver", we want just "appAserver"
			parts := strings.Split(path, "/")
			if len(parts) >= 2 {
				// Try to construct the relative path from the module root
				relativePath := strings.Join(parts[1:], "/")
				fullPath := filepath.Join(g.rootDir, relativePath)

				// Check if this directory exists
				if _, err := os.Stat(fullPath); err == nil {
					pkg, err = build.ImportDir(fullPath, 0)
					if err == nil {
						packages[path] = pkg
						continue
					}
				}
			}
		}

		// Fallback: try ImportDir with the full path as relative
		fullPath := filepath.Join(g.rootDir, path)
		if _, err := os.Stat(fullPath); err == nil {
			pkg, err = build.ImportDir(fullPath, 0)
			if err == nil {
				packages[path] = pkg
				continue
			}
		}

		// Last resort: try build.Import (for standard library packages)
		pkg, err = build.Import(path, g.rootDir, 0)
		if err != nil {
			return nil, err
		}
		packages[path] = pkg
	}
	return packages, nil
}

// imports returns true if path imports any of the packages in "any", transitively
func (g *GoDepFind) imports(path string, packages map[string]*build.Package, any map[string]bool) bool {
	if any[path] {
		return true
	}
	pkg, ok := packages[path]
	if !ok || pkg == nil {
		return false
	}

	// Check test imports if enabled
	if g.testImports {
		for _, imp := range pkg.TestImports {
			if any[imp] {
				return true
			}
		}
		for _, imp := range pkg.XTestImports {
			if any[imp] {
				return true
			}
		}
	}

	// Check regular imports
	for _, imp := range pkg.Imports {
		if g.imports(imp, packages, any) {
			any[path] = true
			return true
		}
	}
	return false
}

// FindReverseDeps finds packages in sourcePath that import any of the targetPaths
func (g *GoDepFind) FindReverseDeps(sourcePath string, targetPaths []string) ([]string, error) {
	// Build target map
	targets := make(map[string]bool)
	for _, targetPath := range targetPaths {
		packages, err := g.listPackages(targetPath)
		if err != nil {
			return nil, err
		}
		for _, path := range packages {
			targets[path] = true
		}
	}

	// Get source packages
	paths, err := g.listPackages(sourcePath)
	if err != nil {
		return nil, err
	}

	packages, err := g.getPackages(paths)
	if err != nil {
		return nil, err
	}

	// Find packages that import targets
	var result []string
	for path := range packages {
		if g.imports(path, packages, targets) {
			result = append(result, path)
		}
	}

	return result, nil
}

// GoFileComesFromMain finds which main packages depend on the given file (cached version)
// fileName: the name of the file to check (e.g., "module3.go")
// Returns: slice of main package paths that depend on this file
func (g *GoDepFind) GoFileComesFromMain(fileName string) ([]string, error) {
	// Ensure cache is initialized
	if err := g.ensureCacheInitialized(); err != nil {
		return nil, err
	}

	// Find the package containing the file using cache
	filePkg, exists := g.fileToPackage[fileName]
	if !exists || filePkg == "" {
		return []string{}, nil // File not found or not in any package
	}

	// Check which main packages import the file's package using cached data
	var result []string
	for _, mainPath := range g.mainPackages {
		if g.cachedMainImportsPackage(mainPath, filePkg) {
			result = append(result, mainPath)
		}
	}

	return result, nil
}

// findMainPackages finds all packages with main function
func (g *GoDepFind) findMainPackages() ([]string, error) {
	allPaths, err := g.listPackages("./...")
	if err != nil {
		return nil, err
	}

	packages, err := g.getPackages(allPaths)
	if err != nil {
		return nil, err
	}

	var mainPaths []string
	for path, pkg := range packages {
		if pkg.Name == "main" {
			mainPaths = append(mainPaths, path)
		}
	}

	return mainPaths, nil
}

// findPackageContainingFile finds which package contains the given file
func (g *GoDepFind) findPackageContainingFile(fileName string) (string, error) {
	allPaths, err := g.listPackages("./...")
	if err != nil {
		return "", err
	}

	packages, err := g.getPackages(allPaths)
	if err != nil {
		return "", err
	}

	for path, pkg := range packages {
		// Check GoFiles
		for _, file := range pkg.GoFiles {
			if filepath.Base(file) == fileName {
				return path, nil
			}
		}
		// Check TestGoFiles if testImports is enabled
		if g.testImports {
			for _, file := range pkg.TestGoFiles {
				if filepath.Base(file) == fileName {
					return path, nil
				}
			}
			for _, file := range pkg.XTestGoFiles {
				if filepath.Base(file) == fileName {
					return path, nil
				}
			}
		}
	}

	return "", nil // File not found in any package
}
