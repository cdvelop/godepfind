package godepfind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// extractImportPathDebug is a debug version of extractImportPath with logging
func extractImportPathDebug(line string) string {
	// Remove comments
	if idx := strings.Index(line, "//"); idx != -1 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	// Skip empty lines
	if line == "" {
		return ""
	}

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

func TestDynamicDependencyDetectionDebug(t *testing.T) {
	// Setup - igual que en el test original
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "appDserver")
	modDir := filepath.Join(tmp, "modules", "database")

	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}

	// go.mod
	modFile := `module testmod

go 1.17
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// main.go inicial SIN imports
	mainSrc := `package main

func main() {
    // initially no imports
}
`
	mainPath := filepath.Join(appDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// db.go
	dbSrc := `package database

// Exported function
func Ping() {}
`
	dbPath := filepath.Join(modDir, "db.go")
	if err := os.WriteFile(dbPath, []byte(dbSrc), 0644); err != nil {
		t.Fatalf("write db.go: %v", err)
	}

	finder := New(tmp)
	relMain := filepath.Join("appDserver", "main.go")

	// Paso 1: main.go no debería reclamar db.go inicialmente
	t.Logf("=== PASO 1: Checking initial state ===")
	isMine, err := finder.ThisFileIsMine(relMain, dbPath, "create")
	if err != nil {
		t.Fatalf("initial check error: %v", err)
	}
	t.Logf("Initial db.go belongs to main: %v (expected: false)", isMine)

	// Paso 2: Modificar main.go para agregar imports
	t.Logf("=== PASO 2: Modifying main.go to add imports ===")
	mainWithImport := `package main

import (
    "testmod/modules/database"
)

func main() {
    database.Ping()
}
`
	if err := os.WriteFile(mainPath, []byte(mainWithImport), 0644); err != nil {
		t.Fatalf("modify main.go: %v", err)
	}

	// Paso 3: Llamar ThisFileIsMine con main.go para actualizar cache
	t.Logf("=== PASO 3: Calling ThisFileIsMine on modified main.go ===")
	isMine, err = finder.ThisFileIsMine(relMain, mainPath, "write")
	if err != nil {
		t.Fatalf("write main error: %v", err)
	}
	t.Logf("Modified main.go belongs to handler: %v (expected: true)", isMine)

	// Paso 4: Verificar estado del cache después de la actualización
	t.Logf("=== PASO 4: Checking cache state ===")
	mains, err := finder.GoFileComesFromMain("db.go")
	if err != nil {
		t.Logf("GoFileComesFromMain error: %v", err)
	} else {
		t.Logf("db.go comes from mains: %v", strings.Join(mains, ","))
	}

	// Paso 5: Investigar qué paquete se detecta para db.go
	t.Logf("=== PASO 5: Investigating package detection ===")
	targetPkg, err := finder.findPackageForFile(dbPath)
	if err != nil {
		t.Logf("findPackageForFile error: %v", err)
	} else {
		t.Logf("db.go is detected as belonging to package: %s", targetPkg)
	}

	// Verificar si ese paquete debería pertenecer al handler
	if targetPkg != "" {
		t.Logf("=== Debugging doesPackageBelongToHandler ===")

		// Check if it's a main package
		isMain := finder.isMainPackage(targetPkg)
		t.Logf("Is %s a main package? %v", targetPkg, isMain)

		// Check if handler file imports this package
		t.Logf("=== Debugging parseFileImports ===")
		handlerAbsPath := filepath.Join(tmp, relMain)
		t.Logf("Handler absolute path: %s", handlerAbsPath)

		// Read the file content to see what's actually there
		content, err := os.ReadFile(handlerAbsPath)
		if err != nil {
			t.Logf("Error reading handler file: %v", err)
		} else {
			t.Logf("Handler file content:\n%s", string(content))
		}

		// Call parseFileImports directly
		imports, err := finder.parseFileImports(handlerAbsPath)
		if err != nil {
			t.Logf("parseFileImports error: %v", err)
		} else {
			t.Logf("Parsed imports: %v", imports)
		}

		// Debug line by line parsing
		t.Logf("=== Manual line parsing debug ===")
		lines := strings.Split(string(content), "\n")
		inImportBlock := false

		for i, line := range lines {
			originalLine := line
			line = strings.TrimSpace(line)
			t.Logf("Line %d: '%s' (trimmed: '%s')", i, originalLine, line)

			// Single line import
			if strings.HasPrefix(line, "import ") {
				t.Logf("  -> Single line import detected")
				// Use the extractImportPath function to see what it returns
				path := extractImportPathDebug(line)
				t.Logf("  -> extractImportPath returned: '%s'", path)
				continue
			}

			// Multi-line import block start
			if line == "import (" {
				t.Logf("  -> Import block start")
				inImportBlock = true
				continue
			}

			// Multi-line import block end
			if inImportBlock && line == ")" {
				t.Logf("  -> Import block end")
				inImportBlock = false
				continue
			}

			// Import inside block
			if inImportBlock {
				t.Logf("  -> Inside import block")
				path := extractImportPathDebug(line)
				t.Logf("  -> extractImportPath returned: '%s'", path)
			}
		}

		importsPackage := finder.handlerFileImportsPackage(relMain, targetPkg)
		t.Logf("Does %s import %s? %v", relMain, targetPkg, importsPackage)

		belongs := finder.doesPackageBelongToHandler(targetPkg, relMain)
		t.Logf("Package %s belongs to handler %s: %v", targetPkg, relMain, belongs)
	}

	// Paso 6: Preguntar si db.go pertenece a main (este debería ser true ahora)
	t.Logf("=== PASO 6: Final check - should db.go belong to main now? ===")
	isMine, err = finder.ThisFileIsMine(relMain, dbPath, "write")
	if err != nil {
		t.Fatalf("final check error: %v", err)
	}
	t.Logf("Final db.go belongs to main: %v (expected: true)", isMine)

	if !isMine {
		t.Errorf("FAILED: db.go should belong to main after import was added")
	} else {
		t.Logf("SUCCESS: db.go correctly belongs to main after import")
	}
}
