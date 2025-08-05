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

### 3. Handler Interface (Simplified)
```go
type DepHandler interface {
    Name() string                           // handler name: wasmH, serverHttp, cliApp
    UnobservedFiles() []string             // main handler files: main.go, main.wasm.go
}
```

**Important**: `UnobservedFiles()` returns the **main files that this handler manages**. GoDepFind uses this to determine file ownership by comparing if a changed file matches any handler's main files.

### 4. Cache Invalidation Strategy

#### File Events Handling:
- **`write`**: Invalidate only the package containing the file
- **`create`**: Re-scan dependencies of the parent package + update fileToPackage mapping
- **`remove`**: Invalidate dependencies pointing to that file + remove from fileToPackage
- **`rename`**: Treat as remove + create sequence

#### File Ownership Method (No Registration Required):
```go
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, fileName, filePath, event string) (bool, error)
```
- **No handler registration needed** - handler passes itself directly
- Simplifies API by eliminating registration complexity
- Direct access to handler's UnobservedFiles() method
- Returns error for robust error handling

### 5. Lazy Loading Implementation
```go
func (g *GoDepFind) ensureCacheInitialized() error {
    if !g.CachedModule {
        return g.rebuildCache()
    }
    return nil
}
```

### 6. Integration with Watcher System

**Cache Management in ThisFileIsMine**: Cache updates happen only when handlers query file ownership
```go
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, fileName, filePath, event string) (bool, error) {
    // Update cache based on file changes when queried
    if err := g.updateCacheForFile(fileName, filePath, event); err != nil {
        return false, err
    }
    
    // Then check ownership
    return g.checkFileOwnership(dh, fileName)
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
    
    // Check if any main package matches handler's managed files
    handlerFiles := dh.UnobservedFiles()
    for _, mainPkg := range mainPackages {
        for _, handlerFile := range handlerFiles {
            // Compare main package with handler's managed files
            if filepath.Base(mainPkg) == handlerFile || strings.Contains(mainPkg, handlerFile) {
                return true, nil
            }
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
    isMine, err := w.goDepFind.ThisFileIsMine(w, fileName, filePath, event)
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

## Pending Questions/Clarifications

### 1. **File Matching Strategy - RESOLVED**
The file matching strategy will use the optimized `GoFileComesFromMain` method:

**Strategy**: 
1. Use `GoFileComesFromMain(fileName)` to get which main packages depend on the changed file
2. Compare the result with the handler's `UnobservedFiles()` to determine ownership
3. If any main package from the result matches any file in `UnobservedFiles()`, the handler owns this file

**Example Logic**:
```go
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, fileName, filePath, event string) (bool, error) {
    // Update cache and get which main packages depend on this file
    mainPackages, err := g.GoFileComesFromMain(fileName)
    if err != nil {
        return false, err
    }
    
    // Check if any main package matches handler's managed files
    handlerFiles := dh.UnobservedFiles()
    for _, mainPkg := range mainPackages {
        for _, handlerFile := range handlerFiles {
            if filepath.Base(mainPkg) == handlerFile || strings.Contains(mainPkg, handlerFile) {
                return true, nil
            }
        }
    }
    
    return false, nil
}
```

This approach is much more intelligent than simple filename comparison because it uses actual dependency analysis.

### 2. **Integration Approach - CONFIRMED**
This approach is confirmed:
- **GoDepFind NO FileEvent**: GoDepFind doesn't implement FileEvent interface
- **Cache updates in ThisFileIsMine**: Cache management only when handlers query
- **Intelligent file ownership**: Uses `GoFileComesFromMain` for dependency-based matching
- **No registration required**: Handlers pass themselves directly

## Cache Optimization Strategy

The key optimization is in `GoFileComesFromMain` method which will be enhanced with caching:
- Cache the results of `go list ./...`
- Cache package dependency graphs
- Cache main package identification
- Only invalidate relevant parts when files change

This makes `ThisFileIsMine` both intelligent and performant.

## Implementation Phases

1. **Phase 1**: Basic cache structure and lazy loading
2. **Phase 2**: Handler interface definition
3. **Phase 3**: `ThisFileIsMine` method implementation (no registration)
4. **Phase 4**: File event handling and cache invalidation
5. **Phase 5**: Integration testing with godev watcher

## Real-World Usage Example

```go
// No FileEvent implementation in GoDepFind

// Handler implements DepHandler and uses ThisFileIsMine
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
    const e = "NewFileEvent Wasm"
    
    if filePath == "" {
        return errors.New(e + "filePath is empty")
    }
    
    // No registration needed - pass self directly
    // Cache is updated internally within ThisFileIsMine
    isMine, err := w.goDepFind.ThisFileIsMine(w, fileName, filePath, event)
    if err != nil {
        return fmt.Errorf("%s: %w", e, err)
    }
    if !isMine {
        return nil
    }
    
    return w.processFileChange(fileName, filePath, event)
}

// Handler implements DepHandler interface
func (w *TinyWasm) Name() string {
    return "wasmH"
}

func (w *TinyWasm) UnobservedFiles() []string {
    return []string{"main.wasm.go", "f.main.go"}
}
```

**Benefits of This Approach**:
- ✅ **No registration complexity** - handlers pass themselves
- ✅ **Simplified API** - eliminates handler management
- ✅ **Direct access** to handler methods
- ✅ **Type safety** - interface ensures proper implementation
- ✅ **Clean separation** - cache maintenance vs. handler logic

Please confirm if this approach addresses all requirements before proceeding with implementation.