#!/bin/bash

echo "=========================================="
echo "GoDepFind Cache Performance Benchmarks"
echo "=========================================="
echo

echo "🔥 Running Cache vs No-Cache Comparison..."
echo "⏱️  Without Cache (each call rebuilds dependency graph):"
go test -bench="BenchmarkGoFileComesFromMainWithoutCache" -benchmem -count=3

echo
echo "⚡ With Cache (reuses dependency graph):"
go test -bench="BenchmarkGoFileComesFromMainWithCache" -benchmem -count=3

echo
echo "=========================================="
echo "🎯 ThisFileIsMine Performance"
echo "=========================================="

echo
echo "⏱️  ThisFileIsMine Without Cache:"
go test -bench="BenchmarkThisFileIsMineWithoutCache" -benchmem -count=3

echo
echo "⚡ ThisFileIsMine With Cache:"
go test -bench="BenchmarkThisFileIsMineWithCache" -benchmem -count=3

echo
echo "=========================================="
echo "🏗️  Cache Initialization Cost"
echo "=========================================="
go test -bench="BenchmarkCacheInitialization" -benchmem -count=3

echo
echo "=========================================="
echo "🌍 Real-World Development Scenario"
echo "=========================================="
echo "📝 Simulating multiple handlers checking file ownership..."
go test -bench="BenchmarkRealWorldScenario" -benchmem -count=3

echo
echo "=========================================="
echo "♻️  Cache Invalidation Performance"
echo "=========================================="
echo "🔄 Testing cache invalidation and rebuilding..."
go test -bench="BenchmarkCacheInvalidation" -benchmem -count=3

echo
echo "=========================================="
echo "📊 Multiple Files Comparison"
echo "=========================================="

echo
echo "⏱️  Multiple Files Without Cache:"
go test -bench="BenchmarkMultipleFilesWithoutCache" -benchmem -count=1

echo
echo "⚡ Multiple Files With Cache:"
go test -bench="BenchmarkMultipleFilesWithCache" -benchmem -count=1

echo
echo "=========================================="
echo "✅ Benchmark Complete!"
echo "=========================================="
echo
echo "📈 Key Metrics to Look For:"
echo "   • Cache should be 100-1000x faster"
echo "   • Memory allocation should be minimal with cache"
echo "   • Real-world scenario should show significant improvement"
echo "   • Cache invalidation should be fast"
echo
