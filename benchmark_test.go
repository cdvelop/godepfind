package godepfind

import (
	"testing"
)

// BenchmarkGoFileComesFromMainWithoutCache tests performance without cache
func BenchmarkGoFileComesFromMainWithoutCache(b *testing.B) {
	// Create fresh instances each time to avoid cache
	for i := 0; i < b.N; i++ {
		finder := New("testproject")
		finder.cachedModule = false // Ensure cache is disabled

		// Test with module1.go which has dependencies
		_, err := finder.GoFileComesFromMain("module1.go")
		if err != nil {
			b.Fatalf("GoFileComesFromMain failed: %v", err)
		}
	}
}

// BenchmarkGoFileComesFromMainWithCache tests performance with cache
func BenchmarkGoFileComesFromMainWithCache(b *testing.B) {
	// Initialize cache once
	finder := New("testproject")

	// Warm up cache
	_, err := finder.GoFileComesFromMain("module1.go")
	if err != nil {
		b.Fatalf("Cache warmup failed: %v", err)
	}

	b.ResetTimer() // Reset timer after cache warmup

	for i := 0; i < b.N; i++ {
		// Test with module1.go using cached data
		_, err := finder.GoFileComesFromMain("module1.go")
		if err != nil {
			b.Fatalf("GoFileComesFromMain failed: %v", err)
		}
	}
}

// BenchmarkThisFileIsMineWithoutCache tests ThisFileIsMine without cache
func BenchmarkThisFileIsMineWithoutCache(b *testing.B) {
	mainFilePath := "appAserver/main.go"

	for i := 0; i < b.N; i++ {
		finder := New("testproject")
		finder.cachedModule = false // Ensure cache is disabled

		_, err := finder.ThisFileIsMine(mainFilePath, "./modules/module1/module1.go", "write")
		if err != nil {
			b.Fatalf("ThisFileIsMine failed: %v", err)
		}
	}
}

// BenchmarkThisFileIsMineWithCache tests ThisFileIsMine with cache
func BenchmarkThisFileIsMineWithCache(b *testing.B) {
	finder := New("testproject")
	mainFilePath := "appAserver/main.go"

	// Warm up cache
	_, err := finder.ThisFileIsMine(mainFilePath, "./modules/module1/module1.go", "write")
	if err != nil {
		b.Fatalf("Cache warmup failed: %v", err)
	}

	b.ResetTimer() // Reset timer after cache warmup

	for i := 0; i < b.N; i++ {
		_, err := finder.ThisFileIsMine(mainFilePath, "./modules/module1/module1.go", "write")
		if err != nil {
			b.Fatalf("ThisFileIsMine failed: %v", err)
		}
	}
}

// BenchmarkCacheInitialization measures cache build time
func BenchmarkCacheInitialization(b *testing.B) {
	for i := 0; i < b.N; i++ {
		finder := New("testproject")
		finder.cachedModule = false

		err := finder.ensureCacheInitialized()
		if err != nil {
			b.Fatalf("Cache initialization failed: %v", err)
		}
	}
}

// BenchmarkMultipleFilesWithCache tests performance with multiple files using cache
func BenchmarkMultipleFilesWithCache(b *testing.B) {
	finder := New("testproject")
	files := []string{"module1.go", "module2.go", "module3.go", "module4.go"}

	// Warm up cache
	for _, file := range files {
		_, _ = finder.GoFileComesFromMain(file)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, file := range files {
			_, err := finder.GoFileComesFromMain(file)
			if err != nil {
				b.Fatalf("GoFileComesFromMain failed for %s: %v", file, err)
			}
		}
	}
}

// BenchmarkMultipleFilesWithoutCache tests performance with multiple files without cache
func BenchmarkMultipleFilesWithoutCache(b *testing.B) {
	files := []string{"module1.go", "module2.go", "module3.go", "module4.go"}

	for i := 0; i < b.N; i++ {
		for _, file := range files {
			finder := New("testproject")
			finder.cachedModule = false

			_, err := finder.GoFileComesFromMain(file)
			if err != nil {
				b.Fatalf("GoFileComesFromMain failed for %s: %v", file, err)
			}
		}
	}
}

// BenchmarkRealWorldScenario simulates real development workflow
func BenchmarkRealWorldScenario(b *testing.B) {
	finder := New("testproject")

	mainFilePaths := []string{
		"appAserver/main.go",
		"appBcmd/main.go",
		"appCwasm/main.go",
	}

	files := []string{"module1/module1.go", "module2/module2.go", "module3/module3.go", "module4/module4.go"}
	events := []string{"write", "create", "remove"}

	// Warm up cache
	for _, mainFilePath := range mainFilePaths {
		for _, file := range files {
			_, _ = finder.ThisFileIsMine(mainFilePath, "modules/"+file, "write")
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate multiple handlers checking file ownership
		mainFilePath := mainFilePaths[i%len(mainFilePaths)]
		file := files[i%len(files)]
		event := events[i%len(events)]

		_, err := finder.ThisFileIsMine(mainFilePath, "modules/"+file, event)
		if err != nil {
			b.Fatalf("Real world scenario failed: %v", err)
		}
	}
}

// BenchmarkCacheInvalidation tests cache invalidation performance
func BenchmarkCacheInvalidation(b *testing.B) {
	finder := New("testproject")

	// Initialize cache
	_, err := finder.GoFileComesFromMain("module1.go")
	if err != nil {
		b.Fatalf("Cache initialization failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate file write that invalidates cache
		err := finder.updateCacheForFile("module1.go", "./modules/module1/module1.go", "write")
		if err != nil {
			b.Fatalf("Cache invalidation failed: %v", err)
		}

		// Access file after invalidation (should use cache if still valid)
		_, err = finder.GoFileComesFromMain("module1.go")
		if err != nil {
			b.Fatalf("Access after invalidation failed: %v", err)
		}
	}
}
