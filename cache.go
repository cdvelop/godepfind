package godepfind

import (
	"fmt"
	"path/filepath"
	"strings"
)

// matchesHandlerFile checks if a main package matches a handler's managed file
func (g *GoDepFind) matchesHandlerFile(mainPkg, handlerFile string) bool {
	// Extract base name from main package path
	baseName := filepath.Base(mainPkg)

	// Direct match
	if baseName == handlerFile {
		return true
	}

	// Check if handler file pattern matches main package
	// e.g., "main.server.go" matches packages containing "server"
	if strings.Contains(handlerFile, ".") {
		parts := strings.SplitSeq(handlerFile, ".")
		for part := range parts {
			if part != "main" && part != "go" && strings.Contains(mainPkg, part) {
				return true
			}
		}
	}

	// Check if main package contains handler file (without extension)
	handlerBase := strings.TrimSuffix(handlerFile, filepath.Ext(handlerFile))
	return strings.Contains(mainPkg, handlerBase)
}

// updateCacheForFile updates cache based on file events
func (g *GoDepFind) updateCacheForFile(fileName, filePath, event string) error {
	// Initialize cache if needed
	if err := g.ensureCacheInitialized(); err != nil {
		return err
	}

	switch event {
	case "write":
		// Invalidate only the package containing the file
		return g.invalidatePackageCache(fileName)
	case "create":
		// Re-scan dependencies of the parent package + update fileToPackage mapping
		return g.handleFileCreate(fileName, filePath)
	case "remove":
		// Invalidate dependencies pointing to that file + remove from fileToPackage
		return g.handleFileRemove(fileName, filePath)
	case "rename":
		// Treat as remove + create sequence
		if err := g.handleFileRemove(fileName, filePath); err != nil {
			return err
		}
		return g.handleFileCreate(fileName, filePath)
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
func (g *GoDepFind) invalidatePackageCache(fileName string) error {
	// Find package containing the file
	if pkg, exists := g.fileToPackage[fileName]; exists {
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
	}
	return nil
}

// handleFileCreate handles file creation events
func (g *GoDepFind) handleFileCreate(fileName, filePath string) error {
	// Find which package this file belongs to
	pkg, err := g.findPackageContainingFile(fileName)
	if err != nil {
		return err
	}

	if pkg != "" {
		// Update fileToPackage mapping
		g.fileToPackage[fileName] = pkg

		// Invalidate package cache to force re-scan
		return g.invalidatePackageCache(fileName)
	}

	return nil
}

// handleFileRemove handles file removal events
func (g *GoDepFind) handleFileRemove(fileName, filePath string) error {
	// Remove from fileToPackage mapping
	delete(g.fileToPackage, fileName)

	// Invalidate package cache
	return g.invalidatePackageCache(fileName)
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

	// 4. Build fileToPackage mapping
	g.fileToPackage = make(map[string]string)
	for pkgPath, pkg := range packages {
		if pkg != nil {
			// Map Go files
			for _, file := range pkg.GoFiles {
				fileName := filepath.Base(file)
				g.fileToPackage[fileName] = pkgPath
			}

			// Map test files if enabled
			if g.testImports {
				for _, file := range pkg.TestGoFiles {
					fileName := filepath.Base(file)
					g.fileToPackage[fileName] = pkgPath
				}
				for _, file := range pkg.XTestGoFiles {
					fileName := filepath.Base(file)
					g.fileToPackage[fileName] = pkgPath
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
			if g.cachedImports(dep, targetPkg, visited) {
				return true
			}
		}
	}

	return false
}
