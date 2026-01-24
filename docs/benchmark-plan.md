# Benchmark Testing Plan

This document explains the benchmarking strategy for sofdevsim-2026, following Go Development Guide §7.

## Purpose

Benchmarks serve to:
1. Establish baseline performance for hot paths
2. Compare FluentFP overhead vs equivalent loops
3. Detect performance regressions in CI
4. Document expected performance characteristics

## Benchmark Categories

### 1. Engine Hot Paths

**File:** `internal/engine/benchmark_test.go`

| Benchmark | What It Measures | Baseline |
|-----------|-----------------|----------|
| `BenchmarkTick` | Full tick execution | 19μs |
| `BenchmarkTick_LargeSimulation` | Tick with 30 devs, 200 tickets | 21μs |
| `BenchmarkFindActiveTicketIndex` | Linear search (100 tickets) | 153ns |
| `BenchmarkVarianceCalculate` | RNG + math | 9μs |

**Critical Bottleneck:** `FindActiveTicketIndex` is O(n) linear search, called once per working developer per tick. Future optimization: hash map.

### 2. FluentFP vs Loop Comparisons

**File:** `internal/engine/fluentfp_bench_test.go`

| FluentFP | Loop | Pattern |
|----------|------|---------|
| `BenchmarkFluentFP_ToFloat64` | `BenchmarkLoop_ToFloat64` | Field extraction |
| `BenchmarkFluentFP_KeepIfLen` | `BenchmarkLoop_FilterCount` | Filter + count |
| `BenchmarkFluentFP_Fold` | `BenchmarkLoop_Accumulate` | Reduction |
| `BenchmarkFluentFP_Unzip4` | `BenchmarkLoop_FourPass` | Multi-field extraction |

**Expected:** FluentFP overhead ≤ 3× for single operations (acceptable for clarity benefits).

### 3. Event Sourcing

**File:** `internal/events/projection_test.go`, `internal/events/upcasting_test.go`

| Benchmark | What It Measures | Target | Baseline |
|-----------|-----------------|--------|----------|
| `BenchmarkProjection_Apply_SingleEvent` | Single event application | < 1μs | 45ns |
| `BenchmarkProjection_ReplayFull` | Replay 1000 events | < 1ms | 36μs |
| `BenchmarkUpcaster_Apply_NoTransform` | Map lookup (miss) | < 250ns | 215ns |
| `BenchmarkUpcaster_Apply_WithTransform` | v1→v2 transform | < 500ns | 483ns |
| `BenchmarkUpcaster_Apply_TransitiveChain` | v1→v2→v3 chain | < 1μs | 740ns |

### 4. TUI Client

**File:** `internal/tui/client_benchmark_test.go`

| Benchmark | What It Measures | Baseline |
|-----------|-----------------|----------|
| `BenchmarkClient_CreateSimulation` | HTTP round-trip + simulation creation | 124μs |
| `BenchmarkClient_Tick` | HTTP round-trip + tick execution | 402μs |
| `BenchmarkClient_Assign` | HTTP round-trip + ticket assignment | 173μs |

**Target:** < 1ms for all local operations.

### 5. API Middleware (NOT YET IMPLEMENTED)

**File:** `internal/api/dedup_bench_test.go` (to be created)

| Benchmark | What It Measures | Target |
|-----------|-----------------|--------|
| `BenchmarkDedup_CacheHit` | Return cached response | < 1μs |
| `BenchmarkDedup_CacheMiss` | Execute + cache response | handler time + < 10μs |
| `BenchmarkDedup_Contention` | Concurrent cache access | measure lock wait |
| `BenchmarkDedup_MemoryGrowth` | Memory per cached response | track allocations |

## Running Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./...

# Engine only
go test -bench=. -benchmem ./internal/engine/

# Event sourcing only
go test -bench=. -benchmem ./internal/events/

# FluentFP comparisons
go test -bench=FluentFP -benchmem ./internal/engine/

# With CPU profiling
go test -bench=BenchmarkTick -cpuprofile=cpu.prof ./internal/engine/
```

## Reading Benchmark Output

```
BenchmarkTick-8    10000    112345 ns/op    8192 B/op    24 allocs/op
```

| Column | Meaning |
|--------|---------|
| `-8` suffix | GOMAXPROCS (CPU cores used) |
| `10000` | Iterations run |
| `ns/op` | Nanoseconds per operation |
| `B/op` | Bytes allocated per operation |
| `allocs/op` | Heap allocations per operation |

## Tracking in CLAUDE.md

After significant changes, update the Benchmarks section in CLAUDE.md with before/after comparisons.

## Future Optimization Candidates

1. **Replace FindActiveTicketIndex with map lookup** - O(n) → O(1)
2. **DedupMiddleware benchmarks** - validate cache performance claims
3. **Projection replay streaming** - memory for large event streams
