# Propuesta Final para Reestructurar `ThisFileIsMine`

## Problema Actual

La firma actual `ThisFileIsMine(dh DepHandler, fileName, filePath, event string) (bool, error)` tiene varios problemas:

1. **Ambigüedad**: `fileName` y `filePath` pueden ser inconsistentes
2. **Redundancia**: Si tenemos `filePath`, `fileName` es derivable con `filepath.Base(filePath)`
3. **Interfaz DepHandler innecesariamente compleja**: El método `Name()` no se usa realmente

## Análisis del Uso Actual

### En DevWatch.handleFileEvent:
```go
isMine, herr := h.depFinder.ThisFileIsMine(handler, fileName, eventName, eventType)
```

Donde:
- `fileName`: nombre del archivo (ej: "main.go") - REDUNDANTE
- `eventName`: path completo del archivo (ej: "pwa/main.server.go")
- `eventType`: tipo de evento ("create", "write", "remove", "rename")

### El parámetro `event` ES necesario porque:
- Se usa en `updateCacheForFile` para determinar operaciones de cache
- `"write"`: invalidar cache del paquete
- `"create"`: re-escanear dependencias + actualizar mapeos
- `"remove"`: invalidar dependencias + remover del mapping  
- `"rename"`: combinación de remove + create

## Propuesta Final: Simplificación Total

### Cambios en DepHandler:
```go
// ANTES:
type DepHandler interface {
    Name() string         // handler name: wasmH, serverHttp, cliApp
    MainInputFileRelativePath() string // package identifier: "appAserver", "appBcmd", "appCwasm", etc.
}

// DESPUÉS:
type DepHandler interface {
    MainInputFileRelativePath() string // path file ej: app/web/main.go  
}
```

### Nueva firma de ThisFileIsMine:
```go
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, filePath, event string) (bool, error)
```

### Lógica de comparación:
```go
func (g *GoDepFind) ThisFileIsMine(dh DepHandler, filePath, event string) (bool, error) {
    // Validar que filePath no sea solo un archivo sin ruta
    if filePath == "" {
        return false, fmt.Errorf("filePath cannot be empty")
    }
    
    if !strings.Contains(filePath, "/") && !strings.Contains(filePath, "\\") {
        return false, fmt.Errorf("filePath must include directory path, not just filename: %s", filePath)
    }
    
    // Derivar fileName del filePath
    fileName := filepath.Base(filePath)
    
    // Actualizar cache con evento
    if err := g.updateCacheForFile(fileName, filePath, event); err != nil {
        return false, fmt.Errorf("cache update failed: %w", err)
    }
    
    // Comparar directamente con MainInputFileRelativePath del handler
    handlerFile := dh.MainInputFileRelativePath()
    if handlerFile == "" {
        return false, fmt.Errorf("handler MainInputFileRelativePath cannot be empty")
    }
    
    // Normalizar paths para comparación
    normalizedFilePath := normalizePathForComparison(filePath, g.rootDir)
    normalizedHandlerPath := normalizePathForComparison(handlerFile, g.rootDir)
    
    if normalizedFilePath == normalizedHandlerPath {
        return true, nil
    }
    
    // Si no es el archivo principal, verificar si es dependencia
    return g.isFileDependencyOfHandler(normalizedFilePath, normalizedHandlerPath)
}
```

## Ventajas de esta Solución

### ✅ Pros:
1. **Eliminación total de redundancia**: `fileName` se deriva automáticamente
2. **Interfaz más simple**: DepHandler solo necesita un método  
3. **Menos parámetros**: De 4 a 3 parámetros
4. **Validación robusta**: Fuerza paths completos, no solo nombres de archivo
5. **Comparación directa**: `filePath` vs `handler.MainInputFileRelativePath()`
6. **Mantiene funcionalidad de cache**: `event` se preserva
7. **Previene ambigüedad**: Imposible pasar archivos sin path

### ❌ Breaking Changes:
1. **DepHandler interface**: Remover método `Name()`
2. **Firma de ThisFileIsMine**: Eliminar parámetro `fileName`
3. **DevWatch**: Cambiar llamada para eliminar `fileName`

## Migración Requerida

### 1. Actualizar DepHandler implementations:
```go
// Remover método Name() de todas las implementaciones
// Ejemplo: serverHandler, wasmHandler, cliHandler, etc.
```

### 2. Actualizar DevWatch.handleFileEvent:
```go
// ANTES:
isMine, herr := h.depFinder.ThisFileIsMine(handler, fileName, eventName, eventType)

// DESPUÉS:
isMine, herr := h.depFinder.ThisFileIsMine(handler, eventName, eventType)
```

### 3. Actualizar todos los tests que usen ThisFileIsMine

## Validaciones Agregadas

### Validación de filePath:
- No puede estar vacío
- Debe contener separadores de directorio (`/` o `\`)
- Debe ser un path completo, no solo un nombre de archivo

### Ejemplo de paths válidos:
- ✅ `"app/web/main.go"`
- ✅ `"pwa/main.server.go"`  
- ✅ `"./src/main.go"`
- ✅ `"/absolute/path/main.go"`

### Ejemplo de paths inválidos:
- ❌ `""` (vacío)
- ❌ `"main.go"` (solo nombre, sin directorio)
- ❌ `"file.go"` (solo nombre, sin directorio)

## Implementación Sugerida

### Orden de cambios:
1. Actualizar interfaz `DepHandler` (remover `Name()`)
2. Actualizar todas las implementaciones de `DepHandler`
3. Cambiar firma de `ThisFileIsMine`
4. Agregar validaciones de path
5. Actualizar `DevWatch.handleFileEvent`
6. Actualizar todos los tests
