package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cdvelop/godepfind"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <source> <target> [target...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s ./... fmt os\n", os.Args[0])
		os.Exit(1)
	}

	// Create a new GoDepFind instance
	finder := godepfind.New(".")

	// Enable test imports if needed
	// finder.SetTestImports(true)

	sourcePath := os.Args[1]
	targetPaths := os.Args[2:]

	// Find reverse dependencies
	deps, err := finder.FindReverseDeps(sourcePath, targetPaths)
	if err != nil {
		log.Fatalf("Error finding dependencies: %v", err)
	}

	// Print results
	fmt.Printf("Packages in '%s' that import %v:\n", sourcePath, targetPaths)
	for _, dep := range deps {
		fmt.Println(dep)
	}

	if len(deps) == 0 {
		fmt.Println("No packages found.")
	}
}
