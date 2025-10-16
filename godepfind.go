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

// ThisFileIsMine decides whether the provided handler (identified by its
// main file path relative to the module root) should handle an event for the
// given file. It normalizes paths, validates the handler main file exists,
// updates internal caches when a main file is written (so dynamic import
// changes are recognized), and finally uses package dependency analysis to
// decide file ownership.
//
// Important: preference is given to checking the specific handler main-file
// imports (not just directory-level mains). That lets the finder disambiguate
// multiple `main` packages that may live in the same directory but are
// selected via build tags (for example: two mains in one folder with
// `//go:build wasm` vs `//go:build !wasm`). In those cases the exact main file
// (and its build tags/imports) determines ownership.
//
// Inputs:
//   - mainInputFileRelativePath: handler main file (e.g. "pwa/main.server.go")
//   - fileAbsPath: target file path (absolute or relative to module root)
//   - event: one of "write","create","remove","rename" (drives cache ops)
//
// Returns: (bool, error) — true when the handler should process the file.
func (g *GoDepFind) ThisFileIsMine(mainInputFileRelativePath, fileAbsPath, event string) (bool, error) {
	// 1. Basic input validation
	if fileAbsPath == "" {
		return false, fmt.Errorf("fileAbsPath cannot be empty")
	}
	if mainInputFileRelativePath == "" {
		return false, fmt.Errorf("handler mainInputFileRelativePath cannot be empty")
	}

	// 2. Normalize file path to absolute
	if !filepath.IsAbs(fileAbsPath) {
		fileAbsPath = filepath.Join(g.rootDir, fileAbsPath)
	}
	absFilePath, err := filepath.Abs(fileAbsPath)
	if err != nil {
		return false, fmt.Errorf("cannot resolve fileAbsPath to absolute path: %w", err)
	}
	fileAbsPath = absFilePath

	// 3. CRITICAL: Verify handler's main file exists
	handlerMainAbsPath := mainInputFileRelativePath
	if !filepath.IsAbs(handlerMainAbsPath) {
		handlerMainAbsPath = filepath.Join(g.rootDir, mainInputFileRelativePath)
	}
	if _, err := os.Stat(handlerMainAbsPath); err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("handler main file does not exist: %s", mainInputFileRelativePath)
		}
		return false, fmt.Errorf("cannot access handler main file %s: %w", mainInputFileRelativePath, err)
	}

	// 4. Validate target file (skip if file doesn't exist or is being written)
	if filepath.Ext(fileAbsPath) == ".go" {
		validator := NewGoFileValidator()
		if isValid, err := validator.IsValidGoFile(fileAbsPath); err != nil {
			return false, fmt.Errorf("file validation failed: %w", err)
		} else if !isValid {
			// File is invalid/empty/being written - skip processing
			return false, nil
		}
	}

	// 5. Direct file comparison - is this the handler's own main file?
	relativeFilePath := strings.TrimPrefix(fileAbsPath, g.rootDir+"/")
	isHandlerMainFile := relativeFilePath == mainInputFileRelativePath

	if isHandlerMainFile {
		// 6. CRITICAL: If this is the handler's main file, update cache for dynamic dependencies
		// This handles cases where main.go is modified to add/remove imports
		if err := g.updateCacheForFileWithContext(fileAbsPath, event, mainInputFileRelativePath); err != nil {
			return false, fmt.Errorf("cache update failed: %w", err)
		}
		return true, nil
	}

	// 7. For non-main files, check package-based ownership (cache already initialized if needed)
	return g.checkPackageBasedOwnership(mainInputFileRelativePath, fileAbsPath)
}

// checkPackageBasedOwnership determines ownership based on Go package dependencies
func (g *GoDepFind) checkPackageBasedOwnership(mainInputFileRelativePath, fileAbsPath string) (bool, error) {
	// Find which package contains the target file
	targetPkg, err := g.findPackageForFile(fileAbsPath)
	if err != nil {
		return false, err
	}
	if targetPkg == "" {
		return false, nil // File not found in any package
	}

	// Check if target package should belong to this handler
	return g.doesPackageBelongToHandler(targetPkg, mainInputFileRelativePath), nil
}

// findPackageForFile finds which package contains the given file
func (g *GoDepFind) findPackageForFile(fileAbsPath string) (string, error) {
	// Ensure cache is initialized
	if err := g.ensureCacheInitialized(); err != nil {
		return "", err
	}

	// Try exact path lookup first (most reliable)
	if pkg, exists := g.filePathToPackage[fileAbsPath]; exists {
		return pkg, nil
	}

	// Fallback: try relative path lookup
	if cwd, err := os.Getwd(); err == nil {
		if relPath, err := filepath.Rel(cwd, fileAbsPath); err == nil {
			if pkg, exists := g.filePathToPackage[relPath]; exists {
				return pkg, nil
			}
		}
	}

	// Last resort: filename-based lookup (may be ambiguous)
	fileName := filepath.Base(fileAbsPath)
	if packages := g.fileToPackages[fileName]; len(packages) > 0 {
		return packages[0], nil
	}

	return "", nil
}

// doesPackageBelongToHandler determines if a package should be handled by this handler
func (g *GoDepFind) doesPackageBelongToHandler(targetPkg, mainInputFileRelativePath string) bool {
	handlerDir := filepath.Dir(mainInputFileRelativePath)

	// Case 1: If target is a main package in the same directory as handler
	if g.isMainPackage(targetPkg) {
		// Extract directory from package path and compare with handler directory
		for _, mainPkg := range g.mainPackages {
			if mainPkg == targetPkg {
				if pkg, exists := g.packageCache[mainPkg]; exists && pkg != nil {
					if relPkgDir, err := filepath.Rel(g.rootDir, pkg.Dir); err == nil {
						return filepath.Clean(relPkgDir) == filepath.Clean(handlerDir)
					}
				}
				// Fallback: compare package name with handler directory
				return filepath.Base(targetPkg) == filepath.Base(handlerDir)
			}
		}
	}

	// Case 2: Check if the SPECIFIC handler file imports this target package
	// This is more precise than checking if any main package in the directory imports it
	return g.handlerFileImportsPackage(mainInputFileRelativePath, targetPkg)
}

// handlerFileImportsPackage checks if a specific handler file imports the given package
func (g *GoDepFind) handlerFileImportsPackage(handlerFileRelativePath, targetPkg string) bool {
	// Ensure cache is initialized
	if err := g.ensureCacheInitialized(); err != nil {
		return false
	}

	// Build the absolute path to the handler file
	handlerAbsPath := handlerFileRelativePath
	if !filepath.IsAbs(handlerAbsPath) {
		handlerAbsPath = filepath.Join(g.rootDir, handlerFileRelativePath)
	}

	// Parse the handler file to extract its imports
	imports, err := g.parseFileImports(handlerAbsPath)
	if err != nil {
		return false
	}

	// Direct import check
	for _, imp := range imports {
		if imp == targetPkg {
			return true
		}
	}

	// Transitive import check - check if any direct import depends on targetPkg
	for _, imp := range imports {
		if g.cachedMainImportsPackage(imp, targetPkg) {
			return true
		}
	}

	return false
}

// parseFileImports extracts the import statements from a specific Go file
func (g *GoDepFind) parseFileImports(filePath string) ([]string, error) {
	// For now, use a simple file parsing approach
	// This is a known limitation - we're parsing at file level but Go packages aggregate all files
	// For the specific use case of main.server.go vs main.wasm.go, we need to parse the files individually

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var imports []string
	lines := strings.Split(string(content), "\n")
	inImportBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Multi-line import block start (check this BEFORE single line import)
		if line == "import (" {
			inImportBlock = true
			continue
		}

		// Multi-line import block end
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Import inside block
		if inImportBlock {
			if path := extractImportPath(line); path != "" {
				imports = append(imports, path)
			}
			continue
		}

		// Single line import (check this AFTER import block detection)
		if strings.HasPrefix(line, "import ") {
			// Extract import path from 'import "path"'
			if path := extractImportPath(line); path != "" {
				imports = append(imports, path)
			}
			continue
		}
	}

	return imports, nil
}

// extractImportPath extracts the import path from an import line
func extractImportPath(line string) string {
	// Remove comments
	if idx := strings.Index(line, "//"); idx != -1 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	// Skip empty lines
	if line == "" {
		return ""
	}

	// Handle different import formats:
	// import "path"
	// "path"
	// alias "path"
	// . "path"
	// _ "path"

	// Remove import keyword if present
	line = strings.TrimPrefix(line, "import ")
	line = strings.TrimSpace(line)

	// Find the quoted path
	start := strings.Index(line, "\"")
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(line, "\"")
	if end == -1 || end <= start {
		return ""
	}

	return line[start+1 : end]
}

// SetTestImports enables or disables inclusion of test imports
func (g *GoDepFind) SetTestImports(enabled bool) {
	g.testImports = enabled
}

// listPackages returns the result of running "go list" with the specified path
// It tolerates build constraint errors (e.g., WASM packages) and returns whatever packages
// it can successfully list, only returning error if no packages are found at all
func (g *GoDepFind) listPackages(path string) ([]string, error) {
	cmd := exec.Command("go", "list", path)
	cmd.Dir = g.rootDir
	// Don't redirect stderr to os.Stderr to avoid polluting logs with build constraint warnings
	out, err := cmd.Output()

	// Parse the output even if the command failed
	packages := strings.Fields(string(out))

	// If we got at least some packages, ignore the error
	// This handles cases where some packages have build constraints (e.g., WASM)
	if len(packages) > 0 {
		return packages, nil
	}

	// Only return error if we couldn't list any packages
	if err != nil {
		return nil, err
	}

	return packages, nil
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

// matchesHandlerFile determines whether a main package path corresponds to the
// handler file provided by the watcher. The logic is intentionally simple and
// path-based: it checks whether the handler's directory matches the package
// directory (using the package cache when available) or if the package name
// matches the handler directory basename.
func (g *GoDepFind) matchesHandlerFile(mainPkg, handlerFile string) bool {
	if handlerFile == "" || mainPkg == "" {
		return false
	}

	// Normalize handler directory relative to rootDir when possible
	handlerDir := filepath.Dir(handlerFile)
	if filepath.IsAbs(handlerFile) {
		// Convert to relative from rootDir to compare with package paths
		if rel, err := filepath.Rel(g.rootDir, handlerFile); err == nil {
			handlerDir = filepath.Dir(rel)
		}
	}
	handlerDir = filepath.ToSlash(handlerDir)

	// 1) Quick base-name match: package base == handler directory base
	if filepath.Base(mainPkg) == filepath.Base(handlerDir) {
		return true
	}

	// 2) Suffix match: package path ends with handlerDir (covers cases like
	//    "testproject/test/pwa" vs handlerDir "test/pwa" or "pwa")
	if handlerDir != "." && handlerDir != "" {
		if strings.HasSuffix(filepath.ToSlash(mainPkg), handlerDir) {
			return true
		}
	}

	// 3) Fall back to packageCache lookup (if available) to compare actual
	// package directory on disk with handlerDir.
	if pkg, ok := g.packageCache[mainPkg]; ok && pkg != nil {
		if relPkgDir, err := filepath.Rel(g.rootDir, pkg.Dir); err == nil {
			relPkgDir = filepath.ToSlash(relPkgDir)
			if relPkgDir == handlerDir || strings.HasSuffix(filepath.ToSlash(mainPkg), relPkgDir) {
				return true
			}
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
