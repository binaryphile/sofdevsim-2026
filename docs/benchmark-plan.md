# Benchmark Testing Plan

This document explains the benchmarking strategy for sofdevsim-2026, following Go Development Guide §7.

## Purpose

Benchmarks serve to:
1. Establish baseline performance for hot paths
2. Compare FluentFP overhead vs equivalent loops
3. Detect performance regressions in CI
4. Document expected performance characteristics

## Hot-Path Analysis

### Simulation Tick (called every day of simulation)

The `Tick()` function orchestrates one simulation day. Its complexity is:

```
O(d + t + c + i)
```

Where:
- d = developers (currently working)
- t = active tickets
- c = completed tickets
- i = open incidents

### Critical Bottleneck: FindActiveTicketIndex

```go
// simulation.go:68-75
func (s Simulation) FindActiveTicketIndex(id string) int {
    for i := range s.ActiveTickets {
        if s.ActiveTickets[i].ID == id {
            return i
        }
    }
    return -1
}
```

**Complexity:** O(n) linear search per call
**Called:** Once per working developer per tick
**Impact:** With 30 devs × 100 tickets × 100 days = 300,000 searches

This is the primary candidate for future optimization (hash map).

### FluentFP Usage in Hot Paths

| Pattern | Location | Called Per |
|---------|----------|------------|
| `slice.From().KeepIf().Len()` | engine.go:145-148 | Tick |
| `slice.From().ToFloat64()` | fever.go:86 | Tick |
| `slice.Fold()` | dora.go:90, 129 | Tick |

## Benchmark Categories

### 1. Engine Benchmarks

**File:** `internal/engine/benchmark_test.go`

| Benchmark | What It Measures | Scaling Factor |
|-----------|-----------------|----------------|
| `BenchmarkTick` | Full tick execution | devs × tickets |
| `BenchmarkFindActiveTicketIndex` | Linear search | active tickets |
| `BenchmarkVarianceCalculate` | RNG + math | constant |

```go
func BenchmarkTick(b *testing.B) {
    sim := setupBenchmarkSimulation(10, 50) // 10 devs, 50 tickets
    eng := NewEngine(sim)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        eng.Tick()
    }
}

func BenchmarkFindActiveTicketIndex(b *testing.B) {
    sim := setupBenchmarkSimulation(0, 100)
    targetID := sim.ActiveTickets[50].ID
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        sim.FindActiveTicketIndex(targetID)
    }
}
```

### 2. FluentFP vs Loop Comparisons

**File:** `internal/engine/fluentfp_bench_test.go`

| FluentFP | Loop | Pattern |
|----------|------|---------|
| `BenchmarkFluentFP_ToFloat64` | `BenchmarkLoop_ToFloat64` | Field extraction |
| `BenchmarkFluentFP_KeepIfLen` | `BenchmarkLoop_FilterCount` | Filter + count |
| `BenchmarkFluentFP_Fold` | `BenchmarkLoop_Accumulate` | Reduction |
| `BenchmarkFluentFP_Unzip4` | `BenchmarkLoop_FourPass` | Multi-field extraction |

```go
func BenchmarkFluentFP_KeepIfLen(b *testing.B) {
    tickets := generateTickets(100)
    cutoff := 50
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = slice.From(tickets).
            KeepIf(func(t Ticket) bool { return t.CompletedTick >= cutoff }).
            Len()
    }
}

func BenchmarkLoop_FilterCount(b *testing.B) {
    tickets := generateTickets(100)
    cutoff := 50
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        count := 0
        for _, t := range tickets {
            if t.CompletedTick >= cutoff {
                count++
            }
        }
        _ = count
    }
}
```

## Expected Results

Per FluentFP documentation:
- Single-operation chains: ~equal to loops
- Multi-operation chains: 2-3× overhead due to intermediate allocations

**Acceptable thresholds:**
- FluentFP overhead ≤ 3× for clarity benefits
- Hot path absolute time ≤ 10μs per tick

**Sample output format:**
```
goos: linux
goarch: amd64
pkg: github.com/binaryphile/sofdevsim-2026/internal/engine
cpu: 12th Gen Intel(R) Core(TM) i7-1265U
BenchmarkTick-8                      10000        112345 ns/op      8192 B/op       24 allocs/op
BenchmarkFindActiveTicketIndex-8   1000000          1234 ns/op         0 B/op        0 allocs/op
BenchmarkFluentFP_KeepIfLen-8       500000          2456 ns/op      1024 B/op        3 allocs/op
BenchmarkLoop_FilterCount-8        1000000          1123 ns/op         0 B/op        0 allocs/op
PASS
```

**Reading the output:**
| Column | Meaning |
|--------|---------|
| `-8` suffix | GOMAXPROCS (CPU cores used) |
| `10000` | Iterations run |
| `ns/op` | Nanoseconds per operation |
| `B/op` | Bytes allocated per operation |
| `allocs/op` | Heap allocations per operation |

## Running Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./internal/engine/

# Just FluentFP comparisons
go test -bench=FluentFP -benchmem ./internal/engine/

# With CPU profiling
go test -bench=BenchmarkTick -cpuprofile=cpu.prof ./internal/engine/
```

## Tracking in CLAUDE.md

After each significant change, update the Benchmarks section in CLAUDE.md:

```markdown
## Benchmarks

**Baseline (DATE):**
```
BenchmarkTick-8                    XXXX ns/op    XXXX B/op    XX allocs/op
```

**After [change description]:**
```
BenchmarkTick-8                    XXXX ns/op    XXXX B/op    XX allocs/op
```

Note: [Explanation of any regression/improvement]
```

## Future Optimization Candidates

1. **Replace FindActiveTicketIndex with map lookup**
   - Current: O(n) per lookup
   - After: O(1) with `map[string]int` index
   - Impact: 10-100× improvement for large simulations

2. **Cache completed ticket counts**
   - Currently scanned in `updateBuffer()`
   - Could be incremented on completion

3. **Batch event generation**
   - Currently generates slice per call
   - Could pre-allocate or pool
