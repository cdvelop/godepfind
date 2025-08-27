# Solución: Detección de Dependencias Dinámicas en GoDepFind

## Descripción del Problema

Actualmente, `GoDepFind` no detecta correctamente las dependencias que se establecen dinámicamente durante el desarrollo. El escenario específico es:

1. **Estado inicial**: Se tienen dos archivos independientes:
   - `appDserver/main.go` (paquete main, sin dependencias)
   - `modules/database/db.go` (paquete exportado, huérfano)
   
2. **Registro inicial**: Ambos archivos se registran con `ThisFileIsMine` usando evento "create" (simulando `InitialRegistration`)

3. **Modificación dinámica**: Se modifica `appDserver/main.go` para importar `modules/database`

4. **Problema**: Cuando posteriormente se modifica `modules/database/db.go`, `ThisFileIsMine` no detecta que pertenece a `appDserver/main.go`

## Root Cause

El problema está en la función `updateCacheForFile` cuando se maneja el evento "write":

```go
case "write":
    // Invalidate only the package containing the file
    return g.invalidatePackageCache(fileName)
```

**Limitación**: Cuando se modifica `appDserver/main.go` para agregar un import, solo se invalida el cache del paquete `appDserver`, pero:

1. **No se re-escanean las dependencias**: El grafo de dependencias (`dependencyGraph`) no se actualiza para reflejar el nuevo import
2. **Cache desactualizado**: `cachedMainImportsPackage` sigue usando el grafo anterior sin la nueva dependencia
3. **Detección fallida**: Cuando llega `modules/database/db.go`, no encuentra la conexión con `appDserver`

## Solución Propuesta: Re-escaneado Selectivo

**Descripción**: Re-escanear las dependencias SOLO cuando el archivo modificado ES exactamente el `mainFilePath` (handlerFile) especificado.

### Lógica de Implementación

```go
func (g *GoDepFind) updateCacheForFile(fileName, filePath, event string) error {
    // Initialize cache if needed
    if err := g.ensureCacheInitialized(); err != nil {
        return err
    }

    switch event {
    case "write":
        // CLAVE: Solo re-escanear si el archivo modificado ES el mainFilePath del handler
        // Esta información debe venir del contexto de la llamada
        if g.isTargetMainFile(filePath) {
            return g.rescanMainPackageDependencies(filePath)
        }
        return g.invalidatePackageCache(fileName)
    case "create":
        return g.handleFileCreate(fileName, filePath)
    case "remove":
        return g.handleFileRemove(fileName, filePath)
    case "rename":
        if err := g.handleFileRemove(fileName, filePath); err != nil {
            return err
        }
        return g.handleFileCreate(fileName, filePath)
    }
    return nil
}
```

### Funciones de Soporte

```go
// isTargetMainFile verifica si el archivo modificado es un mainFilePath activo
func (g *GoDepFind) isTargetMainFile(filePath string) bool {
    // Esta función necesita acceso al contexto del handler
    // Se puede implementar de varias formas:
    
    // Opción 1: Mantener registro de mainFilePaths activos
    return g.activeMainFiles[filePath]
    
    // Opción 2: Verificar si es main package Y está en la lista de monitoreados
    pkg, _ := g.findPackageContainingFileByPath(filePath)
    return g.isMainPackage(pkg) && g.isMonitoredMainFile(filePath)
}

// rescanMainPackageDependencies re-escanea solo las dependencias del paquete main específico
func (g *GoDepFind) rescanMainPackageDependencies(mainFilePath string) error {
    // 1. Identificar el paquete main
    pkg, err := g.findPackageContainingFileByPath(mainFilePath)
    if err != nil {
        return err
    }
    
    // 2. Re-escanear solo este paquete específico
    if err := g.rescanSpecificPackage(pkg); err != nil {
        return err
    }
    
    // 3. Actualizar el grafo de dependencias solo para este main
    return g.updateDependencyGraphForMain(pkg)
}

// rescanSpecificPackage re-escanea un paquete específico y actualiza su cache
func (g *GoDepFind) rescanSpecificPackage(pkgPath string) error {
    // Remover del cache
    delete(g.packageCache, pkgPath)
    delete(g.dependencyGraph, pkgPath)
    
    // Re-escanear
    packages, err := g.getPackages([]string{pkgPath})
    if err != nil {
        return err
    }
    
    // Actualizar cache
    if pkg, exists := packages[pkgPath]; exists {
        g.packageCache[pkgPath] = pkg
        if pkg != nil {
            g.dependencyGraph[pkgPath] = pkg.Imports
        }
    }
    
    return nil
}
```

### Integración con ThisFileIsMine

Para que `updateCacheForFile` sepa cuál es el `mainFilePath` del handler actual, necesitamos modificar la signatura o mantener estado:

```go
// Opción 1: Modificar signatura (recomendado)
func (g *GoDepFind) updateCacheForFileWithContext(fileName, filePath, event, handlerMainFile string) error {
    // ... lógica existente ...
    
    case "write":
        // Verificar si este archivo ES el mainFilePath del handler
        if filePath == handlerMainFile || g.isSameFile(filePath, handlerMainFile) {
            return g.rescanMainPackageDependencies(filePath)
        }
        return g.invalidatePackageCache(fileName)
}

// Actualizar ThisFileIsMine para pasar el contexto
func (g *GoDepFind) ThisFileIsMine(mainFilePath, filePath, event string) (bool, error) {
    // ... validaciones existentes ...
    
    // Update cache with handler context
    if err := g.updateCacheForFileWithContext(fileName, filePath, event, mainFilePath); err != nil {
        return false, fmt.Errorf("cache update failed: %w", err)
    }
    
    // ... resto de la lógica ...
}

// Helper para comparar archivos
func (g *GoDepFind) isSameFile(filePath1, filePath2 string) bool {
    abs1, err1 := filepath.Abs(filePath1)
    abs2, err2 := filepath.Abs(filePath2)
    if err1 != nil || err2 != nil {
        return filePath1 == filePath2
    }
    return abs1 == abs2
}
```

## Ventajas de Esta Solución

✅ **Rendimiento Óptimo**: Solo re-escanea cuando es estrictamente necesario
✅ **Precisión**: Solo afecta el archivo main específico que se modificó  
✅ **Simplicidad**: Lógica directa y fácil de entender
✅ **Bajo Impacto**: No afecta otros handlers o archivos
✅ **Escalabilidad**: O(1) en términos de número de handlers
✅ **Compatibilidad**: No rompe funcionalidad existente

## Casos de Uso Cubiertos

1. **Scenario Original**: 
   - `appDserver/main.go` se modifica → Re-escanea dependencies
   - `modules/database/db.go` se modifica → Solo invalida cache (no re-escanea)
   - Posterior query a `modules/database/db.go` → Encuentra conexión con `appDserver`

2. **Múltiples Handlers**:
   - Handler A monitorea `app1/main.go`
   - Handler B monitorea `app2/main.go`  
   - Solo el handler cuyo mainFile se modifica hace re-escaneado

3. **Archivos No-Main**:
   - Modificaciones a archivos que no son mainFilePath → Solo invalidación de cache
   - Sin overhead de re-escaneado innecesario

## Plan de Implementación

### Cambios Requeridos

1. **Modificar `updateCacheForFile`** para aceptar contexto del handler:
   ```go
   func (g *GoDepFind) updateCacheForFileWithContext(fileName, filePath, event, handlerMainFile string) error
   ```

2. **Crear función de comparación de archivos**:
   ```go
   func (g *GoDepFind) isSameFile(filePath1, filePath2 string) bool
   ```

3. **Implementar re-escaneado selectivo**:
   ```go
   func (g *GoDepFind) rescanMainPackageDependencies(mainFilePath string) error
   func (g *GoDepFind) rescanSpecificPackage(pkgPath string) error
   func (g *GoDepFind) updateDependencyGraphForMain(pkgPath string) error
   ```

4. **Actualizar `ThisFileIsMine`** para pasar el contexto del handler

### Test de Validación

```go
func TestDynamicDependencyDetection(t *testing.T) {
    // Configurar directorio temporal con:
    // - appDserver/main.go (sin imports inicialmente)
    // - modules/database/db.go (archivo huérfano)
    
    finder := New(tempDir)
    
    // 1. Registro inicial - ambos archivos independientes
    isMine, _ := finder.ThisFileIsMine("appDserver/main.go", "appDserver/main.go", "create")
    assert.True(t, isMine) // El main se reconoce a sí mismo
    
    isMine, _ = finder.ThisFileIsMine("appDserver/main.go", "modules/database/db.go", "create")
    assert.False(t, isMine) // El database NO pertenece al main inicialmente
    
    // 2. Modificar main.go para agregar import
    addImportToMainFile("appDserver/main.go", "modules/database")
    
    // 3. Simular evento "write" en el main.go
    isMine, _ = finder.ThisFileIsMine("appDserver/main.go", "appDserver/main.go", "write")
    assert.True(t, isMine) // Trigger re-escaneado
    
    // 4. Verificar que ahora database.go pertenece al main
    isMine, _ = finder.ThisFileIsMine("appDserver/main.go", "modules/database/db.go", "write")
    assert.True(t, isMine) // ¡Ahora SÍ debería detectar la conexión!
}
```

## Próximos Pasos

1. ✅ **Análisis completado** - Solución definida
2. 🔄 **Implementar cambios** en `cache.go` y `godepfind.go`  
3. 🧪 **Crear test comprehensivo** para validar el escenario
4. 🚀 **Ejecutar test** y verificar que funciona correctamente
5. 📚 **Documentar** el comportamiento nuevo en comentarios del código

Esta solución es **simple, eficiente y precisa** - solo re-escanea cuando el archivo específico que está siendo monitoreado por un handler se modifica, evitando overhead innecesario.
