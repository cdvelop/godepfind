package godepfind

import (
	"fmt"
	"path/filepath"
)

// updateCacheForFile updates cache based on file events
func (g *GoDepFind) updateCacheForFile(filePath, event string) error {
	// Initialize cache if needed
	if err := g.ensureCacheInitialized(); err != nil {
		return err
	}

	switch event {
	case "write":
		// Invalidate only the package containing the file
		return g.invalidatePackageCache(filePath)
	case "create":
		// Re-scan dependencies of the parent package + update fileToPackage mapping
		return g.handleFileCreate(filePath)
	case "remove":
		// Invalidate dependencies pointing to that file + remove from fileToPackage
		return g.handleFileRemove(filePath)
	case "rename":
		// Treat as remove + create sequence
		if err := g.handleFileRemove(filePath); err != nil {
			return err
		}
		return g.handleFileCreate(filePath)
	}

	return nil
}

// ensureCacheInitialized initializes cache if not already done (lazy loading)
func (g *GoDepFind) ensureCacheInitialized() error {
	if !g.cachedModule {
		return g.rebuildCache()
	}
	return nil
}

// invalidatePackageCache invalidates cache for a specific package
func (g *GoDepFind) invalidatePackageCache(filePath string) error {
	// Find the package containing this file
	pkg, err := g.findPackageContainingFileByPath(filePath)
	if err != nil || pkg == "" {
		return nil // File not found in any package, nothing to invalidate
	}

	// Remove from caches
	delete(g.packageCache, pkg)
	delete(g.dependencyGraph, pkg)
	delete(g.reverseDeps, pkg)

	// Remove from other packages' dependency lists
	for otherPkg := range g.dependencyGraph {
		deps := g.dependencyGraph[otherPkg]
		for i, dep := range deps {
			if dep == pkg {
				g.dependencyGraph[otherPkg] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}
	return nil
}

// invalidatePackageCacheOnly invalidates only packageCache, preserves dependencyGraph
func (g *GoDepFind) invalidatePackageCacheOnly(filePath string) error {
	// Find the package containing this file
	pkg, err := g.findPackageContainingFileByPath(filePath)
	if err != nil || pkg == "" {
		return nil // File not found in any package, nothing to invalidate
	}

	// Only remove from packageCache, preserve dependencyGraph and reverseDeps
	delete(g.packageCache, pkg)
	return nil
}

// handleFileCreate handles file creation events
func (g *GoDepFind) handleFileCreate(filePath string) error {
	// filePath is now always required and contains full path
	pkg, err := g.findPackageContainingFileByPath(filePath)
	if err != nil {
		return err
	}

	if pkg != "" {
		// Update path mapping
		if absPath, err := filepath.Abs(filePath); err == nil {
			g.filePathToPackage[absPath] = pkg
		}

		// Add to filename mapping (don't overwrite, append if not exists)
		fileName := filepath.Base(filePath)
		if !contains(g.fileToPackages[fileName], pkg) {
			g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkg)
		}

		return g.invalidatePackageCache(filePath)
	}
	return nil
}

// handleFileRemove handles file removal events
func (g *GoDepFind) handleFileRemove(filePath string) error {
	// Remove from path mapping
	if filePath != "" {
		if absPath, err := filepath.Abs(filePath); err == nil {
			delete(g.filePathToPackage, absPath)
		}
	}

	// Remove from filename mapping requires package lookup first
	if filePath != "" {
		pkg, _ := g.findPackageContainingFileByPath(filePath)
		if pkg != "" {
			fileName := filepath.Base(filePath)
			g.fileToPackages[fileName] = removeString(g.fileToPackages[fileName], pkg)
		}
	}

	return g.invalidatePackageCache(filePath)
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// rebuildCache rebuilds the entire cache from scratch
func (g *GoDepFind) rebuildCache() error {
	// 1. Get all packages
	allPaths, err := g.listPackages("./...")
	if err != nil {
		return fmt.Errorf("failed to list packages: %w", err)
	}

	// 2. Build package cache
	packages, err := g.getPackages(allPaths)
	if err != nil {
		return fmt.Errorf("failed to get packages: %w", err)
	}
	g.packageCache = packages

	// 3. Build dependency graph and reverse dependencies
	g.dependencyGraph = make(map[string][]string)
	g.reverseDeps = make(map[string][]string)

	for pkgPath, pkg := range packages {
		if pkg != nil {
			// Store dependencies
			g.dependencyGraph[pkgPath] = pkg.Imports

			// Build reverse dependencies
			for _, imp := range pkg.Imports {
				if g.reverseDeps[imp] == nil {
					g.reverseDeps[imp] = []string{}
				}
				g.reverseDeps[imp] = append(g.reverseDeps[imp], pkgPath)
			}

			// Include test imports if enabled
			if g.testImports {
				for _, imp := range pkg.TestImports {
					if g.reverseDeps[imp] == nil {
						g.reverseDeps[imp] = []string{}
					}
					g.reverseDeps[imp] = append(g.reverseDeps[imp], pkgPath)
				}
				for _, imp := range pkg.XTestImports {
					if g.reverseDeps[imp] == nil {
						g.reverseDeps[imp] = []string{}
					}
					g.reverseDeps[imp] = append(g.reverseDeps[imp], pkgPath)
				}
			}
		}
	}

	// 4. Build file-to-package mappings
	g.filePathToPackage = make(map[string]string)
	g.fileToPackages = make(map[string][]string)
	for pkgPath, pkg := range packages {
		if pkg != nil {
			// Map Go files by absolute path AND collect by filename
			for _, file := range pkg.GoFiles {
				// Absolute path mapping (unique)
				absPath := filepath.Join(pkg.Dir, file)
				g.filePathToPackage[absPath] = pkgPath

				// Filename mapping (may have multiple packages)
				fileName := filepath.Base(file)
				g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkgPath)
			}

			// Map test files if enabled
			if g.testImports {
				for _, file := range pkg.TestGoFiles {
					absPath := filepath.Join(pkg.Dir, file)
					g.filePathToPackage[absPath] = pkgPath
					fileName := filepath.Base(file)
					g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkgPath)
				}
				for _, file := range pkg.XTestGoFiles {
					absPath := filepath.Join(pkg.Dir, file)
					g.filePathToPackage[absPath] = pkgPath
					fileName := filepath.Base(file)
					g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkgPath)
				}
			}
		}
	}

	// 5. Identify main packages
	g.mainPackages = []string{}
	for pkgPath, pkg := range packages {
		if pkg != nil && pkg.Name == "main" {
			g.mainPackages = append(g.mainPackages, pkgPath)
		}
	}

	// 6. Mark cache as initialized
	g.cachedModule = true

	return nil
}

// cachedMainImportsPackage checks if a main package imports a target package using cache
func (g *GoDepFind) cachedMainImportsPackage(mainPath, targetPkg string) bool {
	// Use cached dependency graph for faster lookups
	visited := make(map[string]bool)
	return g.cachedImports(mainPath, targetPkg, visited)
}

// isSameFile compares two file paths for equality (robust absolute comparison)
func (g *GoDepFind) isSameFile(filePath1, filePath2 string) bool {
	abs1, err1 := filepath.Abs(filePath1)
	abs2, err2 := filepath.Abs(filePath2)
	if err1 != nil || err2 != nil {
		return filePath1 == filePath2
	}

	// If one is relative, try to make it absolute relative to rootDir
	if !filepath.IsAbs(filePath2) {
		abs2FromRoot, err := filepath.Abs(filepath.Join(g.rootDir, filePath2))
		if err == nil {
			abs2 = abs2FromRoot
		}
	}
	if !filepath.IsAbs(filePath1) {
		abs1FromRoot, err := filepath.Abs(filepath.Join(g.rootDir, filePath1))
		if err == nil {
			abs1 = abs1FromRoot
		}
	}

	return abs1 == abs2
}

// updateCacheForFileWithContext updates cache based on file events and handler context
func (g *GoDepFind) updateCacheForFileWithContext(filePath, event, handlerMainFile string) error {
	// Initialize cache if needed
	if err := g.ensureCacheInitialized(); err != nil {
		return err
	}

	switch event {
	case "write":
		// Only rescan fully if the modified file is the handler's mainInputFileRelativePath
		if handlerMainFile != "" && g.isSameFile(filePath, handlerMainFile) {
			return g.rescanMainPackageDependencies(filePath)
		}
		// For non-main files, only invalidate package cache (don't touch dependency graph)
		return g.invalidatePackageCacheOnly(filePath)
	case "create":
		return g.handleFileCreate(filePath)
	case "remove":
		return g.handleFileRemove(filePath)
	case "rename":
		if err := g.handleFileRemove(filePath); err != nil {
			return err
		}
		return g.handleFileCreate(filePath)
	}

	return nil
}

// rescanMainPackageDependencies rescans only the dependencies of the main package
func (g *GoDepFind) rescanMainPackageDependencies(mainInputFileRelativePath string) error {
	// Simpler and robust: rebuild entire cache for module when main changes.
	// This ensures dependencyGraph, file mappings and mainPackages stay consistent.
	if err := g.rebuildCache(); err != nil {
		return err
	}
	return nil
}

// cachedImports returns true if path imports targetPkg transitively using cache
func (g *GoDepFind) cachedImports(path, targetPkg string, visited map[string]bool) bool {
	if visited[path] {
		return false // Avoid cycles
	}
	visited[path] = true

	if path == targetPkg {
		return true
	}

	// Use cached dependency graph
	if deps, exists := g.dependencyGraph[path]; exists {
		for _, dep := range deps {
			if dep == targetPkg {
				return true
			}
			if g.cachedImports(dep, targetPkg, visited) {
				return true
			}
		}
	}

	return false
}
