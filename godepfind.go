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
	cachedModule      bool
	packageCache      map[string]*build.Package
	dependencyGraph   map[string][]string // pkg -> dependencies
	reverseDeps       map[string][]string // pkg -> reverse dependencies
	filePathToPackage map[string]string   // absolute file path -> package path (NEW: unique mapping)
	fileToPackages    map[string][]string // filename -> list of package paths (NEW: multiple packages per filename)
	mainPackages      []string
}

// New creates a new GoDepFind instance with the specified root directory
func New(rootDir string) *GoDepFind {
	if rootDir == "" {
		rootDir = "."
	}
	return &GoDepFind{
		rootDir:           rootDir,
		testImports:       false,
		cachedModule:      false,
		packageCache:      make(map[string]*build.Package),
		dependencyGraph:   make(map[string][]string),
		reverseDeps:       make(map[string][]string),
		filePathToPackage: make(map[string]string),
		fileToPackages:    make(map[string][]string),
		mainPackages:      []string{},
	}
}

// ThisFileIsMine determines if a file belongs to a specific handler using path-based resolution
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error) {
	// Validar que filePath no sea solo un archivo sin ruta
	if filePath == "" {
		return false, fmt.Errorf("filePath cannot be empty")
	}

	if !strings.Contains(filePath, "/") && !strings.Contains(filePath, "\\") {
		return false, fmt.Errorf("filePath must include directory path, not just filename: %s", filePath)
	}

	// Derivar fileName del filePath
	fileName := filepath.Base(filePath)

	// Validate input before processing
	shouldProcess, err := g.ValidateInputForProcessing(mainFilePath, fileName, filePath)
	if err != nil {
		return false, err
	}
	if !shouldProcess {
		return false, nil
	}

	// Update cache based on file changes when queried
	if err := g.updateCacheForFile(fileName, filePath, event); err != nil {
		return false, fmt.Errorf("cache update failed: %w", err)
	}

	handlerFile := mainFilePath
	if handlerFile == "" {
		return false, fmt.Errorf("handler mainFilePath cannot be empty")
	}

	// FIRST: Direct file comparison - check if handler manages this specific file
	if filePath != "" && handlerFile != "" {
		// Extract the filename from handler's MainFilePath for comparison
		handlerFileName := filepath.Base(handlerFile)

		// If the filenames match, check if they're in the same relative path
		if fileName == handlerFileName {
			// Get the relative path from the project root
			relativeFilePath := strings.TrimPrefix(filePath, g.rootDir+"/")

			// Compare with handler's MainFilePath
			if relativeFilePath == handlerFile {
				return true, nil
			}
		}

		// Also try absolute path comparison as fallback
		if absFilePath, err := filepath.Abs(filePath); err == nil {
			if absHandlerPath, err := filepath.Abs(handlerFile); err == nil {
				if absFilePath == absHandlerPath {
					return true, nil
				}
			}

			// Try relative path from root
			if !filepath.IsAbs(handlerFile) {
				handlerAbsPath := filepath.Join(g.rootDir, handlerFile)
				if absHandlerPath, err := filepath.Abs(handlerAbsPath); err == nil {
					if absFilePath == absHandlerPath {
						return true, nil
					}
				}
			}
		}
	}

	// SECOND: Package-based resolution for files that aren't directly managed
	var targetPkg string
	if filePath != "" {
		// Use exact path resolution when available (priority)
		if absPath, err := filepath.Abs(filePath); err == nil {
			// Convert to relative path for cache lookup
			relPath, err := filepath.Rel(g.rootDir, absPath)
			if err != nil {
				relPath = filePath // fallback to original
			}
			if pkg, exists := g.filePathToPackage[relPath]; exists {
				targetPkg = pkg
			}
		}
	}

	// If no exact path match, find packages containing the file
	if targetPkg == "" {
		packages := g.fileToPackages[fileName]
		if len(packages) == 0 {
			return false, nil // File not found in any package
		}
		// Use the first package (path resolution should handle disambiguation)
		targetPkg = packages[0]
	}

	// Check if this is a main package that matches the handler
	if g.isMainPackage(targetPkg) && g.matchesHandlerFile(targetPkg, handlerFile) {
		return true, nil
	}

	// Check if any main package imports this target package and matches the handler
	for _, mainPath := range g.mainPackages {
		if g.cachedMainImportsPackage(mainPath, targetPkg) && g.matchesHandlerFile(mainPath, handlerFile) {
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

	// Find packages containing the file using new cache structure
	candidatePackages := g.fileToPackages[fileName]
	if len(candidatePackages) == 0 {
		return []string{}, nil // File not found in any package
	}

	// Check which main packages import any of the candidate packages using cached data
	var result []string
	for _, mainPath := range g.mainPackages {
		for _, filePkg := range candidatePackages {
			if g.cachedMainImportsPackage(mainPath, filePkg) {
				result = append(result, mainPath)
				break // Don't add the same main package multiple times
			}
		}
	}

	return result, nil
}

// isMainPackage checks if a package is a main package
func (g *GoDepFind) isMainPackage(pkgPath string) bool {
	for _, mp := range g.mainPackages {
		if mp == pkgPath {
			return true
		}
	}
	return false
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

// findPackageContainingFileByPath finds which package contains the given file path.
// It first tries the cached package info (packageCache) and falls back to
// scanning packages if cache is not available.
func (g *GoDepFind) findPackageContainingFileByPath(filePath string) (string, error) {
	// Ensure cache is initialized
	if err := g.ensureCacheInitialized(); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	// Prefer cached lookup
	if len(g.packageCache) > 0 {
		for pkgPath, pkg := range g.packageCache {
			if pkg == nil {
				continue
			}
			for _, file := range pkg.GoFiles {
				candidate := file
				if !filepath.IsAbs(candidate) {
					candidate = filepath.Join(pkg.Dir, file)
				}
				candAbs, err := filepath.Abs(candidate)
				if err != nil {
					continue
				}
				if candAbs == absPath {
					return pkgPath, nil
				}
			}
			if g.testImports {
				for _, file := range pkg.TestGoFiles {
					candidate := file
					if !filepath.IsAbs(candidate) {
						candidate = filepath.Join(pkg.Dir, file)
					}
					candAbs, err := filepath.Abs(candidate)
					if err != nil {
						continue
					}
					if candAbs == absPath {
						return pkgPath, nil
					}
				}
				for _, file := range pkg.XTestGoFiles {
					candidate := file
					if !filepath.IsAbs(candidate) {
						candidate = filepath.Join(pkg.Dir, file)
					}
					candAbs, err := filepath.Abs(candidate)
					if err != nil {
						continue
					}
					if candAbs == absPath {
						return pkgPath, nil
					}
				}
			}
		}
	}

	// Fallback: scan all packages
	allPaths, err := g.listPackages("./...")
	if err != nil {
		return "", err
	}
	packages, err := g.getPackages(allPaths)
	if err != nil {
		return "", err
	}
	for path, pkg := range packages {
		if pkg == nil {
			continue
		}
		for _, file := range pkg.GoFiles {
			candidate := file
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(pkg.Dir, file)
			}
			candAbs, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			if candAbs == absPath {
				return path, nil
			}
		}
	}

	return "", nil
}
