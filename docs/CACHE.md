# Cache System Design for GoDepFind

## Problem Statement

Currently, `GoDepFind` doesn't cache the analyzed package information, performing the same expensive operations on each call:
- `go list ./...` to get all packages
- `build.Import` or `build.ImportDir` for each package  
- Recursive dependency analysis

This makes it costly for real development environments where files change regularly.

## Proposed Solution

### 1. Cache Strategy - Hybrid Approach
- **Global cache** with `CachedModule bool` field
- **Selective invalidation** by package when files change
- **Lazy loading** initialization for simplicity

### 2. Cache Structure
```go
type GoDepFind struct {
    rootDir     string
    testImports bool
    
    // Cache fields
    CachedModule    bool
    packageCache    map[string]*build.Package
    dependencyGraph map[string][]string // pkg -> dependencies
    reverseDeps     map[string][]string // pkg -> reverse dependencies
    fileToPackage   map[string]string   // filename -> package path
    mainPackages    []string
}
```

### 3. File Ownership Method
```go
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error)
```
- The `mainFilePath` is passed directly.
- Simplifies API by eliminating the `DepHandler` interface.
- Returns error for robust error handling.

### 4. Lazy Loading Implementation
```go
func (g *GoDepFind) ensureCacheInitialized() error {
    if !g.CachedModule {
        return g.rebuildCache()
    }
    return nil
}
```

### 5. Integration with Watcher System

**Cache Management in ThisFileIsMine**: Cache updates happen only when handlers query file ownership
```go
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error) {
    // Update cache based on file changes when queried
    if err := g.updateCacheForFile(filepath.Base(filePath), filePath, event); err != nil {
        return false, err
    }
    
    // Then check ownership
    // ...
}
```

**Benefits**:
- Cache updates only when needed (on-demand)
- No unnecessary FileEvent implementation in GoDepFind
- Simpler integration - handlers manage their own logic
- Cache management stays internal to GoDepFind

## Cache Persistence
- **In-memory only** (simpler implementation, lost on restart)

## Error Handling
- `ThisFileIsMine` returns `(bool, error)` for robust error handling

## Implementation Details

### File Ownership Logic with Cache Management
```go
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error) {
    if mainFilePath == "" {
        return false, fmt.Errorf("mainFilePath cannot be empty")
    }
    
    // Update cache based on file changes when queried
    if err := g.updateCacheForFile(filepath.Base(filePath), filePath, event); err != nil {
        return false, fmt.Errorf("cache update failed: %w", err)
    }
    
    // Use optimized GoFileComesFromMain to find which main packages depend on this file
    mainPackages, err := g.GoFileComesFromMain(filepath.Base(filePath))
    if err != nil {
        return false, fmt.Errorf("dependency analysis failed: %w", err)
    }
    
    // Check if any main package matches the handler's main file path
    for _, mainPkg := range mainPackages {
        if filepath.Base(mainPkg) == mainFilePath || strings.Contains(mainPkg, mainFilePath) {
            return true, nil
        }
    }
    
    return false, nil
}
```

### Integration with Watcher + ThisFileIsMine
```go
// No FileEvent implementation in GoDepFind - handlers manage their own logic

// Handlers use ThisFileIsMine for their specific logic AND cache management
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
    const e = "NewFileEvent Wasm"
    
    if filePath == "" {
        return errors.New(e + "filePath is empty")
    }
    
    // Cache is updated internally when ThisFileIsMine is called
    isMine, err := w.goDepFind.ThisFileIsMine(w.mainFilePath, filePath, event)
    if err != nil {
        return fmt.Errorf("%s: %w", e, err)
    }
    if !isMine {
        return nil // Not our file, skip processing
    }
    
    // Process file change...
    return w.processFileChange(fileName, filePath, event)
}
```

## Cache Optimization Strategy

The key optimization is in `GoFileComesFromMain` method which will be enhanced with caching:
- Cache the results of `go list ./...`
- Cache package dependency graphs
- Cache main package identification
- Only invalidate relevant parts when files change

This makes `ThisFileIsMine` both intelligent and performant.

## Implementation Phases

1. **Phase 1**: Basic cache structure and lazy loading
2. **Phase 2**: `ThisFileIsMine` method implementation
3. **Phase 3**: File event handling and cache invalidation
4. **Phase 4**: Integration testing with godev watcher

## Real-World Usage Example

```go
// No FileEvent implementation in GoDepFind

// Example of how a handler would use ThisFileIsMine
func (w *SomeHandler) NewFileEvent(fileName, extension, filePath, event string) error {
    const e = "NewFileEvent"
    
    if filePath == "" {
        return errors.New(e + "filePath is empty")
    }
    
    // Cache is updated internally within ThisFileIsMine
    isMine, err := w.goDepFind.ThisFileIsMine(w.mainFilePath, filePath, event)
    if err != nil {
        return fmt.Errorf("%s: %w", e, err)
    }
    if !isMine {
        return nil
    }
    
    return w.processFileChange(fileName, filePath, event)
}
```

**Benefits of This Approach**:
- ✅ **No registration complexity**
- ✅ **Simplified API** - eliminates handler management
- ✅ **Clean separation** - cache maintenance vs. handler logic

Please confirm if this approach addresses all requirements before proceeding with implementation.