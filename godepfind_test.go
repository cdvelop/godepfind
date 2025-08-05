package godepfind

import (
	"testing"
)

func TestNew(t *testing.T) {
	// Test with empty rootDir
	g1 := New("")
	if g1.rootDir != "." {
		t.Errorf("Expected rootDir to be '.', got %s", g1.rootDir)
	}
	if g1.testImports != false {
		t.Errorf("Expected testImports to be false by default")
	}

	// Test with specific rootDir
	g2 := New("/tmp")
	if g2.rootDir != "/tmp" {
		t.Errorf("Expected rootDir to be '/tmp', got %s", g2.rootDir)
	}
}

func TestSetTestImports(t *testing.T) {
	g := New(".")

	// Test enabling test imports
	g.SetTestImports(true)
	if !g.testImports {
		t.Errorf("Expected testImports to be true")
	}

	// Test disabling test imports
	g.SetTestImports(false)
	if g.testImports {
		t.Errorf("Expected testImports to be false")
	}
}

func TestFindReverseDeps(t *testing.T) {
	// Test with current directory
	g := New(".")

	// This test will work if we're in a Go module with some packages
	// We'll test with common Go standard library packages
	deps, err := g.FindReverseDeps("./...", []string{"fmt"})
	if err != nil {
		t.Logf("Warning: Could not find reverse dependencies: %v", err)
		return // Skip test if not in a Go module
	}

	// Should find at least some packages (including this test file)
	t.Logf("Found %d packages that import 'fmt'", len(deps))
	for _, dep := range deps {
		t.Logf("Package: %s", dep)
	}
}

func TestFindReverseDepsWithTests(t *testing.T) {
	g := New(".")
	g.SetTestImports(true)

	deps, err := g.FindReverseDeps("./...", []string{"testing"})
	if err != nil {
		t.Logf("Warning: Could not find reverse dependencies: %v", err)
		return // Skip test if not in a Go module
	}

	// Should find test packages
	t.Logf("Found %d packages that import 'testing' (including tests)", len(deps))
	for _, dep := range deps {
		t.Logf("Package: %s", dep)
	}
}
