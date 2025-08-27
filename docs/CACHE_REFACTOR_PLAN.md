# GoDepFind Cache Refactor Plan

## Current Problem Analysis

### Issue: Non-Unique File Name Mapping
The current implementation uses `fileToPackage map[string]string` which maps file names (e.g., "main.go") to package paths. This creates ambiguity when multiple packages contain files with the same name.

**Current problematic structure:**
```go
type GoDepFind struct {
    fileToPackage   map[string]string   // fileName -> package path (PROBLEM: not unique)
    // ...
}
```

**Specific problems:**
1. **File Name Collisions**: Multiple `main.go` files exist in `testproject/appAserver/main.go`, `testproject/appBcmd/main.go`, `testproject/appCwasm/main.go`
2. **Cache Overwrites**: In `rebuildCache()`, the last package processed wins when mapping `fileName` to `pkgPath`
3. **Incorrect Ownership**: `ThisFileIsMine()` may return true for wrong handlers due to ambiguous file-to-package mapping
4. **Logic Inconsistency**: `findPackageContainingFileByPath()` exists but cache still uses filename-only mapping

### Current Cache Building Process (Problematic)
```go
// In rebuildCache() - CURRENT PROBLEMATIC CODE
for _, file := range pkg.GoFiles {
    fileName := filepath.Base(file)           // "main.go"
    g.fileToPackage[fileName] = pkgPath       // OVERWRITES previous mapping!
}
```

## Proposed Solution: File Path-Based Caching

### 1. Change Cache Structure

**Replace:**
```go
fileToPackage   map[string]string   // fileName -> package path
```

**With:**
```go
filePathToPackage map[string]string   // absolute file path -> package path
fileToPackages    map[string][]string // fileName -> list of package paths (for quick lookup)
```

### 2. Benefits of New Structure

1. **Unique Mapping**: Absolute file paths are guaranteed unique
2. **Fast Lookups**: Both specific path queries and general filename queries supported
3. **No Overwrites**: Each file path maps to exactly one package
4. **Backward Compatibility**: Filename-based queries still work but return all matches

### 3. Implementation Plan

#### Phase 1: Update Data Structures

**File:** `godepfind.go`
```go
type GoDepFind struct {
    rootDir     string
    testImports bool

    // Cache fields - UPDATED STRUCTURE
    cachedModule        bool
    packageCache        map[string]*build.Package
    dependencyGraph     map[string][]string
    reverseDeps         map[string][]string
    filePathToPackage   map[string]string   // NEW: absolute path -> package
    fileToPackages      map[string][]string // NEW: fileName -> []packages
    mainPackages        []string
}
```

#### Phase 2: Update Cache Building Logic

**File:** `cache.go` - `rebuildCache()` method
```go
// 4. Build file-to-package mappings - NEW IMPLEMENTATION
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
        
        // Same for test files if enabled
        if g.testImports {
            for _, file := range pkg.TestGoFiles {
                absPath := filepath.Join(pkg.Dir, file)
                g.filePathToPackage[absPath] = pkgPath
                fileName := filepath.Base(file)
                g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkgPath)
            }
            // ... XTestGoFiles similar
        }
    }
}
```

#### Phase 3: Update ThisFileIsMine Logic

**File:** `godepfind.go` - `ThisFileIsMine()` method

**New simplified logic:**
```go
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error) {
    // Validate and update cache (unchanged)
    shouldProcess, err := g.ValidateInputForProcessing(mainFilePath, filepath.Base(filePath), filePath)
    if err != nil || !shouldProcess {
        return false, err
    }
    
    if err := g.updateCacheForFile(filepath.Base(filePath), filePath, event); err != nil {
        return false, fmt.Errorf("cache update failed: %w", err)
    }

    handlerFile := mainFilePath
    
    // SIMPLIFIED LOGIC: Check if fileName matches handler's MainFilePath
    if filepath.Base(filePath) == handlerFile {
        var candidatePackages []string
        
        // Priority 1: Use exact file path if available
        if filePath != "" {
            if absPath, err := filepath.Abs(filePath); err == nil {
                if pkg, exists := g.filePathToPackage[absPath]; exists {
                    candidatePackages = []string{pkg}
                }
            }
        }
        
        // Priority 2: Fallback to filename-based lookup
        if len(candidatePackages) == 0 {
            candidatePackages = g.fileToPackages[filepath.Base(filePath)]
        }
        
        // Check if any candidate package is a main package AND matches handler
        for _, pkg := range candidatePackages {
            if g.isMainPackage(pkg) && g.matchesHandlerFile(pkg, handlerFile) {
                return true, nil
            }
        }
    }
    
    // Fallback to existing dependency analysis logic (unchanged)
    mainPackages, err := g.GoFileComesFromMain(filepath.Base(filePath))
    if err != nil {
        return false, fmt.Errorf("dependency analysis failed: %w", err)
    }
    
    for _, mainPkg := range mainPackages {
        if g.matchesHandlerFile(mainPkg, handlerFile) {
            return true, nil
        }
    }
    
    return false, nil
}
```

#### Phase 4: Update Cache Management Functions

**File:** `cache.go`

1. **Update `invalidatePackageCache`:**
```go
func (g *GoDepFind) invalidatePackageCache(fileName string) error {
    // Find ALL packages containing this filename
    packages := g.fileToPackages[fileName]
    
    for _, pkg := range packages {
        // Remove from caches
        delete(g.packageCache, pkg)
        delete(g.dependencyGraph, pkg)
        delete(g.reverseDeps, pkg)
        
        // Clean up dependency references
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
```

2. **Update `handleFileCreate`:**
```go
func (g *GoDepFind) handleFileCreate(fileName, filePath string) error {
    if filePath != "" {
        pkg, err := g.findPackageContainingFileByPath(filePath)
        if err != nil {
            return err
        }
        
        if pkg != "" {
            // Update both mappings
            if absPath, err := filepath.Abs(filePath); err == nil {
                g.filePathToPackage[absPath] = pkg
            }
            
            // Add to filename mapping (don't overwrite)
            if !contains(g.fileToPackages[fileName], pkg) {
                g.fileToPackages[fileName] = append(g.fileToPackages[fileName], pkg)
            }
            
            return g.invalidatePackageCache(fileName)
        }
    }
    return nil
}
```

3. **Update `handleFileRemove`:**
```go
func (g *GoDepFind) handleFileRemove(fileName, filePath string) error {
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
            g.fileToPackages[fileName] = removeString(g.fileToPackages[fileName], pkg)
        }
    }
    
    return g.invalidatePackageCache(fileName)
}
```

#### Phase 5: Add Helper Functions

**File:** `godepfind.go`
```go
// isMainPackage checks if a package is a main package
func (g *GoDepFind) isMainPackage(pkgPath string) bool {
    for _, mp := range g.mainPackages {
        if mp == pkgPath {
            return true
        }
    }
    return false
}
```

**File:** `cache.go`
```go
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
```

### 4. Testing Strategy

#### Test Cases to Add/Update

1. **Multiple main.go files test:**
```go
func TestMainFileOwnershipWithMultipleMainFiles(t *testing.T) {
    finder := New("testproject")
    
    // Test each handler recognizes only its own main.go
    testCases := []struct {
        mainFilePath string
        filePath string
        expected bool
    }{
        {"appAserver/main.go", "testproject/appAserver/main.go", true},
        {"appBcmd/main.go", "testproject/appBcmd/main.go", true},
        {"appCwasm/main.go", "testproject/appCwasm/main.go", true},
        // Cross-ownership should be false
        {"appAserver/main.go", "testproject/appBcmd/main.go", false},
        {"appBcmd/main.go", "testproject/appCwasm/main.go", false},
    }
    
    for _, tc := range testCases {
        result, err := finder.ThisFileIsMine(tc.mainFilePath, tc.filePath, "write")
        require.NoError(t, err)
        assert.Equal(t, tc.expected, result, 
            "Main file %s should %s own %s", tc.mainFilePath,
            map[bool]string{true: "", false: "NOT"}[tc.expected], tc.filePath)
    }
}
```

2. **Path-based disambiguation test:**
```go
func TestFilePathDisambiguation(t *testing.T) {
    finder := New("testproject")
    
    // Create handler that manages specific main package
    mainFilePath := "appAserver/main.go"
    
    // Should return true for appAserver/main.go only
    result, err := finder.ThisFileIsMine(mainFilePath, "testproject/appAserver/main.go", "write")
    assert.True(t, result)
    
    // Should return false for other main.go files
    result, err = finder.ThisFileIsMine(mainFilePath, "testproject/appBcmd/main.go", "write")
    assert.False(t, result)
}
```

### 5. Performance Considerations

#### Benefits:
1. **O(1) exact path lookups** via `filePathToPackage`
2. **Reduced false positives** in ownership determination
3. **Cache consistency** - no more overwrites

#### Costs:
1. **Slightly more memory** - storing both path and filename mappings
2. **Initial cache build time** - computing absolute paths

#### Mitigation:
- Use `filepath.Abs()` caching to avoid repeated path resolution
- Lazy initialization remains unchanged

### 6. Migration Strategy

#### Backward Compatibility:
- Keep existing `GoFileComesFromMain()` method unchanged
- Existing API consumers continue working
- New `ThisFileIsMine()` logic is more accurate

#### Deployment:
1. **Phase 1**: Implement new cache structure
2. **Phase 2**: Update `ThisFileIsMine()` logic  
3. **Phase 3**: Add comprehensive tests
4. **Phase 4**: Remove deprecated methods (if any)

### 7. Expected Outcomes

#### Before (Current Issues):
- ❌ `fileToPackage["main.go"]` → `"testproject/appCwasm"` (last package wins)
- ❌ `serverHandler.ThisFileIsMine("main.go", "testproject/appAserver/main.go")` → `false` (wrong!)
- ❌ Cache inconsistency with file path lookups

#### After (Fixed):
- ✅ `filePathToPackage["/abs/path/to/testproject/appAserver/main.go"]` → `"testproject/appAserver"`  
- ✅ `fileToPackages["main.go"]` → `["testproject/appAserver", "testproject/appBcmd", "testproject/appCwasm"]`
- ✅ `serverHandler.ThisFileIsMine("main.go", "testproject/appAserver/main.go")` → `true` ✅
- ✅ Perfect cache consistency and no false positives

### 8. Implementation Priority

**High Priority (Fix core issue):**
1. ✅ Update data structures in `GoDepFind`
2. ✅ Fix `rebuildCache()` to use path-based mapping
3. ✅ Simplify `ThisFileIsMine()` logic

**Medium Priority (Robustness):**
4. ✅ Update cache management functions
5. ✅ Add comprehensive tests

**Low Priority (Optimization):**
6. ✅ Performance optimizations
7. ✅ Documentation updates

This refactor will make the library robust, eliminate logic errors, and provide the unique file path identification needed for reliable ownership determination.
