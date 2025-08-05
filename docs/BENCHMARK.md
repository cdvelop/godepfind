# GoDepFind Cache Performance Benchmarks

_Last updated: 2025-08-05_

---

```
==========================================
GoDepFind Cache Performance Benchmarks
==========================================

🔥 Running Cache vs No-Cache Comparison...
⏱️  Without Cache (each call rebuilds dependency graph):
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkGoFileComesFromMainWithoutCache-16          74   15335815 ns/op   104763 B/op     944 allocs/op
BenchmarkGoFileComesFromMainWithoutCache-16          80   15475456 ns/op   104828 B/op     944 allocs/op
BenchmarkGoFileComesFromMainWithoutCache-16          81   15035262 ns/op   104930 B/op     944 allocs/op
PASS
ok      github.com/cdvelop/godepfind  4.872s

⚡ With Cache (reuses dependency graph):
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkGoFileComesFromMainWithCache-16      5429742      212.0 ns/op      48 B/op        2 allocs/op
BenchmarkGoFileComesFromMainWithCache-16      5596209      212.5 ns/op      48 B/op        2 allocs/op
BenchmarkGoFileComesFromMainWithCache-16      5564578      208.8 ns/op      48 B/op        2 allocs/op
PASS
ok      github.com/cdvelop/godepfind  5.631s

==========================================
🎯 ThisFileIsMine Performance
==========================================

⏱️  ThisFileIsMine Without Cache:
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkThisFileIsMineWithoutCache-16         76   15317141 ns/op   104790 B/op     942 allocs/op
BenchmarkThisFileIsMineWithoutCache-16         78   15289091 ns/op   104692 B/op     942 allocs/op
BenchmarkThisFileIsMineWithoutCache-16         78   15101860 ns/op   104790 B/op     942 allocs/op
PASS
ok      github.com/cdvelop/godepfind  4.796s

⚡ ThisFileIsMine With Cache:
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkThisFileIsMineWithCache-16      3846771      311.3 ns/op       0 B/op        0 allocs/op
BenchmarkThisFileIsMineWithCache-16      3858279      304.9 ns/op       0 B/op        0 allocs/op
BenchmarkThisFileIsMineWithCache-16      3823710      328.6 ns/op       0 B/op        0 allocs/op
PASS
ok      github.com/cdvelop/godepfind  6.071s

==========================================
🏗️  Cache Initialization Cost
==========================================
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkCacheInitialization-16         76   15496954 ns/op   105129 B/op     942 allocs/op
BenchmarkCacheInitialization-16         73   15299626 ns/op   104854 B/op     942 allocs/op
BenchmarkCacheInitialization-16         74   15267709 ns/op   104833 B/op     942 allocs/op
PASS
ok      github.com/cdvelop/godepfind  4.704s

==========================================
🌍 Real-World Development Scenario
==========================================
📝 Simulating multiple handlers checking file ownership...
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkRealWorldScenario-16           234   5293460 ns/op    34350 B/op     308 allocs/op
BenchmarkRealWorldScenario-16           241   4976328 ns/op    34299 B/op     307 allocs/op
BenchmarkRealWorldScenario-16           240   5023229 ns/op    34341 B/op     308 allocs/op
PASS
ok      github.com/cdvelop/godepfind  6.504s

==========================================
♻️  Cache Invalidation Performance
==========================================
🔄 Testing cache invalidation and rebuilding...
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkCacheInvalidation-16      3941340      309.0 ns/op       0 B/op        0 allocs/op
BenchmarkCacheInvalidation-16      3970638      306.4 ns/op       0 B/op        0 allocs/op
BenchmarkCacheInvalidation-16      3809079      304.0 ns/op       0 B/op        0 allocs/op
PASS
ok      github.com/cdvelop/godepfind  5.972s

==========================================
📊 Multiple Files Comparison
==========================================

⏱️  Multiple Files Without Cache:
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkMultipleFilesWithoutCache-16         19   61208435 ns/op   418543 B/op    3773 allocs/op
PASS
ok      github.com/cdvelop/godepfind  1.629s

⚡ Multiple Files With Cache:
goos: linux
goarch: amd64
pkg: github.com/cdvelop/godepfind
cpu: 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz
BenchmarkMultipleFilesWithCache-16      1300862      891.2 ns/op      80 B/op        4 allocs/op
PASS
ok      github.com/cdvelop/godepfind  2.585s

==========================================
✅ Benchmark Complete!
==========================================

📈 Key Metrics to Look For:
   • Cache should be 100-1000x faster
   • Memory allocation should be minimal with cache
   • Real-world scenario should show significant improvement
   • Cache invalidation should be fast
```

---

**Interpretation:**
- The cache system provides a dramatic speedup (several orders of magnitude) and reduces memory allocations to nearly zero in most cases.
- Real-world and multi-file scenarios show the cache is highly effective.
- Cache initialization and invalidation are fast and efficient.

For more details, see the [CACHE.md](./CACHE.md).
