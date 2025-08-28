package godepfind

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProductionIssueReproduction reproduce exactamente el problema que estás viendo en producción
// donde el handler para "/home/cesar/Dev/Pkg/Mine/godev/test/pwa/main.server.go" dice que
// un archivo no le pertenece cuando debería pertenecer
func TestProductionIssueReproduction(t *testing.T) {
	// 1) Crear una estructura de directorios que replica tu entorno de producción
	tmp := t.TempDir()

	// 2) Crear el directorio pwa y su main.server.go
	pwaDir := filepath.Join(tmp, "test", "pwa")
	if err := os.MkdirAll(pwaDir, 0755); err != nil {
		t.Fatalf("mkdir pwa dir: %v", err)
	}

	// 3) Crear main.server.go con contenido similar al tuyo
	mainServerSrc := `package main

import (
	"log"
	"net/http"
)

func main() {
	publicDir := "public"
	fs := http.FileServer(http.Dir(publicDir))
	
	mux := http.NewServeMux()
	mux.Handle("/", fs)
	
	server := &http.Server{
		Addr:    ":4430",
		Handler: mux,
	}
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
`
	mainServerPath := filepath.Join(pwaDir, "main.server.go")
	if err := os.WriteFile(mainServerPath, []byte(mainServerSrc), 0644); err != nil {
		t.Fatalf("write main.server.go: %v", err)
	}

	// 4) Crear otro archivo Go en el mismo directorio que debería pertenecer al handler
	helperSrc := `package main

import "fmt"

func helper() {
	fmt.Println("helper function")
}
`
	helperPath := filepath.Join(pwaDir, "helper.go")
	if err := os.WriteFile(helperPath, []byte(helperSrc), 0644); err != nil {
		t.Fatalf("write helper.go: %v", err)
	}

	// 5) Crear go.mod en la raíz temporal
	modFile := `module testproject

go 1.17
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// 6) Inicializar el finder con la raíz temporal
	finder := New(tmp)

	// 7) Simular exactamente tu caso:
	//    - mainInputFileRelativePath: "test/pwa/main.server.go" (relativo desde la raíz)
	//    - archivo a verificar: helper.go en el mismo directorio
	mainInputFileRelativePath := filepath.Join("test", "pwa", "main.server.go")

	t.Logf("=== Reproduciendo problema de producción ===")
	t.Logf("Finder rootDir: %s", tmp)
	t.Logf("MainInputFileRelativePath: %s", mainInputFileRelativePath)
	t.Logf("Archivo a verificar: %s", helperPath)

	// 8) Usar el método de debug para obtener información detallada
	t.Logf("=== ANTES de DebugThisFileIsMine ===")
	t.Logf("finder.cachedModule: %v", finder.cachedModule)

	isMine, err := finder.DebugThisFileIsMine(mainInputFileRelativePath, helperPath, "write")
	if err != nil {
		t.Logf("Error en DebugThisFileIsMine: %v", err)
	}
	t.Logf("Resultado del debug: isMine=%v", isMine)

	t.Logf("=== DESPUÉS de DebugThisFileIsMine ===")
	t.Logf("finder.cachedModule: %v", finder.cachedModule)

	// 9) Probar con el método normal
	isMineNormal, err := finder.ThisFileIsMine(mainInputFileRelativePath, helperPath, "write")
	if err != nil {
		t.Fatalf("Error en ThisFileIsMine: %v", err)
	}

	t.Logf("Resultado final: isMine=%v", isMineNormal)

	// 10) El helper.go debería pertenecer al handler porque está en el mismo paquete main
	if !isMineNormal {
		t.Errorf("PROBLEMA REPRODUCIDO: helper.go debería pertenecer al handler de main.server.go pero retornó false")
		t.Errorf("Esto explica por qué en producción ves 'isMine=false' cuando debería ser true")
	} else {
		t.Logf("SUCCESS: El problema no se reproduce en este test")
	}

	// 11) Probar también con el propio main.server.go (debería ser true)
	isMainMine, err := finder.ThisFileIsMine(mainInputFileRelativePath, mainServerPath, "write")
	if err != nil {
		t.Fatalf("Error verificando main.server.go: %v", err)
	}
	t.Logf("main.server.go pertenece a su propio handler: %v", isMainMine)

	if !isMainMine {
		t.Errorf("CRÍTICO: main.server.go no pertenece a su propio handler!")
	}
}
