package godepfind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDynamicDependencyDetection simulates:
// 1) initial create of a main without imports and a module package
// 2) modify main to import the module
// 3) ensure changes cause the finder to detect the module file as belonging to the main
func TestDynamicDependencyDetection(t *testing.T) {
	// 1) Crear un directorio temporal que sirve como raíz de trabajo para la prueba
	//    (evita efectos secundarios en el sistema de archivos del desarrollador)
	tmp := t.TempDir()

	// 2) Crear las rutas de aplicación y del módulo dentro del directorio temporal
	//    - appDserver: contendrá el programa "main"
	//    - modules/database: contendrá el paquete de módulo `database`
	appDir := filepath.Join(tmp, "appDserver")
	modDir := filepath.Join(tmp, "modules", "database")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}

	// 3) Escribir un `main.go` inicial SIN importar el paquete del módulo.
	//    Esto simula que el ejecutable no depende aún del módulo.
	mainSrc := `package main

func main() {
    // initially no imports
}
`
	mainPath := filepath.Join(appDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// 4) Escribir el archivo del módulo `db.go` con un paquete `database`
	//    y una función exportada `Ping`. Inicialmente el main no lo importa.
	dbSrc := `package database

// Exported function
func Ping() {}
`
	dbPath := filepath.Join(modDir, "db.go")
	if err := os.WriteFile(dbPath, []byte(dbSrc), 0644); err != nil {
		t.Fatalf("write db.go: %v", err)
	}

	// 5) Añadir un `go.mod` en la raíz temporal para que las herramientas del
	//    ecosistema Go (por ejemplo `go list`) funcionen correctamente.
	modFile := `module testmod

go 1.17
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// 6) Inicializar el "finder" con la raíz temporal. El finder es la
	//    pieza bajo prueba que decide a qué "main" pertenece un fichero.
	finder := New(tmp)

	// 7) Simular el registro/creación inicial de archivos:
	//    - se envía un evento de `create` para `main.go` y debe ser "propio"
	//      (es manejado por el handler del propio main)
	relMain := filepath.Join("appDserver", "main.go")
	isMine, err := finder.ThisFileIsMine(relMain, mainPath, "create")
	if err != nil {
		t.Fatalf("create main error: %v", err)
	}
	if !isMine {
		t.Fatalf("expected main to be owned by handler on create")
	}

	// 8) Comprobar que el archivo del módulo NO pertenece al main inicialmente
	//    porque `main.go` todavía no lo importa.
	isMine, err = finder.ThisFileIsMine(relMain, dbPath, "create")
	if err != nil {
		t.Fatalf("create db error: %v", err)
	}
	if isMine {
		t.Fatalf("expected db NOT to belong to main initially")
	}

	// 9) Modificar `main.go` para agregar el import hacia
	//    `testmod/modules/database` y usar `database.Ping()`.
	//    Esto crea una dependencia dinámica desde el main al módulo.
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

	// 10) Disparar un evento `write` en `main.go` para que el finder
	//     re-evalúe las dependencias y registre que el main ahora usa el módulo.
	isMine, err = finder.ThisFileIsMine(relMain, mainPath, "write")
	if err != nil {
		t.Fatalf("write main error: %v", err)
	}
	if !isMine {
		t.Fatalf("expected write on main to still be owned by handler")
	}

	// 11) Modificar `db.go` (append de una línea) y cerrar el archivo. Esto
	//     simula un cambio en el módulo que debería notificar al finder.
	f, err := os.OpenFile(dbPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open db.go: %v", err)
	}
	if _, err := f.WriteString("// updated\n"); err != nil {
		f.Close()
		t.Fatalf("append db.go: %v", err)
	}
	f.Close()

	// 12) Preguntar al finder si `db.go` pertenece al `main` registrado.
	//     Esperamos `true` porque `main.go` ahora importa el paquete `database`.
	isMine, err = finder.ThisFileIsMine(relMain, dbPath, "write")
	if err != nil {
		t.Fatalf("write db error: %v", err)
	}
	if !isMine {
		// For debugging, try retrieving which mains the file comes from
		mains, _ := finder.GoFileComesFromMain(filepath.Base(dbPath))
		t.Fatalf("expected db to belong to main after import; got false; mains=%v", strings.Join(mains, ","))
	}
}
