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
	// Create temporary directory structure
	tmp := t.TempDir()

	// create directories
	appDir := filepath.Join(tmp, "appDserver")
	modDir := filepath.Join(tmp, "modules", "database")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("mkdir module dir: %v", err)
	}

	// initial main.go WITHOUT import
	mainSrc := `package main

func main() {
    // initially no imports
}
`
	mainPath := filepath.Join(appDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// module db.go
	dbSrc := `package database

// Exported function
func Ping() {}
`
	dbPath := filepath.Join(modDir, "db.go")
	if err := os.WriteFile(dbPath, []byte(dbSrc), 0644); err != nil {
		t.Fatalf("write db.go: %v", err)
	}

	// Create a go.mod in tmpDir to make go list work
	modFile := `module testmod

go 1.17
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Initialize finder with tmpDir as root
	finder := New(tmp)

	// Simulate initial registration: create events
	// main should be recognized as belonging to itself
	relMain := filepath.Join("appDserver", "main.go")
	isMine, err := finder.ThisFileIsMine(relMain, mainPath, "create")
	if err != nil {
		t.Fatalf("create main error: %v", err)
	}
	if !isMine {
		t.Fatalf("expected main to be owned by handler on create")
	}

	// module should NOT belong to main initially
	isMine, err = finder.ThisFileIsMine(relMain, dbPath, "create")
	if err != nil {
		t.Fatalf("create db error: %v", err)
	}
	if isMine {
		t.Fatalf("expected db NOT to belong to main initially")
	}

	// Modify main.go to import the module package
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

	// Trigger write event on main (pass handler mainFilePath as relative)
	isMine, err = finder.ThisFileIsMine(relMain, mainPath, "write")
	if err != nil {
		t.Fatalf("write main error: %v", err)
	}
	if !isMine {
		t.Fatalf("expected write on main to still be owned by handler")
	}

	// Now modify db.go (add a comment) and trigger write
	f, err := os.OpenFile(dbPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open db.go: %v", err)
	}
	if _, err := f.WriteString("// updated\n"); err != nil {
		f.Close()
		t.Fatalf("append db.go: %v", err)
	}
	f.Close()

	// Now db.go should be reported as belonging to the main handler
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
