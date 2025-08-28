package godepfind_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdvelop/godepfind"
)

// TestGoHandlerRoutingIssue is a unit test that validates the file routing logic
// between multiple handlers to ensure correct file ownership assignment.
//
// Test setup covers 3 scenarios:
// 1. Two main files in same directory separated by build tags:
//   - main.server.go (build tag: !wasm) imports "testproject/database"
//   - main.wasm.go (build tag: wasm) imports "testproject/dom"
//
// 2. One main file in separate directory:
//   - cmd/main.go (no build tags) imports "testproject/cmdtool"
//
// Expected behavior:
// 1. Server handler should claim database/db.go (because main.server.go imports database)
// 2. Server handler should NOT claim dom/dom.go or cmdtool/cmd.go (no import relationship)
// 3. WASM handler should claim dom/dom.go (because main.wasm.go imports dom)
// 4. WASM handler should NOT claim database/db.go or cmdtool/cmd.go (no import relationship)
// 5. CMD handler should claim cmdtool/cmd.go (because cmd/main.go imports cmdtool)
// 6. CMD handler should NOT claim database/db.go or dom/dom.go (no import relationship)
//
// This test verifies that godepfind.ThisFileIsMine correctly determines file ownership
// based on import dependencies and build tags, ensuring each handler claims only its relevant files.
func TestGoHandlerRoutingIssue(t *testing.T) {
	// 1. Crear un directorio temporal que represente el proyecto
	tmp := t.TempDir()

	// 2. Crear la estructura de carpetas: pwa, database, dom, cmd, cmdtool
	serverDir := filepath.Join(tmp, "pwa")
	databaseDir := filepath.Join(tmp, "database")
	domDir := filepath.Join(tmp, "dom")
	cmdDir := filepath.Join(tmp, "cmd")
	cmdtoolDir := filepath.Join(tmp, "cmdtool")

	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatalf("mkdir serverDir: %v", err)
	}
	if err := os.MkdirAll(databaseDir, 0755); err != nil {
		t.Fatalf("mkdir databaseDir: %v", err)
	}
	if err := os.MkdirAll(domDir, 0755); err != nil {
		t.Fatalf("mkdir domDir: %v", err)
	}
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir cmdDir: %v", err)
	}
	if err := os.MkdirAll(cmdtoolDir, 0755); err != nil {
		t.Fatalf("mkdir cmdtoolDir: %v", err)
	}

	// 3. Escribir el archivo go.mod básico para el módulo de prueba
	goModContent := `module testproject

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// 4. Crear main.server.go que importa el paquete database (representa el server)
	serverMainPath := filepath.Join(serverDir, "main.server.go")
	serverContent := `//go:build !wasm
// +build !wasm

package main

import "testproject/database"

func main() {

	database.Connect()

	printf("Server starting on port 4430")
}
`
	if err := os.WriteFile(serverMainPath, []byte(serverContent), 0644); err != nil {
		t.Fatalf("write server main: %v", err)
	}

	// 5. Crear database/db.go con una función Connect (archivo que debe pertenecer al servidor)
	dbPath := filepath.Join(databaseDir, "db.go")
	dbContent := `package database

func Connect() {
	println("Connected to database...")
}
`
	if err := os.WriteFile(dbPath, []byte(dbContent), 0644); err != nil {
		t.Fatalf("write db.go: %v", err)
	}

	// 5b. Crear dom/dom.go con una función exportada (domDir ya definido arriba)
	if err := os.MkdirAll(domDir, 0755); err != nil {
		t.Fatalf("mkdir domDir (second): %v", err)
	}
	domPath := filepath.Join(domDir, "dom.go")
	domContent := `package dom

// DomFunc es una función exportada usada por main.wasm.go en la segunda fase.
func DomFunc() {}
`
	if err := os.WriteFile(domPath, []byte(domContent), 0644); err != nil {
		t.Fatalf("write dom.go: %v", err)
	}

	// 5c. Crear main.wasm.go dentro de pwa que importa testproject/dom
	wasmMainPath := filepath.Join(serverDir, "main.wasm.go")
	wasmContent := `//go:build wasm
// +build wasm

package main

import "testproject/dom"

func main() {
	dom.DomFunc()
	println("wasm main")
}
`
	if err := os.WriteFile(wasmMainPath, []byte(wasmContent), 0644); err != nil {
		t.Fatalf("write wasm main: %v", err)
	}

	// 5d. Crear cmd/main.go que importa testproject/cmdtool (caso 3: main en directorio separado)
	cmdMainPath := filepath.Join(cmdDir, "main.go")
	cmdContent := `package main

import "testproject/cmdtool"

func main() {
	cmdtool.Execute()
	println("cmd main")
}
`
	if err := os.WriteFile(cmdMainPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("write cmd main: %v", err)
	}

	// 5e. Crear cmdtool/cmd.go con una función Execute
	cmdtoolPath := filepath.Join(cmdtoolDir, "cmd.go")
	cmdtoolContent := `package cmdtool

// Execute es una función exportada usada por cmd/main.go
func Execute() {
	println("Executing command...")
}
`
	if err := os.WriteFile(cmdtoolPath, []byte(cmdtoolContent), 0644); err != nil {
		t.Fatalf("write cmdtool cmd: %v", err)
	}

	// 6. Preparar contadores para capturar llamadas registradas por cada handler
	var serverCalls []string
	var wasmCalls []string
	var cmdCalls []string

	// 7. Crear un handler de servidor simulado (como goserver.New)
	serverHandler := &TestServerHandler{
		mainPath: "pwa/main.server.go",
		calls:    &serverCalls,
	}

	// 8. Crear un handler WASM simulado que usa main.wasm.go
	// Simulamos el comportamiento real de TinyWasm que construye dinámicamente la ruta
	wasmHandler := &TestWasmHandler{
		webFilesRootRelative: "pwa",          // Como en TinyWasm real
		mainInputFile:        "main.wasm.go", // Como en TinyWasm real
		calls:                &wasmCalls,
	}

	// 9. Crear un handler CMD simulado que usa cmd/main.go
	cmdHandler := &TestCmdHandler{
		mainPath: "cmd/main.go",
		calls:    &cmdCalls,
	}

	// 10. Obtener el buscador de dependencias directamente
	depFinder := godepfind.New(tmp)

	// 10. Comprobar la lógica de enrutamiento: ¿qué handler reclama database/db.go?
	t.Logf("Testing file ownership detection for database/db.go")

	// 13. Preguntarle al depFinder si el handler del servidor reclama db.go
	serverShouldClaim, err := depFinder.ThisFileIsMine(serverHandler.MainInputFileRelativePath(), dbPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine server on db.go: %v", err)
	}
	t.Logf("Server handler (main: %s) claims db.go: %v", serverHandler.MainInputFileRelativePath(), serverShouldClaim)

	// 14. Preguntarle al depFinder si el handler WASM reclama db.go
	//     (se espera false porque main.wasm.go no importa database)
	wasmShouldClaim, err := depFinder.ThisFileIsMine(wasmHandler.MainInputFileRelativePath(), dbPath, "write")
	if err != nil {
		t.Logf("WASM handler (main: %s) error claiming db.go: %v", wasmHandler.MainInputFileRelativePath(), err)
		wasmShouldClaim = false
	} else {
		t.Logf("WASM handler (main: %s) claims db.go: %v", wasmHandler.MainInputFileRelativePath(), wasmShouldClaim)
	}

	// 14b. Preguntarle al depFinder si el handler CMD reclama db.go
	//      (se espera false porque cmd/main.go no importa database)
	cmdShouldClaim, err := depFinder.ThisFileIsMine(cmdHandler.MainInputFileRelativePath(), dbPath, "write")
	if err != nil {
		t.Logf("CMD handler (main: %s) error claiming db.go: %v", cmdHandler.MainInputFileRelativePath(), err)
		cmdShouldClaim = false
	} else {
		t.Logf("CMD handler (main: %s) claims db.go: %v", cmdHandler.MainInputFileRelativePath(), cmdShouldClaim)
	}

	// 15. Analizar resultados para database/db.go - solo server handler debería reclamarlo
	claimersCount := 0
	if serverShouldClaim {
		claimersCount++
	}
	if wasmShouldClaim {
		claimersCount++
	}
	if cmdShouldClaim {
		claimersCount++
	}

	if claimersCount == 0 {
		t.Errorf("UNEXPECTED: No handler claims database/db.go")
		t.Errorf("Expected: Server handler should claim db.go (main.server.go imports database)")
	} else if claimersCount == 1 && serverShouldClaim {
		t.Logf("SUCCESS: Only server handler correctly claims database/db.go")
	} else if claimersCount > 1 {
		t.Errorf("ROUTING CONFLICT: Multiple handlers claim database/db.go")
		t.Errorf("Expected: Only server handler should claim db.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverShouldClaim, wasmShouldClaim, cmdShouldClaim)
	} else {
		t.Errorf("ROUTING ERROR: Wrong handler claims database/db.go")
		t.Errorf("Expected: Only server handler should claim db.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverShouldClaim, wasmShouldClaim, cmdShouldClaim)
	}

	// 16. Comprobar si el handler WASM reclama dom.go (debería ser true)
	wasmClaimsDom, err := depFinder.ThisFileIsMine(wasmHandler.MainInputFileRelativePath(), domPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine wasm on dom.go: %v", err)
	}
	t.Logf("WASM handler claims dom.go: %v", wasmClaimsDom)

	// 17. Comprobar si el servidor reclama dom.go (debería ser false)
	serverClaimsDom, err := depFinder.ThisFileIsMine(serverHandler.MainInputFileRelativePath(), domPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine server on dom.go: %v", err)
	}
	t.Logf("Server handler claims dom.go: %v", serverClaimsDom)

	// 17b. Comprobar si el CMD handler reclama dom.go (debería ser false)
	cmdClaimsDom, err := depFinder.ThisFileIsMine(cmdHandler.MainInputFileRelativePath(), domPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine cmd on dom.go: %v", err)
	}
	t.Logf("CMD handler claims dom.go: %v", cmdClaimsDom)

	// 18. Validar la lógica de dom.go - solo WASM handler debería reclamarlo
	domClaimersCount := 0
	if serverClaimsDom {
		domClaimersCount++
	}
	if wasmClaimsDom {
		domClaimersCount++
	}
	if cmdClaimsDom {
		domClaimersCount++
	}

	if domClaimersCount == 0 {
		t.Errorf("UNEXPECTED: No handler claims dom/dom.go")
		t.Errorf("Expected: WASM handler should claim dom.go (main.wasm.go imports dom)")
	} else if domClaimersCount == 1 && wasmClaimsDom {
		t.Logf("SUCCESS: Only WASM handler correctly claims dom/dom.go")
	} else if domClaimersCount > 1 {
		t.Errorf("ROUTING CONFLICT: Multiple handlers claim dom/dom.go")
		t.Errorf("Expected: Only WASM handler should claim dom.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverClaimsDom, wasmClaimsDom, cmdClaimsDom)
	} else {
		t.Errorf("ROUTING ERROR: Wrong handler claims dom/dom.go")
		t.Errorf("Expected: Only WASM handler should claim dom.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverClaimsDom, wasmClaimsDom, cmdClaimsDom)
	}

	// 19. Comprobar la lógica de cmdtool/cmd.go - solo CMD handler debería reclamarlo
	t.Logf("Testing file ownership detection for cmdtool/cmd.go")

	serverClaimsCmd, err := depFinder.ThisFileIsMine(serverHandler.MainInputFileRelativePath(), cmdtoolPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine server on cmdtool: %v", err)
	}
	t.Logf("Server handler claims cmd.go: %v", serverClaimsCmd)

	wasmClaimsCmd, err := depFinder.ThisFileIsMine(wasmHandler.MainInputFileRelativePath(), cmdtoolPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine wasm on cmdtool: %v", err)
	}
	t.Logf("WASM handler claims cmd.go: %v", wasmClaimsCmd)

	cmdClaimsCmd, err := depFinder.ThisFileIsMine(cmdHandler.MainInputFileRelativePath(), cmdtoolPath, "write")
	if err != nil {
		t.Fatalf("ThisFileIsMine cmd on cmdtool: %v", err)
	}
	t.Logf("CMD handler claims cmd.go: %v", cmdClaimsCmd)

	// 20. Validar la lógica de cmdtool/cmd.go - solo CMD handler debería reclamarlo
	cmdtoolClaimersCount := 0
	if serverClaimsCmd {
		cmdtoolClaimersCount++
	}
	if wasmClaimsCmd {
		cmdtoolClaimersCount++
	}
	if cmdClaimsCmd {
		cmdtoolClaimersCount++
	}

	if cmdtoolClaimersCount == 0 {
		t.Errorf("UNEXPECTED: No handler claims cmdtool/cmd.go")
		t.Errorf("Expected: CMD handler should claim cmd.go (cmd/main.go imports cmdtool)")
	} else if cmdtoolClaimersCount == 1 && cmdClaimsCmd {
		t.Logf("SUCCESS: Only CMD handler correctly claims cmdtool/cmd.go")
	} else if cmdtoolClaimersCount > 1 {
		t.Errorf("ROUTING CONFLICT: Multiple handlers claim cmdtool/cmd.go")
		t.Errorf("Expected: Only CMD handler should claim cmd.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverClaimsCmd, wasmClaimsCmd, cmdClaimsCmd)
	} else {
		t.Errorf("ROUTING ERROR: Wrong handler claims cmdtool/cmd.go")
		t.Errorf("Expected: Only CMD handler should claim cmd.go")
		t.Errorf("Server claims: %v, WASM claims: %v, CMD claims: %v", serverClaimsCmd, wasmClaimsCmd, cmdClaimsCmd)
	}

	// 21. Modificar dom.go con un comentario simple y verificar que el archivo fue actualizado
	domUpdated := `package dom

// DomFunc es una función exportada usada por main.wasm.go en la segunda fase.
// updated comment
func DomFunc() {}
`
	if err := os.WriteFile(domPath, []byte(domUpdated), 0644); err != nil {
		t.Fatalf("update dom.go: %v", err)
	}
	b, err := os.ReadFile(domPath)
	if err != nil {
		t.Fatalf("read dom.go: %v", err)
	}
	if !strings.Contains(string(b), "updated comment") {
		t.Errorf("dom.go fue actualizado pero el contenido esperado no fue encontrado")
	}
}

// TestServerHandler simulates goserver.ServerHandler for testing
type TestServerHandler struct {
	mainPath string
	calls    *[]string
}

func (h *TestServerHandler) MainInputFileRelativePath() string {
	return h.mainPath
}

func (h *TestServerHandler) NewFileEvent(fileName, extension, filePath, event string) error {
	call := "SERVER: " + event + " " + fileName + " " + filePath
	*h.calls = append(*h.calls, call)
	return nil
}

// TestWasmHandler simulates tinywasm.TinyWasm for testing
type TestWasmHandler struct {
	webFilesRootRelative string // Simula Config.WebFilesRootRelative de TinyWasm
	mainInputFile        string // Simula mainInputFile de TinyWasm ("main.wasm.go")
	calls                *[]string
}

func (h *TestWasmHandler) MainInputFileRelativePath() string {
	// Simula el comportamiento real de TinyWasm.MainInputFileRelativePath()
	// return path.Join(rootFolder, w.mainInputFile)
	return h.webFilesRootRelative + "/" + h.mainInputFile
}

func (h *TestWasmHandler) NewFileEvent(fileName, extension, filePath, event string) error {
	call := "WASM: " + event + " " + fileName + " " + filePath
	*h.calls = append(*h.calls, call)
	return nil
}

// TestCmdHandler simulates a command-line tool handler for testing
type TestCmdHandler struct {
	mainPath string
	calls    *[]string
}

func (h *TestCmdHandler) MainInputFileRelativePath() string {
	return h.mainPath
}

func (h *TestCmdHandler) NewFileEvent(fileName, extension, filePath, event string) error {
	call := "CMD: " + event + " " + fileName + " " + filePath
	*h.calls = append(*h.calls, call)
	return nil
}
